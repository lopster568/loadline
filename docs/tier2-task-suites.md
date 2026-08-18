# Tier 2 Task-Suite Specification

| Field | Value |
| --- | --- |
| Task-suite version | 0.1.1 (draft, not yet run) |
| Date | 2026-08-18 |
| Status | Draft, for Phase 2 sign-off (PRD.md 10, Open Decision 5) |
| Scope | Tier 2 dynamic runs only. Tier 1 static sweep is specified in `methodology-v0.md`. |
| Governs | Task selection, fixtures, protocol, and metrics for the 3-server dynamic tier (PRD.md 2.4) |

This document is the separate Tier 2 specification that `methodology-v0.md`'s scope line points to. It defines, concretely enough for a build agent to implement without a new design decision, the servers, the 15 tasks, the run protocol, the extracted metrics, and the versioning discipline for Tier 2.

**Two hard rules from PRD.md 2.4 govern everything below and are restated here because they bound every design choice in this document:**

1. If Tier 2 stops running, the whole leaderboard pauses. A cost-only ranking with no capability axis is worse than no ranking.
2. Results are never compared across harness versions. Harness variance can exceed model variance by a wide margin.

A third constraint, from PRD.md 5.2 and `methodology-v0.md` section 10.6, bounds how results get reported rather than how they get produced: Tier 2 covers 3 servers, enough to keep Tier 1 honest on a capability axis, not enough to generalize. Findings are absolute counts against named servers, never rates. This project has already taken public criticism on small-n framing in a sibling benchmark (jaeger-mcp-bench); section 3.3 below states the rule up front so a future release cannot repeat that mistake by omission.

---

## 1. Server selection

Tier 2 selection is a separate, smaller, capability-driven decision from Tier 1's selection rule (`server-selection.md` A.1). All three proposed servers are drawn from the ratified Tier 1 pool for continuity: the same server versions, provenance, and auth notes already researched in `server-selection.md` apply, and a Tier 2 finding for one of these servers is directly usable to remove the "modeled" label from that server's Tier 1 code-mode cell (`methodology-v0.md` 3.3).

| Server | Category | Auth | Runs on |
| --- | --- | --- | --- |
| `filesystem` | Local, deterministic | None | Either machine |
| `playwright` | Local, browser automation | None | Either machine |
| `github` | Remote, ecosystem-standard | OAuth or PAT | PC (per the build-on-PC rule for heavy/credentialed runs) |

**filesystem.** The reference `@modelcontextprotocol/server-filesystem` implementation (`server-selection.md` candidate 2). It costs nothing to run, needs no credential, and is fully deterministic: given a fixed seed tree and a fixed prompt, the tool calls a well-behaved client issues do not depend on network state, rate limits, or a third party's uptime. That determinism makes it the control server for the suite, the one where a bad trial is attributable to the client or the harness rather than to the server misbehaving. It also anchors the "read-heavy, boring" end of the task spectrum the brief asks for.

**playwright.** The official Microsoft `@playwright/mcp` server (`server-selection.md` candidate 15). It is local (headless Chromium, no external endpoint) and zero-cost, but its tool surface is qualitatively different from filesystem: navigation, DOM querying, form interaction, and screenshot capture chain multiple tool calls per task in a way filesystem tasks rarely require. It is the tool-heavy surface the brief asks for, and it is the single highest-traffic candidate in the whole Tier 1 pool (25.4M npm downloads/month), so its cost profile under real task load is a genuinely high-stakes number for the calculator.

**github.** The official `github-mcp-server` (`server-selection.md` candidate 1). It is the ecosystem's most-used server on every adoption signal `server-selection.md` gathered (32,298 stars, the corpus's clearest "most recognizable server" case), and it is the one Tier 2 server that exercises real auth, a remote API, and side effects that persist past the session, none of which filesystem or playwright can test. It is designated PC-only per the estate's build-on-PC rule (`feedback_build_on_pc.md`): it needs a live PAT/OAuth credential exported into the run environment, the same posture as the credentialed Tier 1 servers in `pc-sweep-runbook.md` section 2, and running it only from the PC keeps that credential off the lighter device.

### 1.1 Substitution rule

If a Tier 2 server is unreachable for an entire scheduled run (not a single failed trial, which is just data per section 6), or is dropped from the Tier 1 pool by a future `server-selection.md` revision, it is replaced following the same Part A rule Tier 1 uses: official-vendor status first, then adoption evidence, then the reserve bench in `server-selection.md` B.5.

- **playwright's named reserve is Chrome DevTools MCP** (`server-selection.md` B.5.3, "Browser automation backup for Playwright"). If playwright is substituted, Chrome DevTools MCP is the default replacement; task prompts in section 2.3 port over with the tool names remapped, since both surfaces are DOM navigation and extraction.
- **filesystem and github have no server-selection.md reserve in the same category as of this version.** A substitution for either applies Part A fresh against the ratified 15 plus reserve bench and is logged as a task-suite MAJOR version bump per section 5, since it invalidates comparability with every prior Tier 2 run for that slot.
- A substitution is never made mid-run to rescue a failing trial. It only happens between scheduled runs, decided before the run starts.

---

## 2. Task design

Fifteen tasks, five per server. Each task fires a single natural-language prompt at the client; the client (Claude Code or Gemini CLI) resolves it using whatever MCP tool calls it chooses, wrapped in the interposer per section 3.

**General fixture rules, all three servers:**

- Every fixture is deterministic and version-controlled. Nothing in a fixture is generated at run time except the per-trial `{trial_id}` token used to keep write tasks from colliding across repeated trials.
- `{trial_id}` is generated by the runner before each trial as an 8-character lowercase hex string and substituted into any task prompt that contains it. It is recorded in the trial's metadata (section 3.2) alongside the rendered prompt.
- Fixture source trees live under `tier2/fixtures/<server>/` in the repo, read-only. Where a task mutates state, the runner copies the fixture into a scratch location before the trial and points the server at the copy, never at the source tree.

**General success-criterion convention:** a task's success criterion is checked one of two ways, stated per task below.

- **Transcript check**, for read/search tasks with no side effect: the client's final assistant-facing message, captured from the client's own transcript or output log, contains the exact expected substring. This is a mechanical string match, not a judgment call; the expected substring is fixed by the fixture and stated per task.
- **State check**, for write and multi-tool-chaining tasks: the runner queries the actual resulting state (a file, a GitHub API object) after the trial completes and compares it against the fixed expected value. This does not depend on what the client said about its own work.

### 2.1 Filesystem tasks

Fixture source: `tier2/fixtures/filesystem/seed/`, copied to a fresh scratch directory before every trial; the filesystem server's allowed root is set to that scratch copy only.

Seed tree (fixed content, created once, never edited in place):

```
seed/
  config/app.json        {"name":"loadline-fixture","version":"4.2.1","env":"test"}
  logs/access.log        137 fixed lines, deterministic content
  docs/
    intro.md             contains one line "TODO: fill in the roadmap section"
    roadmap.md           contains no TODO marker
    changelog.md          contains one line "TODO: backfill 2025 entries"
  notes.txt              a single line: "seed value: baseline"
```

| Task | Prompt (client-facing) | Initial state | Success criterion | Exercises |
| --- | --- | --- | --- | --- |
| **FS-01** | "Read the file `config/app.json` and tell me the value of the `version` field." | Seed tree, unmodified. | Transcript check: final message contains the exact substring `4.2.1`. | Read-heavy, single tool call (`read_file`). |
| **FS-02** | "Search the fixture directory tree for every file that contains the string `TODO` and list their paths, relative to the tree root." | Seed tree, unmodified. | Transcript check: final message contains both `docs/intro.md` and `docs/changelog.md`, and does not contain `docs/roadmap.md`. | Search (`search_files` / grep-equivalent tool) over a multi-file tree. |
| **FS-03** | "Create a new file at `output/{trial_id}.txt` containing exactly the text `tier2 probe {trial_id}` and nothing else." | Seed tree, unmodified; `output/` does not exist yet. | State check: after the trial, `output/{trial_id}.txt` exists in the scratch copy with byte-exact content `tier2 probe {trial_id}` (no trailing content beyond a single trailing newline, if any, which is tolerated). | Write (`write_file`, and implicitly directory creation). |
| **FS-04** | "Get the full directory tree of the fixture root, find the file with the most lines, and write a file at `summary.txt` containing exactly two lines: the relative path of that file, then the line count." | Seed tree, unmodified. `logs/access.log` (137 lines) is the fixed correct answer; no other file in the tree exceeds it. | State check: `summary.txt` exists in the scratch copy with exactly two lines, `logs/access.log` and `137`. | Multi-tool chaining: `directory_tree` or `list_directory` recursively, `read_file` or `read_multiple_files` to compare line counts, `write_file` to record the result. |
| **FS-05** | "Append a new line to `notes.txt` reading exactly `trial {trial_id} recorded`, without altering the existing line." | Seed tree, unmodified; `notes.txt` has its one fixed line. | State check: `notes.txt` in the scratch copy has exactly two lines, the original `seed value: baseline` unchanged, followed by `trial {trial_id} recorded`. | Write via targeted edit (`edit_file`), distinct code path from FS-03's full-file write. |

### 2.2 Playwright tasks

Fixture source: `tier2/fixtures/playwright/*.html`, static, served read-only over local HTTP for the duration of a run by a static file server bound to `127.0.0.1:${LOADLINE_TIER2_HTTP_PORT}` (default `8930`; overridable if the port is taken). No per-trial copy is needed; the pages are read-only except where noted.

Fixture pages, fixed content:

```
landing.html   a page with a nav bar containing a link "Docs" -> docs.html
docs.html      <title>Tier 2 Fixture Docs</title>, plain body text
catalog.html   a <table> of products; the row for "Widget Pro" has price cell "$42.00"
               also contains a link with visible text "Support", href="/support.html"
form.html      a form with labeled fields "Email" and "Message" and a Submit button;
               client-side JS only (no server round trip) renders a confirmation div
               with id="confirmation" reading:
               "Thanks, {email}! Message received: {message}"
```

| Task | Prompt (client-facing) | Initial state | Success criterion | Exercises |
| --- | --- | --- | --- | --- |
| **PW-01** | "Go to `http://127.0.0.1:{port}/catalog.html` and tell me the price listed for the item named 'Widget Pro'." | catalog.html served, unmodified. | Transcript check: final message contains the exact substring `$42.00` (or `42.00`, both accepted). | Read-heavy, single-page extraction. |
| **PW-02** | "On `http://127.0.0.1:{port}/catalog.html`, find the link whose visible text is 'Support' and report its href." | catalog.html served, unmodified. | Transcript check: final message contains the exact substring `/support.html`. | Search over page structure (locate by visible text, not by position). |
| **PW-03** | "Go to `http://127.0.0.1:{port}/landing.html`, click the link labeled 'Docs', and report the title of the page you land on." | landing.html and docs.html served, unmodified. | Transcript check: final message contains the exact substring `Tier 2 Fixture Docs`. | Multi-tool chaining: navigate, click, read title, across a page transition. |
| **PW-04** | "On `http://127.0.0.1:{port}/form.html`, fill the 'Email' field with `agent-{trial_id}@example.com` and the 'Message' field with `loadline tier2 probe`, submit the form, and report the confirmation text shown." | form.html served, unmodified; confirmation div is hidden until submit. | Transcript check: final message contains the exact substring `Thanks, agent-{trial_id}@example.com! Message received: loadline tier2 probe`. | Write-equivalent (form fill and submit) plus multi-tool chaining (fill x2, submit, read result). |
| **PW-05** | "On `http://127.0.0.1:{port}/catalog.html`, take a screenshot of the page and save it to `{output_dir}/{trial_id}.png`." | catalog.html served, unmodified; `{output_dir}` is a per-trial scratch directory the runner creates and passes in. | State check: `{output_dir}/{trial_id}.png` exists after the trial and is a non-empty file (byte-exact image comparison is not required; screenshot rendering is not guaranteed byte-stable across runs, only existence and non-zero size are checked). | Write (file output) via a distinct tool family (`browser_take_screenshot`) from PW-04's interaction tools. |

### 2.3 GitHub tasks

Fixture: a dedicated throwaway repository, `lopster568/loadline-tier2-fixture`, public, owned by the same account the harness's PAT/OAuth grant authenticates as. Baseline state, tagged `tier2-baseline` and re-applied whenever the repo needs resetting:

```
NOTES.md            contains a line: "TARGET: 42"
docs/
  alpha.md
  beta.md
  (exactly these two files, nothing else under docs/)
one open issue, label "tier2-fixture", title exactly:
  "Fixture issue: do not close"
main branch only; no other branches; no open pull requests
```

Read-only tasks (GH-01, GH-02, GH-05) run against this baseline directly and need no reset between trials, since they do not mutate state. Write and multi-tool-chaining tasks (GH-03, GH-04) use `{trial_id}` in every created object's name so repeated trials never collide; a single cleanup pass runs once after all GH-03/GH-04 trials in a suite run complete, deleting every issue, branch, and pull request whose name matches the `tier2-probe-*` / `tier2/*` pattern, restoring the repo to baseline before the next scheduled run.

| Task | Prompt (client-facing) | Initial state | Success criterion | Exercises |
| --- | --- | --- | --- | --- |
| **GH-01** | "In the `lopster568/loadline-tier2-fixture` repo, read `NOTES.md` on the default branch and tell me the value on the line starting with `TARGET:`." | Baseline. | Transcript check: final message contains the exact substring `42`. | Read-heavy, single file read (`get_file_contents`). |
| **GH-02** | "In `lopster568/loadline-tier2-fixture`, find the open issue labeled `tier2-fixture` and report its exact title." | Baseline (exactly one issue exists, so the search is unambiguous). | Transcript check: final message contains the exact substring `Fixture issue: do not close`. | Search (issue search/list filtered by label). |
| **GH-03** | "In `lopster568/loadline-tier2-fixture`, create a new issue titled exactly `tier2-probe-{trial_id}` with body `automated probe, safe to close`." | Baseline. | State check: after the trial, querying the repo's issues for title `tier2-probe-{trial_id}` returns exactly one open issue with that title. | Write (issue creation). |
| **GH-04** | "In `lopster568/loadline-tier2-fixture`, create a new branch named `tier2/{trial_id}` from `main`, add a file at `probe/{trial_id}.txt` containing exactly `ok` on that branch, and open a pull request from it into `main` titled `tier2 probe {trial_id}`." | Baseline. | State check: after the trial, branch `tier2/{trial_id}` exists, contains `probe/{trial_id}.txt` with content `ok`, and an open pull request from that branch into `main` exists titled `tier2 probe {trial_id}`. | Multi-tool chaining: `create_branch`, `create_or_update_file`, `create_pull_request`, three distinct write calls in sequence. |
| **GH-05** | "In `lopster568/loadline-tier2-fixture`, list the files under `docs/` and report their names." | Baseline (`docs/alpha.md`, `docs/beta.md`, nothing else). | Transcript check: final message contains both `alpha.md` and `beta.md`, and does not contain any other filename under `docs/`. | Read-heavy listing, a different call shape (directory listing) from GH-01's single-file read. |

---

## 3. Protocol

### 3.1 Trial matrix

Every task runs **3 trials per client**, both clients wrapped in the interposer:

| Client | Trials per task | Tasks | Trials per client |
| --- | --- | --- | --- |
| Claude Code | 3 | 15 | 45 |
| Gemini CLI | 3 | 15 | 45 |

**90 trials per full suite run.** Each trial is one fresh client session issuing one task prompt against one MCP server, through the interposer, from a clean fixture state per section 2's per-task rules.

### 3.2 What is recorded per trial

| Field | Source | Notes |
| --- | --- | --- |
| Interposer JSONL | `--log` output for that trial's session | One file per trial; see section 6 for the path convention. Contains every request/response frame per `interposer/README.md`. |
| Client-reported token usage | Client's own session/usage output, where the client exposes it | Captured separately from the interposer log; see section 4 for why these are not the same number. |
| Wall time | Runner, timestamped around the client invocation | Start to completion (or timeout) of the client process for that trial. |
| Success/failure | Section 2's stated success criterion for the task | Boolean, plus the raw evidence used to decide it (matched substring, or the queried state). |
| Rendered prompt | Runner, after `{trial_id}` (and `{port}` / `{output_dir}` where applicable) substitution | The exact string sent to the client, not the template. |
| Interposer version | JSONL header, `interposer_version` | Section 5. |
| Task-suite version | This document's version field | Section 5. |
| Client version | Client's own `--version` output, captured at trial start | Section 5. |

### 3.3 The small-n rule

**n = 3 per cell supports per-cell reporting only.** A cell is one (server, task, client) triple. Three trials tell you what happened three times; they do not support a percentage, a confidence interval, or a claim that one client is "more reliable" than another in any statistical sense. Per-cell results are reported as what they are: 3 trials, this many succeeded, these are the per-trial numbers.

**Cross-server or cross-client comparisons require pooling trials across the relevant cells**, and even pooled, they are reported per the honest-metrics rule already in force for this project (PRD.md Success Metrics section): absolute counts, never small-n percentage framing. "2 of 3 trials failed" is reportable. "33% failure rate" is not, at this n, regardless of how many cells get pooled to produce it, unless the pooled n is large enough that a rate framing stops being misleading, a threshold this document does not set and a future release must justify explicitly if it crosses it.

This rule is stated here, not left implicit, because this project has already taken public criticism on exactly this point in a sibling benchmark (jaeger-mcp-bench, referenced in PRD.md 2.4 as the source of the interposer's own instrument debt). Restating the rule in the spec itself, rather than trusting it to survive into every downstream writeup, is the pre-emption.

---

## 4. Metrics extracted

Every metric below is labeled **MEASURED**. Nothing in Tier 2 is MODELED; that distinction exists in Tier 1 (`methodology-v0.md` section 3) because Tier 1 projects client behavior from a published figure. Tier 2 runs the actual client against the actual server and records what happened, which is the entire reason Tier 2 exists: to validate Tier 1's modeled figures against real call traffic, not to model anything of its own (`methodology-v0.md` 3.3).

| Metric | Definition | Source | Label |
| --- | --- | --- | --- |
| Tokens per completed task, schema-attributable | Tokens consumed by tool-definition schemas loaded into the client's context for this session, not by call traffic | Client-reported usage at session start, or the corresponding Tier 1 naive/progressive-mode figure for that server and client mode if the client does not expose a usable session-start figure | MEASURED |
| Tokens per completed task, call-response | Tokens consumed by the actual `tools/call` request and response traffic during the task | Computed from the interposer JSONL: `params_full` of each `tools/call` request plus `result_summary` (or `result_full` under `--full-results`) of each response, counted with the `o200k_base` tokenizer per `methodology-v0.md` 1.6, so the number is comparable to Tier 1's token basis | MEASURED |
| Tool calls per task | Count of `tools/call` request frames in the trial's JSONL | Interposer JSONL, `method == "tools/call"` frames | MEASURED |
| Argument sizes | Byte size of `params_full` per `tools/call` request; reported per call and as a per-task total | Interposer JSONL, `size_bytes` and `params_full` on request frames | MEASURED |
| Retry/error counts | Count of JSON-RPC error responses (`error` field present), plus count of `tools/call` requests with identical method and equivalent `params_full` to an immediately preceding request for the same trial (a same-trial repeat is treated as a retry) | Interposer JSONL | MEASURED |
| Decline rate | Count of trials where the client returned a final message but issued zero `tools/call` frames for a task whose success criterion requires at least one, or where the final message is an explicit refusal | Interposer JSONL (call count) cross-checked against the client's final transcript (refusal text) | MEASURED, reported as an absolute count per section 3.3, never a rate at n = 3 |

**Why the split between schema-attributable and call-response tokens matters.** The interposer measures wire traffic on the MCP stdio pipe: `tools/call` requests and responses, plus the `tools/list` exchange if the trial's client re-issues it. It does not, by itself, see whatever the client injected into the model's context as tool definitions before the first call, because that happens inside the client, not on the MCP wire. Session-start context footprint has to come from the client's own usage reporting (or, failing that, be cross-referenced against Tier 1's measured figure for that exact server, client mode, and version). Conflating the two would misattribute Tier 1's schema cost as if it were something Tier 2 measured directly, which it does not.

---

## 5. Versioning and comparability

Per `methodology-v0.md` section 9, results are comparable only within an identical (harness, methodology/suite) version pair. Tier 2 carries its own version axis for the same reason:

| Stamp | Recorded | Bump trigger |
| --- | --- | --- |
| `interposer_version` | Per `interposer/README.md`, from the JSONL header line of every trial | Interposer's own semver; any change to framing, logged fields, or `result_summary` computation is a version bump per its README |
| Task-suite version (this document) | This document's header field, recorded per trial | See below |
| Client version | Captured at trial start via the client's own `--version` (or equivalent) output | Not versioned by this project; recorded as observed |

**Task-suite version scheme**, mirroring the MAJOR/MINOR/PATCH discipline `methodology-v0.md` 9 applies to the methodology itself:

- **MAJOR**: a task's prompt, fixture, or success criterion changes in any way that changes what counts as success, a task is added or removed, or a server is substituted (section 1.1). Prior Tier 2 results for the affected task or server are not comparable across the bump.
- **MINOR**: a new task is added for an existing server without touching any existing task's prompt, fixture, or criterion. Prior results stay valid.
- **PATCH**: a wording fix, a typo correction, or a documentation clarification with no effect on the rendered prompt or the success check. Prior results stay valid.

Every published Tier 2 result carries all three stamps from the table above. A result missing any stamp is withheld, the same rule `methodology-v0.md` 1.7 applies to Tier 1 cells. Results are never compared across interposer versions or across task-suite MAJOR versions; a season-over-season Tier 2 trend line is only drawn within one (interposer MAJOR, suite MAJOR) pair.

---

## 6. Runbook stub

**Prerequisites.** Interposer built (`go build -o loadline-interposer ./interposer/cmd/loadline-interposer`), both clients installed and authenticated (Claude Code, Gemini CLI), the GitHub PAT/OAuth credential for `lopster568/loadline-tier2-fixture` exported, and the playwright fixture pages reachable at `127.0.0.1:${LOADLINE_TIER2_HTTP_PORT}` via a local static file server started before the run.

**Running one suite end to end** (per server, per client, all 5 tasks, 3 trials each):

1. Reset fixtures: fresh scratch copy of the filesystem seed tree per trial (section 2.1); confirm the playwright static server is up and serving the pinned fixture pages unmodified; confirm the GitHub fixture repo is at `tier2-baseline` with no leftover `tier2-probe-*` / `tier2/*` objects from a prior run.
2. For each of the 15 tasks, for each client, for each of 3 trials: generate `{trial_id}`, render the prompt, point the client's MCP config at the interposer wrapping the target server (per `interposer/README.md`'s Claude Code / Gemini CLI config examples), invoke the client non-interactively with the rendered prompt, and capture wall time, the interposer JSONL, client-reported usage, and client version per section 3.2.
3. Evaluate the trial's success criterion (section 2) immediately, before the next trial starts, so a write-task's mutated state does not leak into a later trial's baseline assumption.
4. After all GH-03/GH-04 trials for the run complete, run the GitHub cleanup pass (section 2.3) once.

**Expected duration.** 90 trials. A filesystem or playwright trial (local, no network round trip beyond loopback) should complete in well under a minute of client think-and-call time; a GitHub trial, authenticated and remote, budget one to two minutes. Budgeting generously for client startup and variance, a full serial run is roughly 2 to 4 hours. Trials against different servers may run concurrently, since each server's log is a separate file and `interposer/README.md` only warns against interleaving frames from concurrent sessions against the *same* log; trials against the same server should stay serial to keep that log's frame order attributable to one session at a time.

**Where outputs land.** `data/tier2/<date>/<server>/<client>/<task_id>/trial-<n>.jsonl` for each trial's interposer log, plus a sibling `trial-<n>-meta.json` carrying the section 3.2 fields (wall time, success/failure, rendered prompt, client-reported usage, all three version stamps). A `data/tier2/<date>/summary.json` aggregates all 90 trials' pass/fail and metrics per section 4, mirroring the `data/runs/<date>/` plus `data/latest.json` pattern Tier 1 already uses.

**The rule: a failed trial publishes as a failed trial.** Per the failure-as-data posture already in force for Tier 1 (`pc-sweep-runbook.md` section 5, `methodology-v0.md` section 7), a trial that times out, errors, or fails its success criterion is kept in `summary.json` exactly as it happened. It is not retried and silently replaced, not hand-edited, and not dropped from the count. If a genuine setup mistake is found mid-run (wrong port, stale credential, fixture not reset), the fix applies to the next run, not to reclassifying a trial that already happened under the broken setup.

**Interposer log handling.** Per `interposer/README.md`'s security section, every trial's JSONL contains complete tool-call arguments and, for the GitHub trials, potentially credential-adjacent material. Logs are treated as secrets: kept out of git, not attached to any published artifact, and deleted once the run they support has been aggregated into `summary.json` and the aggregation has been verified against a sample of the raw logs.

---

## 7. Runner invocation

Verified live on **2026-08-18** against the installed clients and against current vendor documentation. Both clients ship fast, so this section is evidence with a decay rate: re-verify the flags below before a suite run, and record the client version per trial (section 3.2) either way.

Installed versions at verification time: **Claude Code 2.1.234**, **Gemini CLI 0.18.4**.

### 7.1 Claude Code

Sources: `https://code.claude.com/docs/en/headless.md`, `https://code.claude.com/docs/en/cli-reference.md`, `https://code.claude.com/docs/en/mcp.md`, `https://code.claude.com/docs/en/permissions.md`, plus `claude --help` and two live print-mode runs on 2.1.234.

```sh
claude -p "<rendered prompt>" \
  --mcp-config "$TRIAL_DIR/mcp.json" \
  --strict-mcp-config \
  --max-turns 20 \
  --output-format json \
  --permission-mode dontAsk \
  --allowedTools "mcp__filesystem__*" \
  --model claude-sonnet-5 \
  > "$TRIAL_DIR/client.json"
```

| Flag | Behaviour that matters here |
| --- | --- |
| `-p` / `--print` | Print mode: run the prompt, print the response, exit. The prompt is positional; stdin is also read when no positional prompt is given. |
| `--mcp-config <configs...>` | Loads MCP servers from JSON files or inline JSON strings, space-separated. The JSON wraps servers in an `mcpServers` key, the same shape `interposer/README.md` already documents. |
| `--strict-mcp-config` | Uses only the servers from `--mcp-config` and ignores every other MCP configuration source. Confirmed empirically: with it set, the run's `system/init` event listed only the server from the passed file, and none of the machine's user-scope or plugin-bundled servers. |
| `--max-turns <n>` | Print mode only. Caps agentic turns and **exits with an error** when the cap is reached. No limit by default. |
| `--output-format <text\|json\|stream-json>` | `json` emits one structured result object. `stream-json` emits newline-delimited events whose last line is the result. |
| `--permission-mode <mode>` | Modes include `dontAsk` (denies anything not explicitly allowed, never prompts) and `bypassPermissions` (approves everything). Print mode starts in the prompting default unless one is passed, so a suite run must pass one. |
| `--allowedTools` / `--allowed-tools` | Both spellings work. MCP tools are named `mcp__<server>__<tool>`, and an allow rule may glob the tool segment as long as the server segment is literal, so `mcp__filesystem__*` allows that one server's whole surface. A fully unanchored glob such as `*` or `mcp__*` is skipped with a warning in an allow rule and approves nothing. |
| `--model <model>` | Alias or full model name. Pin it; do not let a trial inherit whatever the machine defaults to. |

**Do not use `--bare` for these trials.** It skips auto-discovery of hooks, skills, plugins, `CLAUDE.md` and MCP servers, which sounds like the right hygiene for a benchmark but also skips the project `.mcp.json`. `--strict-mcp-config` plus an explicit `--mcp-config` is the isolation this suite wants, and it keeps the passed server.

**What `--output-format json` gives section 3.2 and section 4.** Field names below are from a live 2.1.234 run, not from a documented schema block; the docs confirm the presence of `total_cost_usd` and a per-model cost breakdown but do not print the full object.

| Field | Use |
| --- | --- |
| `usage.input_tokens`, `usage.output_tokens` | Client-reported token usage, section 3.2. |
| `usage.cache_creation_input_tokens`, `usage.cache_read_input_tokens` | The cache state of the trial. This is the observable OPEN 3 needs before it can be decided either way. |
| `total_cost_usd`, `modelUsage` | Client-side cost estimate, per model. Recorded, not published as a price: PRD.md 6 rides plan quota, so this figure is an estimate of a cost the run did not actually pay. |
| `duration_ms`, `duration_api_ms`, `num_turns` | Cross-check on the runner's own wall clock and on whether `--max-turns` was reached. |
| `session_id`, `uuid`, `is_error`, `stop_reason` | Trial identity and terminal state. |
| `result` | The final assistant-facing message. This is the string every transcript check in section 2 matches against. |
| `permission_denials` | Non-empty means the trial was shaped by the permission mode rather than by the model, which is a setup fault, not a capability result. |

Client version comes from `claude --version`, or from the `claude_code_version` field on the `system/init` event under `--output-format stream-json`.

### 7.2 Gemini CLI

Sources, all fetched live at HEAD on 2026-08-18: `docs/cli/headless.md`, `docs/cli/cli-reference.md`, `docs/cli/settings.md`, `docs/reference/configuration.md`, `docs/tools/mcp-server.md`, `docs/cli/telemetry.md`, `docs/cli/tutorials/automation.md` under `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/`, plus `gemini --help` on 0.18.4.

```sh
cd "$TRIAL_DIR"   # holds .gemini/settings.json, see below
gemini -p "<rendered prompt>" \
  --output-format json \
  --approval-mode yolo \
  --allowed-mcp-server-names filesystem \
  --model flash \
  > "$TRIAL_DIR/client.json"
```

| Flag | Behaviour that matters here |
| --- | --- |
| `-p` / `--prompt` | Forces non-interactive mode. Deprecated in favour of a bare positional prompt but still functional and still used in current doc examples. A bare positional prompt is interactive in a TTY and only goes headless when stdin or stdout is not a TTY, so `-p` is the form a runner should use: it does not depend on how the harness happens to be attached. |
| `-i` / `--prompt-interactive` | Not this. It runs the prompt and then drops into the REPL. |
| `-o` / `--output-format <text\|json\|stream-json>` | Same three shapes as Claude Code. |
| `--approval-mode <default\|auto_edit\|yolo\|plan>` | `yolo` auto-approves everything. `-y` / `--yolo` is the deprecated spelling. |
| `--allowed-mcp-server-names <names...>` | Restricts which of the configured MCP servers load. This is the nearest analogue to `--strict-mcp-config`, and it is weaker: it filters a configured set rather than replacing it. |
| `-m` / `--model` | Defaults to `auto`. Pin it. See the note on `auto` below. |

**MCP wiring is by settings file, not by flag.** The `mcpServers` block lives in `~/.gemini/settings.json` or a project `.gemini/settings.json`, in the shape `interposer/README.md` already shows. Per-key detail from `docs/reference/configuration.md`: at least one of `command`, `url`, `httpUrl` is required, precedence is `httpUrl` then `url` then `command`; `timeout` defaults to 600000 ms; `trust` defaults to `false`; `excludeTools` wins over `includeTools`; and server aliases must not contain underscores, which break the `mcp_<server>_<tool>` name parsing.

**There is no flag or environment variable that redirects the user or project settings file.** `GEMINI_CLI_SYSTEM_DEFAULTS_PATH` and `GEMINI_CLI_SYSTEM_SETTINGS_PATH` redirect the system-level layers only. The consequence for the runner is concrete: a Gemini trial must run with its working directory set to a per-trial directory containing a `.gemini/settings.json` that names exactly one server, pointed at the interposer with that trial's log path. Claude Code takes the equivalent file as a `--mcp-config` argument and needs no such directory. This is the single largest asymmetry between the two runners and it belongs in the runner's design, not in a wrapper script written the morning of a run.

**The turn cap is a settings key, not a flag.** `model.maxSessionTurns` (default `-1`, unlimited) is set in the same per-trial `settings.json`. Headless mode has a dedicated exit code `53` for "turn limit exceeded". To keep the trial matrix even, set `maxSessionTurns` to the same number passed to Claude Code's `--max-turns`.

**What `--output-format json` gives section 3.2 and section 4.** Verified against a live run:

```json
{
  "response": "<final message>",
  "stats": {
    "models": {
      "<model-id>": {
        "api":    {"totalRequests": 1, "totalErrors": 0, "totalLatencyMs": 2875},
        "tokens": {"prompt": 1367, "candidates": 46, "total": 1503,
                   "cached": 0, "thoughts": 90, "tool": 0}
      }
    },
    "tools": {"totalCalls": 0, "totalSuccess": 0, "totalFail": 0,
              "totalDurationMs": 0,
              "totalDecisions": {"accept": 0, "reject": 0, "modify": 0, "auto_accept": 0},
              "byName": {}},
    "files": {"totalLinesAdded": 0, "totalLinesRemoved": 0}
  }
}
```

`response` is the string the transcript checks match against. `stats.models.<id>.tokens.cached` is Gemini's cache observable, the counterpart to Claude Code's `cache_read_input_tokens`. `stats.tools` is a client-side count of tool calls that can be cross-checked against the interposer's own `tools/call` frame count, which is a useful integrity check on the log rather than a second source for the metric.

**Gemini CLI reports no cost figure.** There is no dollar field anywhere in the output, only token counts and latencies. Cross-client cost comparison therefore cannot be built from the clients' own cost reporting, and must not be: the comparable basis is the wire token count from the interposer under one tokenizer, per `methodology-v0.md` 1.6, which is what Tier 2 measures anyway.

**Do not leave `--model` at `auto`.** In the verification run, `auto` resolved to two models for a single prompt, a lite routing pass plus the answering model, each with its own token block in `stats.models`. Client-reported usage is then not attributable to one model, which breaks the section 3.2 record before it reaches section 4. This is distinct from OPEN 8 (which asks whether client versions are pinned) and is settled here: the model is pinned per trial, by flag, and recorded.

**Telemetry is configuration, not a flag.** `telemetry.enabled`, `telemetry.target`, `telemetry.outfile`, `telemetry.otlpEndpoint` and friends are settings keys with `GEMINI_TELEMETRY_*` environment overrides; the OTel stream includes `gemini_cli.token.usage`. No `--telemetry` or `--telemetry-outfile` CLI flag was found in `gemini --help` on 0.18.4 or in the current `cli-reference.md`; treat any claim that those flags exist as unverified. The suite does not need telemetry: `--output-format json` already carries the per-model token counts.

Client version comes from `gemini --version` (`-v` works too).

### 7.3 What remains unbuilt

This section confirms the invocations. It does not build the runner. Still to be written: per-trial directory setup, `{trial_id}` generation and prompt rendering, the per-trial `mcp.json` / `.gemini/settings.json` emission with the interposer wrapping the target server, invocation with wall-clock timing around it, success-criterion evaluation per section 2, and aggregation into `summary.json` per section 6.

---

## 8. OPEN items

None of these blocks writing the spec. The unresolved ones block actually running a suite.

1. **RESOLVED 2026-08-18. Claude Code non-interactive automation.** Confirmed against client 2.1.234 and current docs; see section 7.1.
2. **RESOLVED 2026-08-18. Gemini CLI equivalent.** Confirmed against client 0.18.4 and docs at HEAD; see section 7.2.
3. **Cost attribution under client-side caching.** Three trials against the same server, close together in time, may hit the client's own prompt cache on trials 2 and 3, suppressing schema-attributable tokens relative to a cold trial 1. Whether to force a cold session per trial, or accept and record cache state per trial as an additional field, is not decided.
4. **RESOLVED 2026-08-18. The analyze step exists.** It is `loadline analyze --log <trial.jsonl> [--out <report.json>]`, implemented in `internal/t2analyze`, not a subcommand of the interposer. Keeping it out of the interposer preserves that binary's stdlib-only property, which `interposer/README.md` names as the reason per-call token counting was a v0.1 non-goal in the first place; analysis runs offline on a finished log and has no reason to inherit the constraint. It correlates requests to responses by id, counts `params_full` under `o200k_base` via the same tokenizer Tier 1 uses (`methodology-v0.md` 1.6), and emits the section 4 per-trial metrics stamped with both the log's `interposer_version` and its own analyzer version. One limit is worth stating up front: call-response **result** tokens are only measured when the log was produced with the interposer's `--full-results`, because without it the log carries a byte count and a text length for each result but not the text. The report leaves that figure null and says so rather than estimating it.
5. **NARROWED 2026-08-18. GitHub fixture repo is an operator task.** `lopster568/loadline-tier2-fixture`, its `tier2-baseline` tag, and the reset/cleanup automation in section 2.3 still do not exist. Unlike the local fixtures this one cannot be created from the repo: it needs the credentialed account, so it is Roshan's to create on the PC per the build-on-PC rule, and it stays open until it is.
6. **RESOLVED 2026-08-18. Playwright fixture pages and local server are built.** They live in `tier2/fixtures/playwright/`, with the pages under `site/` rather than directly under the directory as section 2.2's path line says, and `serve.sh` starting `python3 -m http.server` bound to `127.0.0.1` on `${LOADLINE_TIER2_HTTP_PORT:-8930}`. python3 is the tooling choice: present on both machines, no dependency added to the repo, static and read-only with no configuration. The filesystem seed tree of section 2.1 is built alongside it by `tier2/fixtures/filesystem/setup.sh`, which has a `--verify` mode that rebuilds into a temp directory and fails on drift from the committed tree. Both fixture directories carry a README mapping each task id to its prerequisites and a mechanical success-check command.
7. **Decline-rate parsing rule.** Section 4 defines a decline as zero `tools/call` frames plus a completed response, or explicit refusal text, but no documented procedure yet distinguishes a genuine decline from a client that solved the task without needing a tool call (unlikely for these 15 tasks, but not impossible for the read-heavy ones if a client hallucinates an answer instead of calling a tool, which is itself a failure this metric needs to be able to catch, not silently miss as if it were a call-free success).
8. **Client version pinning.** Whether Claude Code and Gemini CLI are pinned to a specific released version for the duration of a suite run, and what happens to an in-flight run if either client auto-updates mid-run, is not decided.
