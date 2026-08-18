#!/usr/bin/env bash
#
# Aggregates a finished Tier 2 run day into a publishable summary.json, per
# docs/tier2-task-suites.md sections 4 and 6.
#
# Usage:
#   tier2/summarize.sh [--date YYYY-MM-DD] [--out DIR] [--stdout]
#
# Defaults: --date today (UTC), --out data/tier2, writing
# <out>/<date>/summary.json. --stdout prints to stdout and writes nothing.
#
# Input is <out>/<date>/manifest.jsonl and nothing else. That is deliberate:
# the per-trial frame logs are secrets (interposer/README.md, "Security") and
# are deleted once a run has been aggregated and spot-checked, so the summary
# has to be reconstructible from the manifest alone. Every analyze figure the
# summary reports was already copied into the manifest row by run-suite.sh at
# trial time.
#
# What it does NOT emit, and will not be extended to emit: rates, percentages,
# or any "N% success" framing. Section 3.3 of the spec is the rule and it is
# not a style preference. A cell is 3 trials. Three trials support "3 trials,
# this many succeeded, here are the three numbers" and nothing beyond it. Every
# per-cell distribution below therefore carries its raw values alongside the
# median, so a reader never has to take a summary statistic on trust at n = 3.

set -euo pipefail

DATE="$(date -u +%F)"
OUT_ROOT=""
TO_STDOUT=0

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "${here}/.." && pwd)"

die() {
  echo "summarize: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
  --date)
    DATE="${2:-}"
    shift 2
    ;;
  --out)
    OUT_ROOT="${2:-}"
    shift 2
    ;;
  --stdout)
    TO_STDOUT=1
    shift
    ;;
  -h | --help)
    sed -n '3,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  *) die "unknown argument: $1" ;;
  esac
done

[[ "$DATE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || die "--date must be YYYY-MM-DD"
OUT_ROOT="${OUT_ROOT:-${repo}/data/tier2}"
RUN_DIR="${OUT_ROOT}/${DATE}"
MANIFEST="${RUN_DIR}/manifest.jsonl"
SUMMARY="${RUN_DIR}/summary.json"

command -v jq >/dev/null 2>&1 || die "jq is required and was not found on PATH"
[[ -f "$MANIFEST" ]] || die "no manifest at ${MANIFEST}"

SUMMARY_SCHEMA="tier2-summary/0.1"

# The jq program below is the whole aggregator.
#
# Trial rows are deduplicated on (suite_version, server, client, task, trial),
# keeping the last occurrence. The manifest is append-only, so a cell re-run
# with run-suite.sh --force after a harness fix leaves both rows in the file;
# the superseded one stays as the record of what the broken setup produced, and
# the summary reports the one that stands. superseded_trial_rows counts them so
# the fact that a re-run happened is on the face of the summary rather than
# buried in the manifest.
jq -s -c \
  --arg schema "$SUMMARY_SCHEMA" \
  --arg date "$DATE" \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)" \
  --arg manifest "${MANIFEST#"${repo}/"}" '

def stats:
  # A distribution reported the way section 3.3 permits: the raw values, plus
  # a median and the endpoints as a convenience. Never a rate.
  (map(select(. != null)) | map(select(type == "number"))) as $v
  | ($v | length) as $n
  | if $n == 0 then {n: 0, median: null, min: null, max: null, values: []}
    else ($v | sort) as $s
      | {n: $n,
         median: (if ($n % 2) == 1 then $s[($n / 2 | floor)]
                  else (($s[($n / 2) - 1] + $s[$n / 2]) / 2) end),
         min: $s[0], max: $s[-1], values: $v}
    end;

def cellkey: [.suite_version, .server, .client, .task] | join("|");
def trialkey: [.suite_version, .server, .client, .task, (.trial | tostring)] | join("|");

[.[] | select(.type == "trial")] as $all_trials
| ($all_trials | group_by(trialkey) | map(.[-1])) as $trials
| (($all_trials | length) - ($trials | length)) as $superseded
| [.[] | select(.type == "run_header")] as $headers
| [.[] | select(.type == "run_footer")] as $footers

| {
  schema: $schema,
  summary_version: "0.1",
  date: $date,
  generated_at: $generated_at,
  manifest: $manifest,

  reporting_rule: ("Absolute counts only, never rates or percentages "
    + "(docs/tier2-task-suites.md section 3.3). A cell is one "
    + "(server, task, client) triple; at n = 3 a cell supports "
    + "\"this many of this many trials succeeded\" and the per-trial numbers, "
    + "nothing statistical. Distributions carry their raw values for that reason."),

  suite_versions: ([$trials[].suite_version] | unique),
  interposer_versions: ([$trials[].interposer_version] | map(select(. != null and . != "")) | unique),
  analyzer_versions: ([$trials[].analyzer_version] | map(select(. != null and . != "")) | unique),
  servers: ([$trials[].server] | unique),
  clients: ([$trials[].client] | unique),
  resolved_models: ([$trials[] | {client, resolved_model}] | unique),

  # Every model a trial billed, pinned and auxiliary alike. Claude Code runs a
  # small side model in the same trial as the pinned one; folding it away would
  # understate what a task cost.
  models_billed: ([$trials[] | (.models_used // [])[] | .model] | unique),

  # A summary spanning more than one task-suite MAJOR is not one result set.
  # Section 5 forbids comparing across the bump, so the flag rides on the
  # summary rather than being left for a reader to work out from the list.
  mixed_suite_versions: (([$trials[].suite_version] | unique | length) > 1),

  # Section 5 / OPEN 8. True if any run in the day recorded a client version
  # change across its own trials. A day carrying this is not comparable and is
  # republished only by being run again.
  version_drift: ([$footers[].version_drift] | any),
  drifted_runs: [$footers[] | select(.version_drift == true)
                 | {run_id, server, client,
                    client_versions_at_start, client_versions_at_end}],

  superseded_trial_rows: $superseded,

  runs: [$headers[] as $h
    | ($footers | map(select(.run_id == $h.run_id)) | .[-1]) as $f
    | {run_id: $h.run_id, server: $h.server, client: $h.client,
       model: $h.model, suite_version: $h.suite_version,
       server_pkg: $h.server_pkg,
       interposer_version: $h.interposer_version,
       loadline_version: $h.loadline_version,
       started_at: $h.started_at, ended_at: ($f.ended_at // null),
       trials_per_task: $h.trials_per_task, tasks: $h.tasks,
       max_turns: $h.max_turns, trial_timeout_s: $h.trial_timeout_s,
       full_results: $h.full_results,
       isolation: $h.isolation,
       playwright_browser: ($h.playwright_browser // null),
       api_keys_stripped_from_clients: $h.api_keys_stripped_from_clients,
       client_versions_at_start: $h.client_versions_at_start,
       client_versions_at_end: ($f.client_versions_at_end // null),
       version_drift: ($f.version_drift // null),
       trials_executed: ($f.trials_executed // null),
       trials_skipped: ($f.trials_skipped // null),
       interrupted: ($f.interrupted // null)}],

  cells: [$trials | group_by(cellkey) | .[]
    | (. | sort_by(.suite_seq)) as $c
    | {
      suite_version: $c[0].suite_version,
      server: $c[0].server,
      task: $c[0].task,
      client: $c[0].client,
      model: $c[0].model,
      resolved_model: $c[0].resolved_model,
      check_kind: $c[0].check_kind,

      trials: ($c | length),
      successes: ([$c[] | select(.success == true)] | length),
      failures: ([$c[] | select(.success != true)] | length),

      # Section 4.1. Every bucket that occurred, as an absolute count. The
      # three zero-tool buckets are never folded together.
      classifications: ([$c[].classification] | group_by(.)
                        | map({key: .[0], value: length}) | from_entries),

      timed_out_trials: ([$c[] | select(.timed_out == true)] | length),
      client_json_invalid_trials: ([$c[] | select(.client_json_valid != true)] | length),
      permission_denial_trials: ([$c[] | select((.permission_denials // 0) > 0)] | length),
      fixture_extra_path_trials: ([$c[] | select(((.fixture_extra_paths // []) | length) > 0)] | length),
      fixture_extra_paths: ([$c[].fixture_extra_paths // []] | add | unique),

      # A trial where the client attempted a call to a tool the server had
      # advertised and the call never reached the wire. Positive means the
      # cell is measuring a client or interop fault rather than the server.
      harness_suspect_trials: ([$c[] | select(.harness_suspect == true)] | length),
      tool_call_gap: ([$c[].tool_call_gap] | stats),

      tool_calls: ([$c[].tool_calls] | stats),
      wall_ms: ([$c[].wall_ms] | stats),

      # Section 4 wire metrics, as copied into the manifest from the analyze
      # report of each trial. tool_call_result_tokens is null for a trial logged
      # without --full-results, and stats drops nulls rather than treating a
      # missing measurement as a zero.
      tool_call_arg_tokens: ([$c[].analyze.tool_call_arg_tokens] | stats),
      tool_call_result_tokens: ([$c[].analyze.tool_call_result_tokens] | stats),
      tool_call_arg_bytes: ([$c[].analyze.tool_call_arg_bytes] | stats),
      tool_call_result_bytes: ([$c[].analyze.tool_call_result_bytes] | stats),
      wire_bytes: ([$c[].analyze.bytes] | stats),
      jsonrpc_errors: ([$c[].analyze.jsonrpc_errors] | stats),
      tool_errors: ([$c[].analyze.tool_errors] | stats),
      retries: ([$c[].analyze.retries] | stats),
      tools_called: ([$c[].analyze.tools // []] | add | unique),
      analyze_warnings: ([$c[].analyze.warnings // []] | add | unique),

      # Section 3.2 / OPEN 3. Cache figures are recorded, not controlled, and
      # a cache number without its ordinal is uninterpretable, so the per-trial
      # series below carries both and the medians are a convenience on top.
      cache: {
        basis: ($c[0].cache.basis // null),
        claude: (if $c[0].cache.basis == "claude" then {
            cache_creation_input_tokens: ([$c[].cache.cache_creation_input_tokens] | stats),
            cache_read_input_tokens: ([$c[].cache.cache_read_input_tokens] | stats),
            input_tokens: ([$c[].cache.input_tokens] | stats),
            output_tokens: ([$c[].cache.output_tokens] | stats)
          } else null end),
        gemini: (if $c[0].cache.basis == "gemini" then {
            cached: ([$c[].cache.cached] | stats),
            prompt: ([$c[].cache.prompt] | stats),
            candidates: ([$c[].cache.candidates] | stats),
            total: ([$c[].cache.total] | stats)
          } else null end)
      },

      client_reported: {
        total_cost_usd: ([$c[].client_reported.total_cost_usd] | stats),
        num_turns: ([$c[].client_reported.num_turns] | stats),
        duration_ms: ([$c[].client_reported.duration_ms] | stats)
      },

      # Section 3.3 again: at n = 3 the per-trial rows are the result, and the
      # aggregates above are the convenience. Ordinals are here because a cache
      # figure is only readable next to the order the trial ran in.
      per_trial: [$c[] | {
        trial: .trial,
        suite_seq: .suite_seq,
        trial_id: .trial_id,
        started_at: .started_at,
        success: .success,
        classification: .classification,
        refusal_markers: .refusal_markers,
        check_evidence: .check_evidence,
        tool_calls: .tool_calls,
        tool_call_gap: .tool_call_gap,
        harness_suspect: .harness_suspect,
        wall_ms: .wall_ms,
        timed_out: .timed_out,
        exit_code: .exit_code,
        tool_call_arg_tokens: .analyze.tool_call_arg_tokens,
        tool_call_result_tokens: .analyze.tool_call_result_tokens,
        cache: .cache,
        interposer_version: .interposer_version,
        client_versions: .client_versions
      }]
    }],

  totals: {
    trials: ($trials | length),
    successes: ([$trials[] | select(.success == true)] | length),
    failures: ([$trials[] | select(.success != true)] | length),
    classifications: ([$trials[].classification] | group_by(.)
                      | map({key: .[0], value: length}) | from_entries),
    harness_suspect_trials: ([$trials[] | select(.harness_suspect == true)] | length),
    timed_out_trials: ([$trials[] | select(.timed_out == true)] | length),
    by_server: ([$trials | group_by(.server) | .[]
      | {key: .[0].server,
         value: {trials: length,
                 successes: ([.[] | select(.success == true)] | length)}}]
      | from_entries),
    by_client: ([$trials | group_by(.client) | .[]
      | {key: .[0].client,
         value: {trials: length,
                 successes: ([.[] | select(.success == true)] | length)}}]
      | from_entries)
  }
}' "$MANIFEST" | jq . >"${SUMMARY}.tmp"

if ((TO_STDOUT)); then
  cat "${SUMMARY}.tmp"
  rm -f "${SUMMARY}.tmp"
  exit 0
fi

mv "${SUMMARY}.tmp" "$SUMMARY"
echo "summarize: wrote ${SUMMARY}" >&2
jq -r '"  " + (.totals.trials | tostring) + " trials, "
  + (.totals.successes | tostring) + " succeeded, "
  + (.cells | length | tostring) + " cells, version_drift="
  + (.version_drift | tostring)
  + (if .mixed_suite_versions then "  MIXED SUITE VERSIONS: " + (.suite_versions | join(", ")) else "" end)
  + (if .totals.harness_suspect_trials > 0 then "  harness_suspect trials: "
       + (.totals.harness_suspect_trials | tostring) else "" end)' "$SUMMARY" >&2
