#!/usr/bin/env bash
#
# Runs one Tier 2 suite: one server, one client, N trials of each selected
# task, per docs/tier2-task-suites.md sections 2, 3, 6 and 7.
#
# One invocation is one (server, client) pair. Trials are serial within a
# suite, because section 6 only permits concurrency across servers, never
# against one server's log.
#
# Usage:
#   tier2/run-suite.sh --server filesystem|playwright|github \
#                      --client claude|gemini \
#                      [--trials N] [--tasks FS-01,FS-03] [--date YYYY-MM-DD] \
#                      [--out DIR] [--timeout SECONDS] [--max-turns N] \
#                      [--model ID] [--no-full-results] [--dry-run] [--force]
#   tier2/run-suite.sh --self-test
#     Runs the runner's own unit cases (section 4.1 precedence rule, the
#     image-digest failure path, the tool-call gap's name resolution) and
#     exits. No server, client or fixture is touched.
#
# Defaults: --trials 3, all five tasks for the server, --date today (UTC),
# --out data/tier2, --timeout 300, --max-turns 20.
#
# Outputs, all under <out>/<date>/:
#   manifest.jsonl                       run header, one row per trial, run footer
#   <client>/<task>-t<N>.jsonl           interposer frame log for the trial
#   <client>/<task>-t<N>.client.json     the client's own --output-format json
#   <client>/<task>-t<N>.analyze.json    loadline analyze report for the log
#   <client>/work/<task>-t<N>/           per-trial cwd: mcp config, prompt, transcript
#   <client>/scratch/<task>-t<N>/        per-trial fixture copy (filesystem tasks)
#
# Resumable: a trial whose (server, client, task, trial) row already exists in
# the day's manifest is skipped, so an interrupted suite can be re-invoked with
# the same arguments and picks up where it stopped.
#
# The frame logs are secrets (interposer/README.md, "Security"). data/tier2/ is
# gitignored for that reason; do not move these artifacts out of it.
#
# Cost safety: Claude Code bills a metered API wallet instead of the operator's
# plan whenever ANTHROPIC_API_KEY (or ANTHROPIC_AUTH_TOKEN) is present in its
# environment. Tier 2 runs on plan quota by design (PRD.md 6), so every client
# invocation below is launched with those variables stripped, and the runner
# never sources the repo's .env. The same stripping is applied to the Gemini
# key variables so a trial cannot silently move onto metered billing there
# either.

set -euo pipefail

# ---------------------------------------------------------------- constants --

SUITE_VERSION="1.0.7"      # docs/tier2-task-suites.md header field
MANIFEST_SCHEMA="tier2-manifest/0.1"

# The github server's default image, no tag: the same reference server_argv's
# docker branch below launches. One constant so the launch command and the
# digest resolution can never name two different images.
GITHUB_IMAGE="ghcr.io/github/github-mcp-server"

# The docker CLI used to resolve a container server's image digest.
# Overridable so --self-test can point it at a binary that does not exist and
# exercise the failure-as-data path without a docker daemon.
DOCKER_BIN="${LOADLINE_TIER2_DOCKER_BIN:-docker}"

# Refusal markers for the OPEN 7 classification rule. This list is the
# documented heuristic, reproduced verbatim in section 4.1 of the spec.
# Matched case-insensitively against the client's final message, and consulted
# only for a trial that made zero tool calls AND failed its success check.
REFUSAL_MARKERS=(
  "can't" "cannot" "can not" "unable to" "not able to"
  "don't have access" "do not have access" "no access to"
  "not permitted" "not allowed" "don't have permission" "do not have permission"
  "i'm sorry" "i am sorry" "i apologize"
  "isn't available" "is not available" "unavailable"
  "no tools" "no such tool" "tool is not"
)

# matched_refusal_markers TRANSCRIPT -> JSON array of the markers found.
matched_refusal_markers() {
  local transcript="$1" m out=()
  [[ -f "$transcript" ]] || { echo '[]'; return; }
  for m in "${REFUSAL_MARKERS[@]}"; do
    if grep -Fqi -- "$m" "$transcript"; then out+=("$m"); fi
  done
  if [[ ${#out[@]} -eq 0 ]]; then
    echo '[]'
  else
    printf '%s\n' "${out[@]}" | jq -R . | jq -s -c .
  fi
}

# classify_trial CLIENT_FAILED SUCCESS TOOL_CALLS TRANSCRIPT -> two lines on
# stdout: the classification, then the matched-refusal-marker JSON array
# (always "[]" outside the branch that consults it). CLIENT_FAILED and SUCCESS
# are "true" or "false".
#
# This is the OPEN 7 rule, spec section 4.1. The success check runs first:
# client_error means the client errored out before answering, and a trial
# whose success check passed demonstrably did answer, so a passed check
# outranks client_error. client_failed is still passed through by the caller
# into the manifest's client_reported_error field regardless of what this
# function returns, so the evidence a client reported its own failure is never
# lost even when it does not decide the bucket.
#
# Found on the Tier 2 first full run (suite 1.0.1, 2026-08-18):
# GH-03/gemini/t3 recorded a non-null Gemini error object, success true, and
# one tools/call frame. The old order checked client_failed first and bucketed
# it client_error, an infrastructure-fault reading on a trial that plainly
# succeeded.
classify_trial() {
  local client_failed="$1" success="$2" tool_calls="$3" transcript="$4"
  local classification matched="[]"
  if [[ "$success" == true ]]; then
    if [[ "$tool_calls" -gt 0 ]]; then classification="tool_use_success"; else classification="answered_without_tools"; fi
  elif [[ "$client_failed" == true ]]; then
    classification="client_error"
  elif [[ "$tool_calls" -gt 0 ]]; then
    classification="tool_use_failed"
  else
    matched="$(matched_refusal_markers "$transcript")"
    if [[ "$matched" != "[]" ]]; then classification="declined"; else classification="failed_no_tool_use"; fi
  fi
  echo "$classification"
  echo "$matched"
}

# gemini_tool_call_gap LOG CLIENTJSON -> integer, or null when it cannot be
# computed.
#
# Counted only over tool names the server actually advertised in its tools/list
# response, which is the whole point of the measure. A naive totalCalls minus
# wire-frames difference is not a fault signal: with the built-ins switched off
# the model regularly tries one that no longer exists, and Gemini counts that
# attempt in stats.tools while the server never hears about it. Both filesystem
# trials of the 2026-08-18 rerun showed a positive naive gap that was entirely
# run_shell_command, which is the isolation working exactly as designed. A gap
# on a tool the server did advertise is the different thing: the client meant
# to call the server, and the call never left the client.
#
# Requires --full-results, because the served tool list is only in the log when
# response payloads are logged. Without it the function returns null rather
# than guessing.
#
# The client keys stats.tools.byName by the name the model called, and that
# spelling changed under the client upgrade. On 0.18.4 an MCP tool was keyed by
# its bare server-side name (browser_navigate, read_text_file); on 0.55.1 it is
# keyed by the fully qualified mcp_<server>_<tool> (mcp_playwright_browser_
# navigate). The wire name in the log is the bare one either way, because that
# is what travels in the tools/call frame. Matching the two sides on the bare
# name alone therefore scored every served tool at zero client calls against a
# positive wire count, every difference came out negative, and the
# select(. > 0) filter swallowed the lot: the gap would have read a clean 0 on
# every trial and the OPEN 11 detector would have been silently dead on the new
# client while still appearing to run.
#
# The measure therefore resolves the spelling per served tool rather than
# summing both. Summing was the 1.0.4 rule and it opens a smaller hole of its
# own on exactly one server: with the built-in surface off, a call the model
# forms to a bare name dies inside the client and still lands in
# stats.tools.byName, and the advertised-name filter only drops that stray when
# no server advertised the name. The filesystem server advertises read_file,
# write_file and list_directory, which section 7.1 item 2 of the spec already
# records as colliding with Gemini's built-ins, so on 0.55.1 the real call is
# keyed mcp_filesystem_write_file, the stray is keyed write_file, and adding
# them guarantees no cancellation against the wire count. The term would go
# positive on a trial where nothing was lost on the pipe. Resolving instead
# takes the qualified key when the trial's own client JSON has one and the bare
# key otherwise: the two keys are different evidence and summing throws the
# distinction away. 0.18.4 is untouched, since only the bare key exists there.
#
# Residual, stated rather than hidden: on 0.55.1 a served tool keyed bare with
# no qualified key present still uses the bare count, so a stray in that one
# shape would still read positive. It did not occur on any of the 45 Gemini
# trials of the 1.0.1 run: 14 of them carry a bare stray, 4 of those are
# filesystem trials, and none of the stray names was one its own server
# advertised. Any future change to how a client names a server's tool has to
# be checked against this function, not assumed; --self-test asserts all three
# spelling shapes.
gemini_tool_call_gap() {
  local logfile="$1" clientjson="$2" server="$3"
  [[ -s "$logfile" && -s "$clientjson" ]] || { echo null; return; }
  jq -s -c --slurpfile cj "$clientjson" --arg srv "$server" '
    ([.[] | select(.result_full.tools != null) | .result_full.tools[].name] | unique) as $served
    | ([.[] | select(.method == "tools/call") | .params_full.name]
       | group_by(.) | map({key: .[0], value: length}) | from_entries) as $wire
    | (($cj[0].stats.tools.byName // {})) as $client
    | if ($served | length) == 0 then null
      else [$served[]
            | . as $t
            | ("mcp_" + $srv + "_" + $t) as $q
            | (if ($client | has($q)) then ($client[$q].count // 0)
               else ($client[$t].count // 0) end)
              - ($wire[$t] // 0)]
           | map(select(. > 0)) | add // 0
      end' "$logfile" 2>/dev/null || echo null
}

# gemini_tool_call_gap_bare_unwired LOG CLIENTJSON SERVER -> integer, or null
# when it cannot be computed (same conditions as gemini_tool_call_gap).
#
# The evidence the 1.0.5 resolution rule discards, kept as its own labeled
# field. When a trial holds both spellings for a served tool, the qualified
# key wins the gap arithmetic and the bare count is dropped, but that count is
# not noise: a call attributed to a name that never appeared in a tools/call
# frame died inside the client, which is a different fact from a call the wire
# lost, and one integer cannot carry both.
#
# Per served tool, the bare-key count, counted only when the qualified key for
# that same tool is present in stats.tools.byName. With the qualified key
# present, every wire frame for the tool is attributed to the qualified
# spelling, which is what makes the bare count unwired by construction. When
# no qualified key exists for the tool, the bare count is already
# gemini_tool_call_gap's fallback evidence (the residual documented above) and
# is not repeated here. The same anti-double-reporting logic gates the whole
# field: on a trial with no mcp_<srv>_ key at all the client is 0.18.4-style,
# the bare key IS the real spelling, and the field reads 0.
#
# A positive value here does NOT set harness_suspect. It is the model reaching
# for a disabled built-in whose name the server happens to share, dying inside
# the client by design; the interop fault signal stays with tool_call_gap.
gemini_tool_call_gap_bare_unwired() {
  local logfile="$1" clientjson="$2" server="$3"
  [[ -s "$logfile" && -s "$clientjson" ]] || { echo null; return; }
  jq -s -c --slurpfile cj "$clientjson" --arg srv "$server" '
    ([.[] | select(.result_full.tools != null) | .result_full.tools[].name] | unique) as $served
    | (($cj[0].stats.tools.byName // {})) as $client
    | if ($served | length) == 0 then null
      elif ([$client | keys[] | select(startswith("mcp_" + $srv + "_"))] | length) == 0 then 0
      else [$served[]
            | . as $t
            | if ($client | has("mcp_" + $srv + "_" + $t))
              then ($client[$t].count // 0) else 0 end]
           | add // 0
      end' "$logfile" 2>/dev/null || echo null
}

# self_test -> 0 and "ok" per case on stdout if every case below holds, 1 and a
# diff otherwise. Exercises classify_trial's precedence rule, the image-digest
# failure path and gemini_tool_call_gap's name resolution directly; no client,
# network call or fixture is touched. Invoked by `run-suite.sh --self-test`.
self_test() {
  local failures=0
  local no_transcript declined_transcript failed_transcript
  no_transcript="$(mktemp)"
  declined_transcript="$(mktemp)"
  failed_transcript="$(mktemp)"
  echo "" >"$no_transcript"
  echo "I don't have access to that repository." >"$declined_transcript"
  echo "The count is 42." >"$failed_transcript"

  assert_classify() {
    local name="$1" client_failed="$2" success="$3" tool_calls="$4" transcript="$5" want="$6"
    local got
    got="$(classify_trial "$client_failed" "$success" "$tool_calls" "$transcript" | head -n1)"
    if [[ "$got" == "$want" ]]; then
      echo "ok   $name -> $got"
    else
      echo "FAIL $name -> got $got, want $want"
      failures=$((failures + 1))
    fi
  }

  # GH-03/gemini/t3: Gemini error object non-null (client_failed) + success
  # true + >=1 tools/call frame -> tool_use_success. The passed check outranks
  # the reported error.
  assert_classify "client_failed + success + tool_calls: tool_use_success" \
    true true 1 "$no_transcript" "tool_use_success"

  # Inverse: error non-null + success false -> client_error. Nothing to
  # outrank the reported error when the trial did not answer.
  assert_classify "client_failed + no success: client_error" \
    true false 1 "$no_transcript" "client_error"

  # Success with zero tools/call frames is answered_without_tools, unchanged
  # from 4.1.
  assert_classify "success, zero tool calls: answered_without_tools" \
    false true 0 "$no_transcript" "answered_without_tools"

  # The same with a reported client error: the check still outranks it, so the
  # trial is answered_without_tools and not client_error. This is the second
  # bucket the amended precedence moves, and the one no run so far has hit, so
  # it is asserted here rather than left to the next run to discover.
  assert_classify "client_failed + success, zero tool calls: answered_without_tools" \
    true true 0 "$no_transcript" "answered_without_tools"

  # No client_failed, tool calls, check failed -> tool_use_failed, unchanged.
  assert_classify "tool_use_failed, unchanged" \
    false false 2 "$no_transcript" "tool_use_failed"

  # Zero tool calls, check failed, refusal marker present -> declined,
  # unchanged.
  assert_classify "declined, unchanged" \
    false false 0 "$declined_transcript" "declined"

  # Zero tool calls, check failed, no refusal marker -> failed_no_tool_use,
  # unchanged.
  assert_classify "failed_no_tool_use, unchanged" \
    false false 0 "$failed_transcript" "failed_no_tool_use"

  # server_image_digest capture, failure-as-data case: docker absent must
  # record a "digest unavailable: " string on the trial, not abort the run.
  # DOCKER_BIN is overridden to a path that cannot exist for the duration of
  # this one call, which is the same seam LOADLINE_TIER2_DOCKER_BIN gives an
  # operator on a machine without docker; the global is restored immediately
  # after so the rest of --self-test (and any real run) sees the real default.
  local digest_result saved_docker_bin="$DOCKER_BIN"
  DOCKER_BIN="/nonexistent/loadline-self-test-docker-$$"
  digest_result="$(resolve_image_digest "$GITHUB_IMAGE")"
  DOCKER_BIN="$saved_docker_bin"
  if [[ "$digest_result" == "digest unavailable: "* ]]; then
    echo "ok   server_image_digest records failure string when docker is absent -> $digest_result"
  else
    echo "FAIL server_image_digest docker-absent case -> got '$digest_result', want a 'digest unavailable: ' prefixed string"
    failures=$((failures + 1))
  fi

  # gemini_tool_call_gap name-resolution cases. The whole reason the 0.55.1
  # rename went unnoticed for a release is that a dead detector and a healthy
  # one print the same character, so both directions are asserted here: the
  # false positive the summing rule allows, and the silent zero the name
  # matching exists to catch. Each case builds a two-line log (a tools/list
  # response advertising write_file, then N tools/call frames for it) and a
  # client JSON with the byName block under test, so nothing outside this
  # function is touched.
  assert_gap() {
    local name="$1" byname="$2" wire_calls="$3" want="$4" want_bare="$5"
    local gap_log gap_cj got got_bare i
    gap_log="$(mktemp)"
    gap_cj="$(mktemp)"
    printf '%s\n' '{"result_full":{"tools":[{"name":"write_file"}]}}' >"$gap_log"
    for ((i = 0; i < wire_calls; i++)); do
      printf '%s\n' '{"method":"tools/call","params_full":{"name":"write_file"}}' >>"$gap_log"
    done
    printf '{"stats":{"tools":{"byName":%s}}}\n' "$byname" >"$gap_cj"
    got="$(gemini_tool_call_gap "$gap_log" "$gap_cj" filesystem)"
    got_bare="$(gemini_tool_call_gap_bare_unwired "$gap_log" "$gap_cj" filesystem)"
    rm -f "$gap_log" "$gap_cj"
    if [[ "$got" == "$want" && "$got_bare" == "$want_bare" ]]; then
      echo "ok   $name -> gap=$got bare_unwired=$got_bare"
    else
      echo "FAIL $name -> got gap=$got bare_unwired=$got_bare, want gap=$want bare_unwired=$want_bare"
      failures=$((failures + 1))
    fi
  }

  # FALSE POSITIVE, must be 0. Two real qualified calls, both on the wire, plus
  # one bare stray that died inside the client because tools.core: ["mcp_*"]
  # never registered the bare name. The stray survives the advertised-name
  # filter because the filesystem server does advertise write_file (spec 7.1
  # item 2), so summing the two keys reports a client-side interop fault on a
  # trial where nothing was lost on the pipe.
  assert_gap "both spellings present: qualified wins, bare stray kept as its own evidence" \
    '{"write_file": {"count": 1}, "mcp_filesystem_write_file": {"count": 2}}' 2 "0" "1"

  # SILENT ZERO, must stay caught at 2. The OPEN 11 case the name matching
  # exists for: three qualified calls, one wire frame, two calls that never
  # left the client. This is the guard that resolution does not re-break what
  # summing was introduced to fix.
  assert_gap "qualified only: calls that never reached the wire are still counted" \
    '{"mcp_filesystem_write_file": {"count": 3}}' 1 "2" "0"

  # OLD CLIENT, must be 2. Gemini 0.18.4 keys byName by the bare server-side
  # name, so the bare count is the real count and the arithmetic is unchanged.
  assert_gap "bare only: 0.18.4 spelling unchanged, bare_unwired gated off" \
    '{"write_file": {"count": 3}}' 1 "2" "0"

  rm -f "$no_transcript" "$declined_transcript" "$failed_transcript"

  if [[ "$failures" -eq 0 ]]; then
    echo "self_test: all cases passed"
    return 0
  else
    echo "self_test: $failures case(s) failed"
    return 1
  fi
}

# resolve_image_digest IMAGE_REF -> prints the digest reference the image
# resolves to right now (e.g. ghcr.io/github/github-mcp-server@sha256:...),
# per docker's own RepoDigests, or a "digest unavailable: " string on any
# failure (docker missing, pull failed, no repo digest reported). Failure is
# data, not a reason to stop the run: the caller records whatever this prints.
#
# Pulls first, then inspects. Inspecting a locally cached tag without pulling
# can report a digest for an older image than the one that actually runs, if
# the tag moved upstream since the last pull; that is the exact reproducibility
# gap this field exists to close, so skipping the pull would defeat the point.
resolve_image_digest() {
  local image_ref="$1" pull_out digest
  if ! command -v "$DOCKER_BIN" >/dev/null 2>&1; then
    echo "digest unavailable: docker CLI '${DOCKER_BIN}' is not available"
    return
  fi
  if ! pull_out="$("$DOCKER_BIN" pull "$image_ref" 2>&1)"; then
    echo "digest unavailable: docker pull ${image_ref} failed: $(printf '%s' "$pull_out" | tail -n1)"
    return
  fi
  digest="$("$DOCKER_BIN" image inspect --format '{{index .RepoDigests 0}}' "$image_ref" 2>/dev/null || true)"
  if [[ "$digest" != *"@sha256:"* ]]; then
    echo "digest unavailable: docker image inspect reported no repo digest for ${image_ref}"
    return
  fi
  printf '%s' "$digest"
}

# resolve_current_server_image_digest -> the digest resolve_image_digest
# reports for the image the running SERVER launches via docker right now, or
# the empty string when SERVER is not launched via docker today: filesystem
# and playwright never are, and github isn't when LOADLINE_TIER2_GITHUB_CMD
# overrides it to the non-container Go binary (section 7.4). The empty string
# is what lets the manifest record JSON null for "not applicable" instead of a
# string, so that stays distinguishable from "applicable but the resolve
# failed".
resolve_current_server_image_digest() {
  if [[ "$SERVER" == "github" && -z "${LOADLINE_TIER2_GITHUB_CMD:-}" ]]; then
    resolve_image_digest "$GITHUB_IMAGE"
  else
    printf ''
  fi
}

# The suite measures what an MCP server costs, so a client that answers a
# fixture task with its own file or shell tools produces no measurement at all.
# Both clients therefore run with their built-in tool surface switched off, by
# the mechanism each one offers.
#
# Built-in Claude Code tools denied per trial. The --allowedTools allowlist of
# spec section 7.1 is not enough on its own: under --permission-mode dontAsk
# the read-only built-ins (Read, Glob, Grep) still run without a prompt, and a
# read task then gets answered by Read while the MCP server sees no call at
# all. Verified on 2.1.234: FS-01 with the allowlist alone made zero tools/call
# frames and still returned the right answer, and the same trial with these
# tools denied called mcp__filesystem__read_text_file with no permission
# denials recorded. This is the Claude-side counterpart to the Gemini
# allowlist below.
CLAUDE_DISALLOWED_TOOLS="Bash,Read,Write,Edit,MultiEdit,NotebookEdit,Glob,Grep,LS,WebFetch,WebSearch,Task,TodoWrite,SlashCommand"

# Claude Code also runs with --setting-sources "" so that no settings source
# (user, project or local) is loaded, which is what keeps the operator's own
# CLAUDE.md out of the trial.
#
# This is not hygiene, it is a measurement correctness fix, and the fault it
# repairs was live. In the 2026-08-18 github shakedown GH-03 came back with
# zero tool calls and this final message: "I have a standing instruction (from
# your global CLAUDE.md) that I never send outbound communications - including
# creating issues - myself". The trial measured the operator's memory file, not
# the client against the server, and the classifier bucketed it
# failed_no_tool_use, which reads as a hallucination and is not what happened.
# Verified on 2.1.234: with no flag the client answers a question whose only
# source is ~/.claude/CLAUDE.md; with --setting-sources "" the same question
# comes back NONE, and the MCP server passed by --mcp-config still loads.
#
# --bare is the flag that sounds like the right answer and is not. It skips
# CLAUDE.md, but its own help text says Anthropic auth then becomes "strictly
# ANTHROPIC_API_KEY or apiKeyHelper", OAuth and keychain never read: it would
# move every trial onto the metered wallet this runner exists to avoid.
CLAUDE_SETTING_SOURCES=""

# Gemini's built-in tool allowlist. Registers no built-in tool and leaves the
# whole MCP surface intact, because no built-in name matches the pattern and
# every MCP tool does.
#
# tools.core rather than tools.exclude, and the difference is not cosmetic:
# exclusion matches a bare tool name, and the filesystem server's surface
# collides with Gemini's built-ins on read_file, write_file and list_directory,
# so an exclusion list naming those also removed the *server's* tools. The
# first Gemini shakedown failed FS-03 and FS-04 exactly that way, with the
# model reporting that write_file did not exist while create_directory and
# edit_file did. tools.exclude is deprecated in 0.55 besides, in favour of the
# policy engine.
#
# The value is ["mcp_*"] and not the empty array this runner used on 0.18.4.
# On 0.55.1 a non-empty tools.core is required, and the reason is upstream
# behaviour rather than a preference: any truthy tools.core makes the client
# append a wildcard "*" DENY policy rule at a priority just under the
# allowlist's own ALLOW rules, and that DENY carries no MCP exemption
# (gemini-cli issue #28361, open). With tools.core [] nothing matches the
# allowlist, the wildcard DENY strips the MCP tools too, the request goes out
# with an empty tool list, and the API rejects it outright:
#   tools[0].tool_type: required one_of 'tool_type' must have one initialized
#   field
# Reproduced on 0.55.1 on 2026-08-19, with and without an MCP server
# configured; the trial dies before it issues a single call.
#
# ["mcp_*"] is the fail-closed form of the same intent. Gemini names an MCP
# tool mcp_<server>_<tool>, so every server tool matches the allowlist
# explicitly and outranks the wildcard DENY, while no built-in name matches and
# all of them fall through to it. Verified on 0.55.1: a read task calls
# mcp_filesystem_read_text_file with one frame on the wire, and a shell request
# comes back "I do not have a tool to execute shell commands" with
# run_shell_command recorded as an attempted-and-failed call the server never
# saw. It also survives the upstream fix: if #28361 lands and tools.core stops
# touching MCP tools, ["mcp_*"] still registers zero built-ins.
GEMINI_CORE_TOOLS='["mcp_*"]'

# Environment variables Gemini CLI must not pick up from a project .env file.
# Unlike Claude Code, which ignores .env entirely (verified on 2.1.234 with a
# planted invalid key in a parent directory), Gemini CLI walks up from its
# working directory and loads the first .env it finds. A trial's working
# directory is inside this repo, and this repo's .env carries the Tier 1 sweep
# credentials, so without this exclusion every Gemini trial silently switched
# off the operator's OAuth session onto the API key in that file and died with
# ProjectIdRequiredError. Excluding the key variables also keeps the trial off
# metered billing, which is the same rule the NOENV stripping enforces for the
# runner's own environment.
GEMINI_EXCLUDED_ENV_VARS='["GEMINI_API_KEY","GOOGLE_API_KEY","GOOGLE_GENAI_API_KEY","GOOGLE_GENAI_USE_VERTEXAI","ANTHROPIC_API_KEY","ANTHROPIC_AUTH_TOKEN","ANTHROPIC_BASE_URL"]'

# ------------------------------------------------------------------ options --

SERVER=""
CLIENT=""
TRIALS=3
TASKS_ARG=""
DATE="$(date -u +%F)"
OUT_ROOT=""
TIMEOUT=300
MAX_TURNS=20
MODEL=""
FULL_RESULTS=1
DRY_RUN=0
FORCE=0

# The digest of the container image the run actually used, resolved once by
# prepare_suite_fixture before the first trial. Empty (-> JSON null in the
# manifest) for a server that is not launched via docker; see
# resolve_current_server_image_digest.
SERVER_IMAGE_DIGEST=""

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "${here}/.." && pwd)"

die() { echo "run-suite: $*" >&2; exit 1; }
log() { echo "[$(date -u +%H:%M:%S)] $*" >&2; }

# --self-test runs classify_trial's own unit cases and exits; no server,
# client or --date is required, and nothing under data/tier2/ is touched.
if [[ "${1:-}" == "--self-test" ]]; then
  self_test
  exit $?
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
  --server) SERVER="${2:-}"; shift 2 ;;
  --client) CLIENT="${2:-}"; shift 2 ;;
  --trials) TRIALS="${2:-}"; shift 2 ;;
  --tasks) TASKS_ARG="${2:-}"; shift 2 ;;
  --date) DATE="${2:-}"; shift 2 ;;
  --out) OUT_ROOT="${2:-}"; shift 2 ;;
  --timeout) TIMEOUT="${2:-}"; shift 2 ;;
  --max-turns) MAX_TURNS="${2:-}"; shift 2 ;;
  --model) MODEL="${2:-}"; shift 2 ;;
  --no-full-results) FULL_RESULTS=0; shift ;;
  --dry-run) DRY_RUN=1; shift ;;
  # Re-run cells that are already in the day's manifest. The manifest stays
  # append-only, so the superseded rows remain as the record of what the
  # broken setup produced; summarize.sh reads the last row per cell key. This
  # is for re-running after a harness fix, not for retrying a trial that
  # failed on its merits, which section 6 forbids.
  --force) FORCE=1; shift ;;
  -h | --help) sed -n '3,41p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
  *) die "unknown argument: $1" ;;
  esac
done

case "$SERVER" in
filesystem | playwright | github) ;;
"") die "--server is required (filesystem|playwright|github)" ;;
*) die "unknown server: $SERVER" ;;
esac
case "$CLIENT" in
claude | gemini) ;;
"") die "--client is required (claude|gemini)" ;;
*) die "unknown client: $CLIENT" ;;
esac
[[ "$TRIALS" =~ ^[0-9]+$ && "$TRIALS" -ge 1 ]] || die "--trials must be a positive integer"
[[ "$DATE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || die "--date must be YYYY-MM-DD"
OUT_ROOT="${OUT_ROOT:-${repo}/data/tier2}"

if [[ -z "$MODEL" ]]; then
  case "$CLIENT" in
  claude) MODEL="claude-sonnet-5" ;;
  # The spec's section 7.2 example writes the `flash` alias. The runner pins
  # the full id instead: an alias is free to point at a different model
  # between runs, which is the same failure mode section 7.2 rejects `auto`
  # for. The id the client actually billed is read back out of the client's
  # own stats block and recorded per trial either way.
  gemini) MODEL="gemini-2.5-flash" ;;
  esac
fi

command -v jq >/dev/null 2>&1 || die "jq is required and was not found on PATH"
command -v go >/dev/null 2>&1 || die "go is required to build the interposer and analyzer"
command -v "$CLIENT" >/dev/null 2>&1 || die "client binary '$CLIENT' not found on PATH"

# Cost-safety guard. Presence of an API key in the runner's own environment is
# not an error, because the runner does other things with it; carrying it into
# a client subprocess is, because it moves the trial onto metered billing. The
# stripping happens at the invocation, not here, so this only reports it.
STRIPPED_KEYS=()
for v in ANTHROPIC_API_KEY ANTHROPIC_AUTH_TOKEN GEMINI_API_KEY GOOGLE_API_KEY GOOGLE_GENAI_API_KEY; do
  if [[ -n "${!v:-}" ]]; then STRIPPED_KEYS+=("$v"); fi
done
if [[ ${#STRIPPED_KEYS[@]} -gt 0 ]]; then
  echo "run-suite: stripping ${STRIPPED_KEYS[*]} from every client invocation" \
    "(Tier 2 runs on plan quota, not on a metered API wallet)" >&2
fi
# NOENV is prepended to every client invocation.
#
# DISABLE_AUTOUPDATER rides along with the key stripping because it is the same
# kind of guard: a thing the client would otherwise do to itself mid-run. The
# 2026-08-18 github shakedown recorded version_drift on its own footer because
# Claude Code updated 2.1.234 -> 2.1.235 between the run's first and last
# trial, which under the section 5 rule makes the whole run uncomparable. This
# is not the version pin OPEN 8 correctly refuses to promise: it holds nothing
# across runs and claims nothing about which version the operator has. It only
# stops the client from changing underneath a run that is already in flight.
# Drift detection stays on regardless, because an operator-driven update
# between runs is still the thing the footer has to catch.
NOENV=(env -u ANTHROPIC_API_KEY -u ANTHROPIC_AUTH_TOKEN -u GEMINI_API_KEY -u GOOGLE_API_KEY -u GOOGLE_GENAI_API_KEY
  DISABLE_AUTOUPDATER=1)

# gemini_cloud_project prints the Google Cloud project id a Gemini trial needs,
# or nothing.
#
# Gemini CLI resolves .env by walking up from its working directory and taking
# the first file it finds, and only falls back to ~/.gemini/.env when it finds
# none. The operator's project id lives in that fallback file, and this repo
# has a .env of its own, so any trial whose working directory sits inside the
# repo finds the repo file, never loads the fallback, and dies with
# ProjectIdRequiredError before it issues a single tool call. Passing the value
# in the subprocess environment reproduces exactly what the client would have
# loaded on its own outside the repo. A variable already set in the environment
# wins over the .env file (gemini-cli settings.js), so this is stable whichever
# file the client ends up finding.
gemini_cloud_project() {
  if [[ -n "${GOOGLE_CLOUD_PROJECT:-}" ]]; then
    printf '%s' "$GOOGLE_CLOUD_PROJECT"
    return
  fi
  local f="${HOME}/.gemini/.env"
  [[ -f "$f" ]] || return 0
  sed -n 's/^[[:space:]]*GOOGLE_CLOUD_PROJECT[[:space:]]*=[[:space:]]*//p' "$f" |
    head -1 | tr -d "\"'\r"
}

# ------------------------------------------------------------------- tasks --

tasks_for_server() {
  case "$1" in
  filesystem) echo "FS-01 FS-02 FS-03 FS-04 FS-05" ;;
  playwright) echo "PW-01 PW-02 PW-03 PW-04 PW-05" ;;
  github) echo "GH-01 GH-02 GH-03 GH-04 GH-05" ;;
  esac
}

server_of_task() {
  case "$1" in
  FS-*) echo filesystem ;;
  PW-*) echo playwright ;;
  GH-*) echo github ;;
  *) echo "" ;;
  esac
}

read -r -a ALL_TASKS <<<"$(tasks_for_server "$SERVER")"
if [[ -n "$TASKS_ARG" ]]; then
  IFS=',' read -r -a SELECTED <<<"$TASKS_ARG"
else
  SELECTED=("${ALL_TASKS[@]}")
fi
for t in "${SELECTED[@]}"; do
  [[ "$(server_of_task "$t")" == "$SERVER" ]] || die "task $t does not belong to server $SERVER"
  found=0
  for a in "${ALL_TASKS[@]}"; do
    if [[ "$a" == "$t" ]]; then found=1; fi
  done
  ((found)) || die "unknown task: $t"
done

# prompt_for TASK TRIAL_ID PORT OUTPUT_DIR
# The prompt strings are verbatim from docs/tier2-task-suites.md section 2. A
# change to any of them is a task-suite MAJOR bump (section 5), so they are
# single-quoted here and the placeholders are substituted by expansion, never
# by re-typing the sentence.
prompt_for() {
  local task="$1" tid="$2" port="$3" outdir="$4" p=""
  case "$task" in
  FS-01) p='Read the file `config/app.json` and tell me the value of the `version` field.' ;;
  FS-02) p='Search the fixture directory tree for every file that contains the string `TODO` and list their paths, relative to the tree root.' ;;
  FS-03) p='Create a new file at `output/{trial_id}.txt` containing exactly the text `tier2 probe {trial_id}` and nothing else.' ;;
  FS-04) p='Get the full directory tree of the fixture root, find the file with the most lines, and write a file at `summary.txt` containing exactly two lines: the relative path of that file, then the line count.' ;;
  FS-05) p='Append a new line to `notes.txt` reading exactly `trial {trial_id} recorded`, without altering the existing line.' ;;
  PW-01) p='Go to `http://127.0.0.1:{port}/catalog.html` and tell me the price listed for the item named '"'"'Widget Pro'"'"'.' ;;
  PW-02) p='On `http://127.0.0.1:{port}/catalog.html`, find the link whose visible text is '"'"'Support'"'"' and report its href.' ;;
  PW-03) p='Go to `http://127.0.0.1:{port}/landing.html`, click the link labeled '"'"'Docs'"'"', and report the title of the page you land on.' ;;
  PW-04) p='On `http://127.0.0.1:{port}/form.html`, fill the '"'"'Email'"'"' field with `agent-{trial_id}@example.com` and the '"'"'Message'"'"' field with `loadline tier2 probe`, submit the form, and report the confirmation text shown.' ;;
  PW-05) p='On `http://127.0.0.1:{port}/catalog.html`, take a screenshot of the page and save it to `{output_dir}/{trial_id}.png`.' ;;
  GH-01) p='In the `lopster568/loadline-tier2-fixture` repo, read `NOTES.md` on the default branch and tell me the value on the line starting with `TARGET:`.' ;;
  GH-02) p='In `lopster568/loadline-tier2-fixture`, find the open issue labeled `tier2-fixture` and report its exact title.' ;;
  GH-03) p='In `lopster568/loadline-tier2-fixture`, create a new issue titled exactly `tier2-probe-{trial_id}` with body `automated probe, safe to close`.' ;;
  GH-04) p='In `lopster568/loadline-tier2-fixture`, create a new branch named `tier2/{trial_id}` from `main`, add a file at `probe/{trial_id}.txt` containing exactly `ok` on that branch, and open a pull request from it into `main` titled `tier2 probe {trial_id}`.' ;;
  GH-05) p='In `lopster568/loadline-tier2-fixture`, list the files under `docs/` and report their names.' ;;
  *) die "no prompt for task $task" ;;
  esac
  p="${p//\{trial_id\}/$tid}"
  p="${p//\{port\}/$port}"
  p="${p//\{output_dir\}/$outdir}"
  printf '%s' "$p"
}

# --------------------------------------------------------------- fixtures ---

PORT="${LOADLINE_TIER2_HTTP_PORT:-8930}"
PW_BROWSER="${LOADLINE_TIER2_PW_BROWSER:-chromium}"
FIXTURES="${repo}/tier2/fixtures"
GH_REPO="${LOADLINE_TIER2_GH_REPO:-lopster568/loadline-tier2-fixture}"

# GitHub state checks poll rather than read once; see check_task GH-03.
GH_CHECK_ATTEMPTS="${LOADLINE_TIER2_GH_CHECK_ATTEMPTS:-15}"
GH_CHECK_INTERVAL="${LOADLINE_TIER2_GH_CHECK_INTERVAL:-2}"

# Suite-lifetime private directory. Nothing in it is an output; it exists so the
# GitHub credential has somewhere to live that is neither an environment
# variable the client can see nor a file inside the published run tree.
SECRET_DIR=""
GH_TOKEN_FILE=""
cleanup_secrets() {
  [[ -n "$SECRET_DIR" && -d "$SECRET_DIR" ]] && rm -rf "$SECRET_DIR"
  return 0
}
trap cleanup_secrets EXIT

# prepare_github_credential writes the 0600 env-file the container reads.
#
# The token comes from LOADLINE_TIER2_GITHUB_TOKEN if the operator set one, and
# otherwise from `gh auth token`, which is the same credential reset.sh and the
# GH-03/GH-04 state checks already use. Taking it from gh rather than from a
# separate PAT keeps one credential in play for the whole GitHub path, so there
# is one thing to revoke.
prepare_github_credential() {
  local tok=""
  if [[ -n "${LOADLINE_TIER2_GITHUB_TOKEN:-}" ]]; then
    tok="$LOADLINE_TIER2_GITHUB_TOKEN"
  else
    command -v gh >/dev/null 2>&1 || die "gh CLI is required for the github suite"
    tok="$(gh auth token 2>/dev/null || true)"
  fi
  [[ -n "$tok" ]] || die "no GitHub token: set LOADLINE_TIER2_GITHUB_TOKEN or authenticate gh"
  SECRET_DIR="$(mktemp -d)"
  chmod 700 "$SECRET_DIR"
  GH_TOKEN_FILE="${SECRET_DIR}/github.env"
  ( umask 077 && printf 'GITHUB_PERSONAL_ACCESS_TOKEN=%s\n' "$tok" >"$GH_TOKEN_FILE" )
  chmod 600 "$GH_TOKEN_FILE"
}

# server_argv SCRATCH OUTDIR  -> prints the server argv, one element per line.
server_argv() {
  local scratch="$1" outdir="$2"
  case "$SERVER" in
  filesystem)
    printf '%s\n' npx -y "@modelcontextprotocol/server-filesystem@${FS_PKG_VERSION}" "$scratch"
    ;;
  playwright)
    # --browser is not optional on this box, and the reason is a real fault the
    # 2026-08-18 playwright shakedown hit: @playwright/mcp defaults to the
    # `chrome` channel, meaning branded Google Chrome at /opt/google/chrome,
    # which is not installed. Every tool call failed with "Chromium
    # distribution 'chrome' is not found" while the server itself started and
    # listed all 24 tools, so the fault presents as a task failure rather than
    # as a setup error. LOADLINE_TIER2_PW_BROWSER overrides the choice for a
    # machine that has something else.
    #
    # --output-dir points the server's own artifact directory at the trial's
    # output directory, so PW-05's screenshot lands where the state check reads
    # it whether the model passes an absolute path or a bare filename.
    printf '%s\n' npx -y "@playwright/mcp@${PW_PKG_VERSION}" \
      --headless --isolated \
      --browser "$PW_BROWSER" \
      --output-dir "$outdir"
    ;;
  github)
    # Overridable because the official server ships as a container and as a
    # Go binary, and which one the operator has is a machine fact, not a
    # methodology decision. Whatever is used is recorded in the log header.
    #
    # The default is the ghcr container, and the credential reaches it by
    # --env-file, not by -e. -e GITHUB_PERSONAL_ACCESS_TOKEN forwards the
    # variable from the docker CLI's own environment, and the docker CLI here
    # is a grandchild of the client process, so that spelling requires the
    # token to be present in the client's environment. Tier 2's rule is the
    # opposite: the token exists only in the server's environment. --env-file
    # reads it from a 0600 file created for the suite and removed on exit, so
    # the token is never in the client environment, never in an argv any ps
    # listing can read, never in the MCP config file the client parses, and
    # never in the manifest.
    if [[ -n "${LOADLINE_TIER2_GITHUB_CMD:-}" ]]; then
      # shellcheck disable=SC2086
      printf '%s\n' ${LOADLINE_TIER2_GITHUB_CMD}
    else
      printf '%s\n' docker run -i --rm \
        --env-file "$GH_TOKEN_FILE" \
        ghcr.io/github/github-mcp-server stdio
    fi
    ;;
  esac
}

# resolve_pkg_versions pins the npm package the trials launch. npx would
# otherwise resolve "latest" independently per trial, which can silently
# change the server under test mid-suite.
resolve_pkg_versions() {
  FS_PKG_VERSION="${LOADLINE_TIER2_FS_PKG_VERSION:-}"
  PW_PKG_VERSION="${LOADLINE_TIER2_PW_PKG_VERSION:-}"
  case "$SERVER" in
  filesystem)
    if [[ -z "$FS_PKG_VERSION" ]]; then
      FS_PKG_VERSION="$(npm view @modelcontextprotocol/server-filesystem version 2>/dev/null || true)"
      [[ -n "$FS_PKG_VERSION" ]] || die "could not resolve @modelcontextprotocol/server-filesystem version; set LOADLINE_TIER2_FS_PKG_VERSION"
    fi
    ;;
  playwright)
    if [[ -z "$PW_PKG_VERSION" ]]; then
      PW_PKG_VERSION="$(npm view @playwright/mcp version 2>/dev/null || true)"
      [[ -n "$PW_PKG_VERSION" ]] || die "could not resolve @playwright/mcp version; set LOADLINE_TIER2_PW_PKG_VERSION"
    fi
    ;;
  esac
  FS_PKG_VERSION="${FS_PKG_VERSION:-n/a}"
  PW_PKG_VERSION="${PW_PKG_VERSION:-n/a}"
}

# prepare_suite_fixture runs once per suite, before the first trial.
prepare_suite_fixture() {
  case "$SERVER" in
  filesystem)
    "${FIXTURES}/filesystem/setup.sh" --verify >/dev/null ||
      die "filesystem seed tree has drifted from setup.sh; fix it before running a suite"
    ;;
  playwright)
    "${FIXTURES}/playwright/serve.sh" --check >/dev/null 2>&1 ||
      die "playwright fixture pages are not served on 127.0.0.1:${PORT}; start tier2/fixtures/playwright/serve.sh first"
    # Browser preflight. Without it the missing-browser fault of the 2026-08-18
    # shakedown is silent at setup time and surfaces five trials later as five
    # task failures, because @playwright/mcp starts, lists all 24 tools, and
    # only fails when a tool call actually needs a browser. A run that cannot
    # possibly succeed should stop before it burns plan quota.
    # install-browser is idempotent and returns success when the browser is
    # already present, so a failure here is a real one and stops the run.
    npx -y "@playwright/mcp@${PW_PKG_VERSION}" install-browser "$PW_BROWSER" >/dev/null 2>&1 ||
      die "the '${PW_BROWSER}' browser is not installed and could not be installed; every playwright trial would fail on the first tool call. Install it with: npx @playwright/mcp@${PW_PKG_VERSION} install-browser ${PW_BROWSER}"
    ;;
  github)
    prepare_github_credential
    "${FIXTURES}/github/reset.sh" ||
      die "github fixture is not at baseline and reset.sh could not fix it"
    ;;
  esac
  # Resolved once here, before the first trial, and reused as the constant
  # server_image_digest value for the run header and every trial row. A
  # container server's tag is mutable (docs/tier2-task-suites.md 7.4), so this
  # is the run's only record of which build actually ran. write_footer
  # re-resolves it after the last trial to catch a tag that moved mid-run.
  SERVER_IMAGE_DIGEST="$(resolve_current_server_image_digest)"
}

# prepare_trial_fixture TASK SCRATCH runs before every trial.
prepare_trial_fixture() {
  local task="$1" scratch="$2"
  case "$SERVER" in
  filesystem)
    rm -rf "$scratch"
    mkdir -p "$(dirname "$scratch")"
    cp -a "${FIXTURES}/filesystem/seed" "$scratch"
    ;;
  playwright)
    mkdir -p "$scratch"
    "${FIXTURES}/playwright/serve.sh" --check >/dev/null 2>&1 ||
      die "playwright fixture pages stopped being served mid-suite"
    ;;
  github)
    mkdir -p "$scratch"
    # A reset before every trial, not once per suite as spec section 2.3
    # originally said. Two reasons, both learned on 2026-08-18. Keying created
    # objects on {trial_id} keeps trials from colliding, but it does not keep a
    # read-only task from reading a repo that a previous mutating trial left
    # dirty: GH-02 asks for "the open issue labeled tier2-fixture" and GH-05
    # for the contents of docs/, and both are stated against a baseline that
    # once-per-suite cleanup does not restore between them. Second, the
    # cleanup path in reset.sh was the one piece of the GitHub fixture that had
    # never executed (OPEN 5), and running it every trial is what exercises it.
    # It costs about a second of API round trips per trial.
    "${FIXTURES}/github/reset.sh" >/dev/null ||
      die "github fixture drifted from baseline before ${task}; reset.sh could not fix it"
    ;;
  esac
}

# ------------------------------------------------------------ success check --
#
# One function per task, mechanical, exactly as written in the fixture READMEs.
# $1 task, $2 trial_id, $3 transcript path, $4 scratch dir, $5 output dir.
# Prints one line of evidence on stdout; exit status is the check.

# FS-04 normalization, suite 1.0.0, spec section 2.1. Both functions are pure
# string rewrites with no knowledge of the expected answer: they must not be
# able to turn a wrong answer into a right one, only an equivalent spelling of
# the right one into the canonical spelling.
#
# fs04_normalize_path: trims whitespace, strips one layer of surrounding
# backticks or quotes, drops a trailing comma or period, drops one leading
# "./", and reduces an absolute or otherwise-prefixed path to its trailing
# "logs/access.log" component when it ends in exactly that path segment. It
# does not touch the path's own characters otherwise, so "logs/other.log" stays
# "logs/other.log" and fails.
fs04_normalize_path() {
  local p="$1"
  p="${p#"${p%%[![:space:]]*}"}"
  p="${p%"${p##*[![:space:]]}"}"
  p="${p#[\`\"\']}"
  p="${p%[\`\"\']}"
  p="${p%[.,]}"
  p="${p#./}"
  # Any prefix ending in a slash is dropped only when what remains is the exact
  # fixture path, which is the "absolute path suffix" form section 2.1 accepts.
  if [[ "$p" == */logs/access.log ]]; then p="logs/access.log"; fi
  printf '%s' "$p"
}

# fs04_normalize_count: trims whitespace, strips one layer of surrounding
# backticks or quotes, and removes comma thousands separators. The result must
# be bare digits; anything else (a word, a unit, a range) is returned unchanged
# and therefore fails the comparison.
fs04_normalize_count() {
  local c="$1"
  c="${c#"${c%%[![:space:]]*}"}"
  c="${c%"${c##*[![:space:]]}"}"
  c="${c#[\`\"\']}"
  c="${c%[\`\"\']}"
  c="${c//,/}"
  printf '%s' "$c"
}

check_task() {
  local task="$1" tid="$2" transcript="$3" scratch="$4" outdir="$5"
  case "$task" in
  FS-01)
    grep -Fq '4.2.1' "$transcript" && echo "transcript contains 4.2.1"
    ;;
  FS-02)
    # Suite 1.0.0 (OPEN 9): the negative clause is gone. It required the final
    # message not to contain docs/roadmap.md, which fired on a materially
    # correct answer that named the non-matching files in an explanatory
    # sentence, and passed on the same client phrasing the same finding without
    # one. It tested phrasing, not capability. What the task measures is whether
    # the client searched the tree and found the two files that carry the
    # marker, and the positive clause is the whole of that.
    grep -Fq 'docs/intro.md' "$transcript" &&
      grep -Fq 'docs/changelog.md' "$transcript" &&
      echo "transcript names docs/intro.md and docs/changelog.md"
    ;;
  FS-03)
    [[ "$(cat "${scratch}/output/${tid}.txt" 2>/dev/null)" == "tier2 probe ${tid}" ]] &&
      echo "output/${tid}.txt has the expected content"
    ;;
  FS-04)
    # Suite 1.0.0 (OPEN 9): compared after normalization, not byte for byte.
    # The old check failed a Gemini trial that wrote ./logs/access.log with the
    # correct count, which is a path-format artifact and not a capability
    # result. The normalization rule is stated in spec section 2.1 and
    # implemented in fs04_normalize_path / fs04_normalize_count below; it
    # touches path spelling and numeric adornment only. The two-line shape, the
    # identity of the file, and the value of the count are all still asserted,
    # so the genuine miscount the same client produced in another trial (100)
    # still fails.
    local -a s04=()
    local s04_file="${scratch}/summary.txt" s04_line s04_path s04_count
    [[ -f "$s04_file" ]] || return 1
    while IFS= read -r s04_line || [[ -n "$s04_line" ]]; do
      s04+=("${s04_line%$'\r'}")
    done <"$s04_file"
    while [[ ${#s04[@]} -gt 0 && -z "${s04[${#s04[@]} - 1]//[[:space:]]/}" ]]; do
      unset "s04[$((${#s04[@]} - 1))]"
    done
    [[ ${#s04[@]} -eq 2 ]] || return 1
    s04_path="$(fs04_normalize_path "${s04[0]}")"
    s04_count="$(fs04_normalize_count "${s04[1]}")"
    [[ "$s04_path" == "logs/access.log" && "$s04_count" == "137" ]] &&
      echo "summary.txt normalizes to logs/access.log and 137 (raw: ${s04[0]} | ${s04[1]})"
    ;;
  FS-05)
    [[ "$(cat "${scratch}/notes.txt" 2>/dev/null)" == "$(printf 'seed value: baseline\ntrial %s recorded' "$tid")" ]] &&
      echo "notes.txt has the seed line plus the trial line"
    ;;
  PW-01)
    grep -Fq '42.00' "$transcript" && echo "transcript contains 42.00"
    ;;
  PW-02)
    grep -Fq '/support.html' "$transcript" && echo "transcript contains /support.html"
    ;;
  PW-03)
    grep -Fq 'Tier 2 Fixture Docs' "$transcript" && echo "transcript contains the docs page title"
    ;;
  PW-04)
    grep -Fq "Thanks, agent-${tid}@example.com! Message received: loadline tier2 probe" "$transcript" &&
      echo "transcript contains the confirmation string"
    ;;
  PW-05)
    [[ -s "${outdir}/${tid}.png" ]] && echo "screenshot exists and is non-empty"
    ;;
  GH-01)
    grep -Fq '42' "$transcript" && echo "transcript contains 42"
    ;;
  GH-02)
    grep -Fq 'Fixture issue: do not close' "$transcript" && echo "transcript contains the issue title"
    ;;
  GH-03)
    # Polled, not read once. GitHub's issue list endpoint is served from a
    # cache that lags creation: measured on 2026-08-18, an issue created
    # through the REST API was invisible to repos/:r/issues?state=open for
    # about six seconds, and the 20:44 shakedown trial failed on exactly that
    # window with the issue sitting correctly in the repo and its url in the
    # client's own transcript. Polling reads state the trial already produced;
    # it does not re-run the trial, which section 6 forbids. The number of
    # polls it took is recorded in the evidence string so a slow read is
    # visible rather than smoothed away.
    local n="" attempt=0
    for ((attempt = 1; attempt <= GH_CHECK_ATTEMPTS; attempt++)); do
      n="$(gh api "repos/${GH_REPO}/issues?state=open&per_page=100" \
        --jq "[.[] | select(.title == \"tier2-probe-${tid}\")] | length" 2>/dev/null || echo 0)"
      [[ "$n" == "1" ]] && break
      sleep "$GH_CHECK_INTERVAL"
    done
    [[ "$n" == "1" ]] &&
      echo "exactly one open issue titled tier2-probe-${tid} (visible on poll ${attempt})"
    ;;
  GH-04)
    # Polled for the same reason as GH-03, and both halves of the criterion are
    # re-read each pass: the branch content and the pull request become visible
    # independently.
    local content="" pr="" attempt=0
    for ((attempt = 1; attempt <= GH_CHECK_ATTEMPTS; attempt++)); do
      content="$(gh api "repos/${GH_REPO}/contents/probe/${tid}.txt?ref=tier2/${tid}" \
        --jq '.content' 2>/dev/null | tr -d '\n' | base64 -d 2>/dev/null | tr -d '\n' || true)"
      pr="$(gh api "repos/${GH_REPO}/pulls?state=open&per_page=100" \
        --jq "[.[] | select(.head.ref == \"tier2/${tid}\" and .base.ref == \"main\" and .title == \"tier2 probe ${tid}\")] | length" 2>/dev/null || echo 0)"
      [[ "$content" == "ok" && "$pr" == "1" ]] && break
      sleep "$GH_CHECK_INTERVAL"
    done
    [[ "$content" == "ok" && "$pr" == "1" ]] &&
      echo "branch file is ok and one matching open pull request exists (visible on poll ${attempt})"
    ;;
  GH-05)
    grep -Fq 'alpha.md' "$transcript" && grep -Fq 'beta.md' "$transcript" &&
      echo "transcript lists alpha.md and beta.md"
    ;;
  *) die "no success check for task $task" ;;
  esac
}

# fixture_extra_paths TASK TRIAL_ID SCRATCH
#
# Prints a JSON array of paths in the filesystem scratch copy that are neither
# seed-tree members nor the task's own expected output. The client's working
# directory is the fixture root (see run_trial), so anything a client decides
# to drop in its cwd lands in the tree the checks read. An empty array is the
# evidence that the trial measured the fixture and not the fixture plus junk;
# a non-empty one is recorded rather than repaired, because a client writing
# into the tree is a finding about the client.
fixture_extra_paths() {
  local task="$1" tid="$2" scratch="$3" expected actual extra
  if [[ "$SERVER" != "filesystem" || ! -d "$scratch" ]]; then
    echo '[]'
    return
  fi
  expected="$(cd "${FIXTURES}/filesystem/seed" && find . -mindepth 1 -printf '%P\n' | sort)"
  case "$task" in
  FS-03) expected="$(printf '%s\noutput\noutput/%s.txt\n' "$expected" "$tid" | sort)" ;;
  FS-04) expected="$(printf '%s\nsummary.txt\n' "$expected" | sort)" ;;
  esac
  actual="$(cd "$scratch" && find . -mindepth 1 -printf '%P\n' | sort)"
  extra="$(comm -13 <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") || true)"
  if [[ -z "$extra" ]]; then
    echo '[]'
  else
    printf '%s\n' "$extra" | jq -R . | jq -s -c .
  fi
}

check_kind() {
  case "$1" in
  FS-01 | FS-02 | PW-01 | PW-02 | PW-03 | PW-04 | GH-01 | GH-02 | GH-05) echo transcript ;;
  *) echo state ;;
  esac
}

# --------------------------------------------------------------- versions ---

client_version() {
  case "$1" in
  claude) claude --version 2>/dev/null | head -1 | tr -d '\r' ;;
  gemini) gemini --version 2>/dev/null | head -1 | tr -d '\r' ;;
  esac
}

versions_json() {
  jq -n \
    --arg claude "$(command -v claude >/dev/null 2>&1 && client_version claude || echo 'not installed')" \
    --arg gemini "$(command -v gemini >/dev/null 2>&1 && client_version gemini || echo 'not installed')" \
    '{claude: $claude, gemini: $gemini}'
}

# ------------------------------------------------------------------ layout --

RUN_DIR="${OUT_ROOT}/${DATE}"
CLIENT_DIR="${RUN_DIR}/${CLIENT}"
WORK_ROOT="${CLIENT_DIR}/work"
SCRATCH_ROOT="${CLIENT_DIR}/scratch"
MANIFEST="${RUN_DIR}/manifest.jsonl"
BIN_DIR="${repo}/tier2/bin"
INTERPOSER="${BIN_DIR}/loadline-interposer"
LOADLINE="${BIN_DIR}/loadline"

build_binaries() {
  mkdir -p "$BIN_DIR"
  # The interposer is its own Go module, so it builds from inside its own
  # directory. Building it from the repo root fails with "main module does not
  # contain package".
  (cd "${repo}/interposer" && go build -o "$INTERPOSER" ./cmd/loadline-interposer) ||
    die "interposer build failed"
  (cd "$repo" && go build -o "$LOADLINE" ./cmd/loadline) || die "loadline build failed"
}

# The resume key is (suite_version, server, client, task, trial), not the four
# fields spec section 6 originally named. suite_version belongs in it: a task
# suite MAJOR bump makes prior rows incomparable by definition (section 5), so a
# re-invocation after a bump must re-run the cell rather than resume over rows
# recorded under the criteria the bump replaced. Found when the 1.0.0 criteria
# fixes needed revalidating on the same date as the 0.1.2 shakedown they came
# from.
already_done() {
  local task="$1" n="$2"
  ((FORCE)) && return 1
  [[ -f "$MANIFEST" ]] || return 1
  jq -s -e --arg v "$SUITE_VERSION" --arg s "$SERVER" --arg c "$CLIENT" \
    --arg t "$task" --argjson n "$n" \
    'any(.[]; .type == "trial" and .suite_version == $v and .server == $s
               and .client == $c and .task == $t and .trial == $n)' \
    "$MANIFEST" >/dev/null 2>&1
}

# ------------------------------------------------------------------- trial --

# run_trial TASK N SEQ
run_trial() {
  local task="$1" n="$2" seq="$3"
  local tid work scratch outdir logfile clientjson analyzejson transcript promptfile
  tid="$(openssl rand -hex 4 2>/dev/null || head -c4 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  work="${WORK_ROOT}/${task}-t${n}"
  scratch="${SCRATCH_ROOT}/${task}-t${n}"
  outdir="${work}/out"
  logfile="${CLIENT_DIR}/${task}-t${n}.jsonl"
  clientjson="${CLIENT_DIR}/${task}-t${n}.client.json"
  analyzejson="${CLIENT_DIR}/${task}-t${n}.analyze.json"
  transcript="${work}/transcript.txt"
  promptfile="${work}/prompt.txt"

  rm -rf "$work"
  mkdir -p "$work" "$outdir"
  rm -f "$logfile"
  prepare_trial_fixture "$task" "$scratch"

  local prompt
  prompt="$(prompt_for "$task" "$tid" "$PORT" "$outdir")"
  printf '%s\n' "$prompt" >"$promptfile"

  # Per-trial MCP config: the server command wrapped in the interposer, with
  # this trial's log path. Absolute paths throughout, because the client may
  # launch the server from a directory the runner did not pick
  # (interposer/README.md).
  local -a srv=()
  while IFS= read -r line; do srv+=("$line"); done < <(server_argv "$scratch" "$outdir")
  local -a iargs=("--log" "$logfile")
  if ((FULL_RESULTS)); then iargs+=("--full-results"); fi
  iargs+=("--")

  # Built by piping the argv through jq rather than with --args, because jq
  # parses a positional beginning with "--" as one of its own options.
  local args_json server_json
  args_json="$(printf '%s\n' "${iargs[@]}" "${srv[@]}" | jq -R -n -c '[inputs]')"
  server_json="$(jq -n --arg cmd "$INTERPOSER" --argjson args "$args_json" \
    '{command: $cmd, args: $args}')"

  # The client's working directory is the fixture root for a filesystem trial.
  # This is not cosmetic. Both clients advertise the MCP roots capability and
  # send their working directory as the only root, and
  # @modelcontextprotocol/server-filesystem replaces its command-line allowed
  # directories with the client's roots when it gets them. Launching the client
  # anywhere else makes the server refuse every path inside the scratch copy
  # ("Access denied - path outside allowed directories"), which is what the
  # first shakedown trial hit. It also makes the section 2.1 prompts' relative
  # paths resolve where they are meant to. Nothing the runner writes goes into
  # that directory: the MCP config, the prompt, the transcript and the client
  # output all live in the sibling work directory, so the fixture tree the
  # client sees is exactly the seed tree.
  local cwd="$work"
  if [[ "$SERVER" == "filesystem" ]]; then cwd="$scratch"; fi

  local started ended wall exit_code=0 timed_out=false
  started="$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)"
  local t0 t1
  t0="$(date +%s%3N)"

  if [[ "$CLIENT" == "claude" ]]; then
    jq -n --arg name "$SERVER" --argjson s "$server_json" '{mcpServers: {($name): $s}}' \
      >"${work}/mcp.json"
    set +e
    (cd "$cwd" && timeout -k 10 "$TIMEOUT" "${NOENV[@]}" claude -p "$prompt" \
      --mcp-config "${work}/mcp.json" \
      --strict-mcp-config \
      --setting-sources "$CLAUDE_SETTING_SOURCES" \
      --max-turns "$MAX_TURNS" \
      --output-format json \
      --permission-mode dontAsk \
      --allowedTools "mcp__${SERVER}__*" \
      --disallowedTools "$CLAUDE_DISALLOWED_TOOLS" \
      --model "$MODEL" \
      >"$clientjson" 2>"${work}/client.stderr")
    exit_code=$?
    set -e
  else
    # Gemini CLI has no --mcp-config equivalent, and there is no override for
    # the user or workspace settings layer (spec 7.2). GEMINI_CLI_SYSTEM_SETTINGS_PATH
    # redirects the system layer, which is the one layer that can be pointed
    # at a per-trial file, and it is what keeps a .gemini/ directory out of the
    # fixture tree that the same trial has to use as its working directory.
    #
    # --skip-trust is mandatory from 0.55 on and is not optional hygiene. A
    # trial's working directory is a freshly created scratch or work directory
    # that no operator has ever trusted, and on an untrusted directory the
    # client refuses the headless run outright: exit 55,
    # FatalUntrustedWorkspaceError, nothing on stdout, so the trial records as
    # client_error and measures nothing. Two lesser effects fire first on the
    # same check and are worth naming because either one alone would corrupt a
    # trial silently: --approval-mode yolo is downgraded to "default", and
    # every MCP server is disabled, including the one this suite exists to
    # measure ("MCP servers are configured but disabled because this folder is
    # untrusted"). The flag's whole implementation is to set
    # GEMINI_CLI_TRUST_WORKSPACE=true at argument-parse time, which is the
    # env-var form of the same switch and is checked first and unconditionally;
    # both were verified working on 0.55.1. The flag is used here because it
    # lands in the recorded argv where a reader will look for it.
    jq -n --arg name "$SERVER" --argjson s "$server_json" \
      --argjson turns "$MAX_TURNS" --argjson core "$GEMINI_CORE_TOOLS" \
      --argjson envexcl "$GEMINI_EXCLUDED_ENV_VARS" \
      '{mcpServers: {($name): $s}, model: {maxSessionTurns: $turns}, tools: {core: $core},
        advanced: {excludedEnvVars: $envexcl}}' \
      >"${work}/gemini-settings.json"
    local -a gemini_env=(GEMINI_CLI_SYSTEM_SETTINGS_PATH="${work}/gemini-settings.json")
    if [[ -n "$GEMINI_CLOUD_PROJECT" ]]; then
      gemini_env+=(GOOGLE_CLOUD_PROJECT="$GEMINI_CLOUD_PROJECT")
    fi
    set +e
    (cd "$cwd" && env "${gemini_env[@]}" \
      timeout -k 10 "$TIMEOUT" "${NOENV[@]}" gemini -p "$prompt" \
      --output-format json \
      --approval-mode yolo \
      --skip-trust \
      --allowed-mcp-server-names "$SERVER" \
      --model "$MODEL" \
      >"$clientjson" 2>"${work}/client.stderr")
    exit_code=$?
    set -e
  fi

  t1="$(date +%s%3N)"
  ended="$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)"
  wall=$((t1 - t0))
  if [[ "$exit_code" == "124" || "$exit_code" == "137" ]]; then timed_out=true; fi

  # Transcript: the client's own final assistant-facing message, which is what
  # every section 2 transcript check matches against.
  # client_ok is "the client produced a parseable result object". client_failed
  # is the wider question the classifier actually needs: did this trial fail
  # inside the client or the API rather than at the model's own hand. The two
  # are separate because a trial can hand back a perfectly valid JSON document
  # that reports its own failure, and only the first of those two facts should
  # switch off the token and cache recording below.
  #
  # Found on 2026-08-19, first re-shakedown of the upgraded Gemini client.
  # PW-02 came back with a valid document, four successful tool calls on the
  # wire, a zero tool-call gap, an empty response string, and this:
  #   "error": {"type": "INVALID_STREAM", "message": "The model returned an
  #    empty response with no text or thoughts. This may be a transient API
  #    issue; please try again."}
  # The transcript check then failed against an empty string and the classifier
  # bucketed it tool_use_failed, which publishes as a capability claim about
  # @playwright/mcp on the strength of a transient API fault. It is the same
  # class of defect as the client_error bucket already existing at all, reached
  # by a route the original rule did not cover: the JSON parsed, so nothing
  # noticed.
  local client_ok=true client_failed=false
  if ! jq -e . "$clientjson" >/dev/null 2>&1; then
    client_ok=false
    client_failed=true
    : >"$transcript"
  elif [[ "$CLIENT" == "claude" ]]; then
    jq -r '.result // ""' "$clientjson" >"$transcript"
  else
    jq -r '.response // ""' "$clientjson" >"$transcript"
    if jq -e '.error != null' "$clientjson" >/dev/null 2>&1; then client_failed=true; fi
  fi

  # Success check, run immediately so a write task's mutated state cannot leak
  # into a later trial's baseline (section 6 step 3).
  local success=false evidence=""
  set +e
  if evidence="$(check_task "$task" "$tid" "$transcript" "$scratch" "$outdir" 2>/dev/null)"; then
    success=true
  fi
  set -e
  [[ -n "$evidence" ]] || evidence="check failed"

  local extra_paths
  extra_paths="$(fixture_extra_paths "$task" "$tid" "$scratch")"

  # Analyze the frame log.
  local analyze_ok=true
  if [[ -s "$logfile" ]]; then
    "$LOADLINE" analyze --log "$logfile" --out "$analyzejson" >/dev/null 2>&1 || analyze_ok=false
  else
    analyze_ok=false
    jq -n '{}' >"$analyzejson"
  fi

  local tool_calls=0 analyze_block='null' interposer_version="" analyzer_version=""
  if [[ "$analyze_ok" == true ]] && jq -e .totals "$analyzejson" >/dev/null 2>&1; then
    tool_calls="$(jq -r '.totals.tool_calls' "$analyzejson")"
    interposer_version="$(jq -r '.interposer_version // ""' "$analyzejson")"
    analyzer_version="$(jq -r '.analyzer_version // ""' "$analyzejson")"
    analyze_block="$(jq -c '{
      sessions: .sessions,
      tool_calls: .totals.tool_calls,
      tool_call_arg_tokens: .totals.tool_call_arg_tokens,
      tool_call_arg_bytes: .totals.tool_call_arg_bytes,
      tool_call_result_tokens: .totals.tool_call_result_tokens,
      tool_call_result_tokens_meta_stripped: .totals.tool_call_result_tokens_meta_stripped,
      tool_call_result_bytes: .totals.tool_call_result_bytes,
      requests: .totals.requests,
      responses: .totals.responses,
      jsonrpc_errors: .totals.jsonrpc_errors,
      tool_errors: .totals.tool_errors,
      retries: .totals.retries,
      bytes: .totals.bytes,
      wire_duration_ms: .wall.duration_ms,
      tools: [.tool_calls[].tool],
      warnings: (.warnings // [])
    }' "$analyzejson")"
  fi

  # OPEN 7 classification, per spec section 4.1. See classify_trial's own
  # comment for the precedence rule and the finding that motivated it.
  # client_failed is recorded on the trial as client_reported_error below
  # regardless of which bucket this returns, so the evidence a client reported
  # its own failure is never lost even when it does not decide the bucket.
  local classify_out classification matched
  classify_out="$(classify_trial "$client_failed" "$success" "$tool_calls" "$transcript")"
  classification="$(head -n1 <<<"$classify_out")"
  matched="$(tail -n1 <<<"$classify_out")"

  # OPEN 3 cache figures, from the client's own JSON.
  local cache_block='null' client_reported='null' resolved_model="$MODEL" denials=0
  local models_used='[]'
  if [[ "$client_ok" == true ]]; then
    if [[ "$CLIENT" == "claude" ]]; then
      cache_block="$(jq -c '{
        basis: "claude",
        cache_creation_input_tokens: (.usage.cache_creation_input_tokens // null),
        cache_read_input_tokens: (.usage.cache_read_input_tokens // null),
        input_tokens: (.usage.input_tokens // null),
        output_tokens: (.usage.output_tokens // null)
      }' "$clientjson")"
      client_reported="$(jq -c '{
        num_turns: (.num_turns // null),
        duration_ms: (.duration_ms // null),
        duration_api_ms: (.duration_api_ms // null),
        total_cost_usd: (.total_cost_usd // null),
        is_error: (.is_error // null),
        stop_reason: (.stop_reason // null),
        session_id: (.session_id // null),
        permission_denials: ((.permission_denials // []) | length)
      }' "$clientjson")"
      denials="$(jq -r '((.permission_denials // []) | length)' "$clientjson")"
      # The answering model is the one that produced the output tokens, not the
      # first key in modelUsage. Claude Code runs a small auxiliary model
      # alongside the pinned one in the same trial, and `keys | first` sorts
      # alphabetically, so on 2.1.235 every trial run at --model claude-sonnet-5
      # recorded resolved_model claude-haiku-4-5-20251001 off the back of a
      # 19-output-token side pass while the 812-output-token answer came from
      # sonnet. Found by the summarizer, which surfaced the wrong model on the
      # face of the run.
      resolved_model="$(jq -r '
        ((.modelUsage // {}) | to_entries
         | max_by(.value.outputTokens // 0) | .key) // empty' "$clientjson")"
      [[ -n "$resolved_model" ]] || resolved_model="$MODEL"
      # Every model the trial actually billed, so the auxiliary pass is visible
      # rather than folded into the pinned model's figure.
      models_used="$(jq -c '[(.modelUsage // {}) | to_entries[]
        | {model: .key, input_tokens: (.value.inputTokens // null),
           output_tokens: (.value.outputTokens // null),
           cost_usd: (.value.costUSD // null)}]' "$clientjson")"
    else
      cache_block="$(jq -c '{
        basis: "gemini",
        cached: ([.stats.models[]?.tokens.cached // 0] | add // 0),
        prompt: ([.stats.models[]?.tokens.prompt // 0] | add // 0),
        candidates: ([.stats.models[]?.tokens.candidates // 0] | add // 0),
        total: ([.stats.models[]?.tokens.total // 0] | add // 0),
        models: ((.stats.models // {}) | keys)
      }' "$clientjson")"
      client_reported="$(jq -c '{
        client_tool_calls: (.stats.tools.totalCalls // null),
        client_tool_fail: (.stats.tools.totalFail // null),
        api_requests: ([.stats.models[]?.api.totalRequests // 0] | add // 0),
        api_errors: ([.stats.models[]?.api.totalErrors // 0] | add // 0),
        error: (.error // null)
      }' "$clientjson")"
      resolved_model="$(jq -r '((.stats.models // {}) | to_entries
        | max_by(.value.tokens.candidates // 0) | .key) // empty' "$clientjson")"
      [[ -n "$resolved_model" ]] || resolved_model="$MODEL"
      models_used="$(jq -c '[(.stats.models // {}) | to_entries[]
        | {model: .key, input_tokens: (.value.tokens.prompt // null),
           output_tokens: (.value.tokens.candidates // null), cost_usd: null}]' "$clientjson")"
    fi
  fi

  # Tool-call gap: what the client says it attempted, minus what the interposer
  # saw on the wire. A positive gap means the client tried to call the server
  # and the call never left the client, which is a harness or client/server
  # interop fault and not a capability result about the server.
  #
  # This exists because of the 2026-08-18 playwright shakedown. Gemini CLI
  # 0.18.4 validates a tool's arguments against the schema the server
  # advertised, and its bundled validator has no JSON Schema draft 2020-12
  # meta-schema registered. @playwright/mcp declares 2020-12 on every tool;
  # @modelcontextprotocol/server-filesystem declares draft-07. So the same
  # client that ran the filesystem suite clean failed every playwright call
  # after the first with "no schema with key or ref
  # https://json-schema.org/draft/2020-12/schema", client-side, before the
  # frame was written. On the wire that reads as one tool call and a wrong
  # answer, which the classifier would otherwise bucket as tool_use_failed: a
  # capability claim the trial does not support.
  #
  # The gap is recorded, not acted on. Whether it earns a classification bucket
  # of its own is a spec decision (section 4.1), filed as OPEN 12, not
  # something the runner decides on its own.
  local tool_call_gap='null' tool_call_gap_bare_unwired='null' harness_suspect=false
  if [[ "$client_ok" == true && "$CLIENT" == "gemini" ]]; then
    tool_call_gap="$(gemini_tool_call_gap "$logfile" "$clientjson" "$SERVER")"
    [[ -n "$tool_call_gap" ]] || tool_call_gap='null'
    if [[ "$tool_call_gap" != "null" && "$tool_call_gap" -gt 0 ]]; then harness_suspect=true; fi
    # Deliberately no harness_suspect here: see the function's comment.
    tool_call_gap_bare_unwired="$(gemini_tool_call_gap_bare_unwired "$logfile" "$clientjson" "$SERVER")"
    [[ -n "$tool_call_gap_bare_unwired" ]] || tool_call_gap_bare_unwired='null'
  fi

  local rel_log rel_client rel_analyze rel_work rel_scratch
  rel_log="${logfile#"${repo}/"}"
  rel_client="${clientjson#"${repo}/"}"
  rel_analyze="${analyzejson#"${repo}/"}"
  rel_work="${work#"${repo}/"}"
  rel_scratch="${scratch#"${repo}/"}"

  jq -n -c \
    --arg schema "$MANIFEST_SCHEMA" \
    --arg suite "$SUITE_VERSION" \
    --arg run_id "$RUN_ID" \
    --arg date "$DATE" \
    --arg server "$SERVER" \
    --arg task "$task" \
    --arg client "$CLIENT" \
    --arg model "$MODEL" \
    --arg resolved_model "$resolved_model" \
    --argjson models_used "$models_used" \
    --argjson trial "$n" \
    --argjson suite_seq "$seq" \
    --arg trial_id "$tid" \
    --arg started_at "$started" \
    --arg ended_at "$ended" \
    --argjson wall_ms "$wall" \
    --argjson exit_code "$exit_code" \
    --argjson timed_out "$timed_out" \
    --argjson client_json_valid "$client_ok" \
    --argjson success "$success" \
    --arg check_kind "$(check_kind "$task")" \
    --arg evidence "$evidence" \
    --arg classification "$classification" \
    --argjson client_reported_error "$client_failed" \
    --argjson refusal_markers "$matched" \
    --argjson tool_calls "$tool_calls" \
    --argjson tool_call_gap "$tool_call_gap" \
    --argjson tool_call_gap_bare_unwired "$tool_call_gap_bare_unwired" \
    --argjson harness_suspect "$harness_suspect" \
    --argjson permission_denials "$denials" \
    --argjson fixture_extra_paths "$extra_paths" \
    --argjson cache "$cache_block" \
    --argjson client_reported "$client_reported" \
    --argjson analyze "$analyze_block" \
    --arg interposer_version "$interposer_version" \
    --arg analyzer_version "$analyzer_version" \
    --argjson versions "$VERSIONS_START" \
    --arg server_pkg "$SERVER_PKG" \
    --arg server_image_digest "$SERVER_IMAGE_DIGEST" \
    --arg prompt "$prompt" \
    --arg log "$rel_log" --arg client_json "$rel_client" --arg analyze_json "$rel_analyze" \
    --arg work "$rel_work" --arg scratch "$rel_scratch" \
    '{type: "trial", schema: $schema, suite_version: $suite, run_id: $run_id, date: $date,
      server: $server, task: $task, client: $client, model: $model,
      resolved_model: $resolved_model, models_used: $models_used,
      trial: $trial, trial_ordinal: $trial,
      suite_seq: $suite_seq, trial_id: $trial_id,
      started_at: $started_at, ended_at: $ended_at, wall_ms: $wall_ms,
      exit_code: $exit_code, timed_out: $timed_out, client_json_valid: $client_json_valid,
      success: $success, check_kind: $check_kind, check_evidence: $evidence,
      classification: $classification, client_reported_error: $client_reported_error,
      refusal_markers: $refusal_markers,
      tool_calls: $tool_calls, tool_call_gap: $tool_call_gap,
      tool_call_gap_bare_unwired: $tool_call_gap_bare_unwired,
      harness_suspect: $harness_suspect, permission_denials: $permission_denials,
      fixture_extra_paths: $fixture_extra_paths,
      cache: $cache, client_reported: $client_reported, analyze: $analyze,
      interposer_version: $interposer_version, analyzer_version: $analyzer_version,
      client_versions: $versions, server_pkg: $server_pkg,
      server_image_digest: (if $server_image_digest == "" then null else $server_image_digest end),
      rendered_prompt: $prompt,
      artifacts: {log: $log, client_json: $client_json, analyze: $analyze_json,
                  work: $work, scratch: $scratch}}' >>"$MANIFEST"

  local gapnote=""
  if [[ "$harness_suspect" == true ]]; then gapnote="  HARNESS-SUSPECT gap=$tool_call_gap"; fi
  log "$task t$n  success=$success  class=$classification  tool_calls=$tool_calls  wall=${wall}ms  exit=$exit_code${gapnote}"
}

# -------------------------------------------------------------------- main --

resolve_pkg_versions
case "$SERVER" in
filesystem) SERVER_PKG="@modelcontextprotocol/server-filesystem@${FS_PKG_VERSION}" ;;
playwright) SERVER_PKG="@playwright/mcp@${PW_PKG_VERSION}" ;;
github) SERVER_PKG="${LOADLINE_TIER2_GITHUB_CMD:-ghcr.io/github/github-mcp-server}" ;;
esac

RUN_ID="${DATE}-${SERVER}-${CLIENT}-$(date -u +%H%M%S)"

if ((DRY_RUN)); then
  echo "run_id       $RUN_ID"
  echo "server       $SERVER ($SERVER_PKG)"
  echo "client       $CLIENT (model $MODEL)"
  echo "tasks        ${SELECTED[*]}"
  echo "trials       $TRIALS each, $((${#SELECTED[@]} * TRIALS)) total"
  echo "manifest     $MANIFEST"
  echo "logs         ${CLIENT_DIR}/<task>-t<N>.jsonl"
  for t in "${SELECTED[@]}"; do
    echo "prompt $t: $(prompt_for "$t" "deadbeef" "$PORT" "${WORK_ROOT}/${t}-t1/out")"
  done
  exit 0
fi

mkdir -p "$RUN_DIR" "$CLIENT_DIR" "$WORK_ROOT" "$SCRATCH_ROOT"
build_binaries
prepare_suite_fixture

GEMINI_CLOUD_PROJECT="$(gemini_cloud_project)"
if [[ "$CLIENT" == "gemini" && -z "$GEMINI_CLOUD_PROJECT" ]]; then
  log "no GOOGLE_CLOUD_PROJECT found in the environment or ~/.gemini/.env; a Workspace account will fail with ProjectIdRequiredError"
fi

VERSIONS_START="$(versions_json)"
INTERPOSER_VERSION="$("$INTERPOSER" --version 2>/dev/null | tr -d '\r' | head -1)"
LOADLINE_VERSION="$("$LOADLINE" version 2>/dev/null | tr -d '\r' | head -1)"

jq -n -c \
  --arg schema "$MANIFEST_SCHEMA" --arg run_id "$RUN_ID" --arg date "$DATE" \
  --arg server "$SERVER" --arg client "$CLIENT" --arg model "$MODEL" \
  --arg started_at "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)" \
  --argjson trials "$TRIALS" \
  --arg tasks "${SELECTED[*]}" \
  --argjson versions "$VERSIONS_START" \
  --arg interposer "$INTERPOSER_VERSION" --arg loadline "$LOADLINE_VERSION" \
  --arg suite "$SUITE_VERSION" --arg server_pkg "$SERVER_PKG" \
  --argjson full_results "$([[ $FULL_RESULTS -eq 1 ]] && echo true || echo false)" \
  --argjson max_turns "$MAX_TURNS" --argjson timeout_s "$TIMEOUT" \
  --arg claude_disallowed "$CLAUDE_DISALLOWED_TOOLS" --argjson gemini_core "$GEMINI_CORE_TOOLS" \
  --arg claude_setting_sources "$CLAUDE_SETTING_SOURCES" \
  --arg pw_browser "$PW_BROWSER" \
  --arg api_keys_stripped "${STRIPPED_KEYS[*]:-none}" \
  --arg server_image_digest "$SERVER_IMAGE_DIGEST" \
  '{type: "run_header", schema: $schema, run_id: $run_id, date: $date, server: $server,
    client: $client, model: $model, started_at: $started_at, trials_per_task: $trials,
    tasks: ($tasks | split(" ")), client_versions_at_start: $versions,
    interposer_version: $interposer, loadline_version: $loadline, suite_version: $suite,
    server_pkg: $server_pkg,
    server_image_digest: (if $server_image_digest == "" then null else $server_image_digest end),
    full_results: $full_results, max_turns: $max_turns,
    trial_timeout_s: $timeout_s,
    isolation: {claude_disallowed_tools: ($claude_disallowed | split(",")),
                claude_setting_sources: $claude_setting_sources,
                gemini_core_tools: $gemini_core,
                autoupdater_disabled: true},
    playwright_browser: $pw_browser,
    api_keys_stripped_from_clients: $api_keys_stripped}' >>"$MANIFEST"

log "run $RUN_ID: ${#SELECTED[@]} tasks x $TRIALS trials, $CLIENT/$MODEL against $SERVER"

EXECUTED=0
SKIPPED=0
SEQ=0

write_footer() {
  local versions_end drift=false
  versions_end="$(versions_json)"
  if [[ "$versions_end" != "$VERSIONS_START" ]]; then drift=true; fi
  # Re-resolved rather than reused: the whole reason server_image_digest
  # exists is that a container server's tag is mutable (docs/tier2-task-
  # suites.md 7.4), so re-checking here is what would actually catch the tag
  # moving mid-run, the same way re-reading the client version here is what
  # catches a mid-run auto-update.
  local image_digest_end
  image_digest_end="$(resolve_current_server_image_digest)"
  if [[ "$image_digest_end" != "$SERVER_IMAGE_DIGEST" ]]; then drift=true; fi
  jq -n -c \
    --arg schema "$MANIFEST_SCHEMA" --arg run_id "$RUN_ID" \
    --arg ended_at "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)" \
    --argjson start "$VERSIONS_START" --argjson end "$versions_end" \
    --argjson drift "$drift" \
    --arg image_digest_start "$SERVER_IMAGE_DIGEST" --arg image_digest_end "$image_digest_end" \
    --argjson executed "$EXECUTED" --argjson skipped "$SKIPPED" \
    --arg server "$SERVER" --arg client "$CLIENT" --argjson interrupted "${INTERRUPTED:-false}" \
    '{type: "run_footer", schema: $schema, run_id: $run_id, server: $server, client: $client,
      ended_at: $ended_at, client_versions_at_start: $start, client_versions_at_end: $end,
      server_image_digest_at_start: (if $image_digest_start == "" then null else $image_digest_start end),
      server_image_digest_at_end: (if $image_digest_end == "" then null else $image_digest_end end),
      version_drift: $drift, trials_executed: $executed, trials_skipped: $skipped,
      interrupted: $interrupted}' >>"$MANIFEST"
  if [[ "$drift" == true ]]; then
    log "VERSION DRIFT: a client version or the container image digest changed mid-run; this run is not comparable"
  fi
}

INTERRUPTED=false
on_interrupt() {
  INTERRUPTED=true
  write_footer
  log "interrupted after $EXECUTED trials; re-invoke with the same arguments to resume"
  exit 130
}
trap on_interrupt INT TERM

for task in "${SELECTED[@]}"; do
  for ((n = 1; n <= TRIALS; n++)); do
    if already_done "$task" "$n"; then
      SKIPPED=$((SKIPPED + 1))
      log "$task t$n already in the manifest, skipping"
      continue
    fi
    SEQ=$((SEQ + 1))
    run_trial "$task" "$n" "$SEQ"
    EXECUTED=$((EXECUTED + 1))
  done
done

write_footer
log "done: $EXECUTED executed, $SKIPPED skipped, manifest $MANIFEST"
