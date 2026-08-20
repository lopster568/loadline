# Tier 2 Task-Suite Specification

| Field | Value |
| --- | --- |
| Task-suite version | 1.0.4 (PATCH: the runner now records `server_image_digest` for a container-launched server; see the changelog below) |
| Date | 2026-08-20 |
| Status | Draft, for Phase 2 sign-off (PRD.md 10, Open Decision 5). All three suites shaken down on both clients on 2026-08-18, one trial per cell, and the Gemini side re-shaken down on 2026-08-19 against Gemini CLI 0.55.1 after the client upgrade. **The first full run is done**: 2026-08-18, suite 1.0.1, 90 trials, 87 successes, Claude Code 45 of 45 and Gemini CLI 42 of 45, no version drift and no `harness_suspect` trial. No cell is blocked. |
| Scope | Tier 2 dynamic runs only. Tier 1 static sweep is specified in `methodology-v0.md`. |
| Governs | Task selection, fixtures, protocol, and metrics for the 3-server dynamic tier (PRD.md 2.4) |

This document is the separate Tier 2 specification that `methodology-v0.md`'s scope line points to. It defines, concretely enough for a build agent to implement without a new design decision, the servers, the 15 tasks, the run protocol, the extracted metrics, and the versioning discipline for Tier 2.

### Changelog

**1.0.4, 2026-08-20. PATCH: the runner now records `server_image_digest` for a container-launched server.** No prompt, fixture or success criterion moves, so every 1.0.3 result stays comparable across the bump.

The 2026-08-18 github rows record the server pin as `ghcr.io/github/github-mcp-server (container, no digest recorded by the runner)` (`data/tier2-published.json`), a mutable tag rather than a build. `run-suite.sh` now resolves the digest the run actually used once per run, before the first trial (`docker pull` then `docker image inspect --format '{{index .RepoDigests 0}}'`), and records it as `server_image_digest` in the run header and every trial row, alongside the existing `server_pkg` field. `summarize.sh` carries it into `runs[]` and `cells[]` in `summary.json`. A server that is not launched via docker (filesystem, playwright, or github overridden to the non-container Go binary by `LOADLINE_TIER2_GITHUB_CMD`) records `null`. A docker fault (missing binary, failed pull, no repo digest reported) is recorded as a `"digest unavailable: ..."` string rather than stopping the run; failure is data, the same rule the rest of this harness applies.

The run footer re-resolves the digest after the last trial and compares it against the value recorded at the start, the same mechanism section 5 already uses for client versions: if the two differ, `version_drift` is stamped `true` and the run is not comparable, because a mutable tag that moved mid-run is exactly the condition this field exists to catch. `server_image_digest_at_start` and `server_image_digest_at_end` are recorded on the run footer, and both ride along in `summary.json`'s `drifted_runs` entries next to the existing client-version snapshot.

No published Tier 2 row moves and no summary changes: the field did not exist before this bump, so nothing under `data/tier2/` or `data/tier2-published.json` is touched. Sections 5 and 7.4 are updated below to describe the field.

*Decided by the session under the operator's standing instruction; veto window open.*

**1.0.3, 2026-08-20. PATCH: section 4.1 now states that a passed success check outranks `client_error`.** No prompt, fixture or success criterion moves, so every 1.0.2 result stays comparable across the bump.

The Tier 2 first full run (suite 1.0.1, 2026-08-18) recorded GH-03/gemini/t3 as `client_error`: a non-null Gemini `error` object, condition 2 of the 1.0.1 detection rule. The trial's own state check had already passed, `exactly one open issue titled tier2-probe-78a3e055 (visible on poll 1)`, and the interposer log shows one `tools/call` frame, `issue_write`. The client reported its own failure and the server still received and answered the call. The runner checked `client_error` before the success check and bucketed the trial as an infrastructure fault on a trial that plainly succeeded.

Ruling: a passed success check outranks `client_error`. A trial that answers is not "errored out before answering" regardless of what the client's own status object says about itself. Section 4.1 now states the precedence: the success check runs first, and `client_error` applies only when it did not pass. The tool-call-frame split this document has always used is otherwise unchanged: a passed check with at least one `tools/call` frame is `tool_use_success`, with zero it is `answered_without_tools`. The non-null `error` object is not discarded, it is recorded on the trial as `client_reported_error`, so a trial can carry both a `client_reported_error` flag and a `tool_use_success` classification, and neither hides the other.

The reclassification was mechanical: `tier2/run-suite.sh` reorders the existing precedence and `tier2/summarize.sh` recomputes `classification` from each trial row's own recorded fields (success, tool_calls, `client_json_valid`, the Gemini `error` object, the matched refusal markers) rather than trusting the value stored on the row, which is what let both already-recorded runs (2026-08-18 and 2026-08-19) reclassify without a single trial being re-run. Exactly one trial moved: GH-03/gemini/t3, `client_error` to `tool_use_success`. The suite 1.0.1 Gemini `tool_use_success` count for the full run moves from 41 of 45 to 42 of 45, and `client_error` from 1 of 45 to 0 of 45; every other cell, in both days' data, is unchanged. This does not touch the `successes` figure in the Status line above: that field counts the trial's own state check, which had already recorded GH-03/gemini/t3 as a success before this bump.

This is the same class of defect as the 1.0.1 entry below, in the same direction: a narrower rule than section 4.1 intended, this time in the ordering rather than the detection.

*Decided by the session under the operator's standing instruction; veto window open.*

**1.0.2, 2026-08-20. PATCH: the section 4 metrics table no longer claims a schema-attributable figure the harness does not capture.** No prompt, fixture or success criterion moves, so every 1.0.0 and 1.0.1 result stays comparable across the bump. The standing rows carry 1.0.1 and stay where they are; the resume and cell keys of sections 6 and 7.5 are untouched by a metrics-table correction.

Section 4's first row specified "tokens per completed task, schema-attributable", labeled it MEASURED, and gave its source as the client's usage at session start "or the corresponding Tier 1 naive/progressive-mode figure for that server and client mode if the client does not expose a usable session-start figure". Both halves were wrong in the same direction.

Neither client exposes a session-start figure that isolates a tool-definition footprint. Claude Code's `--output-format json` carries `usage.input_tokens`, `usage.cache_creation_input_tokens` and `usage.cache_read_input_tokens` for the whole session; Gemini CLI carries `stats.models.<id>.tokens.prompt`, `.input` and `.cached`. Every one of those contains the system prompt and the conversation alongside the tool definitions, and no field separates them. So the fallback clause is the clause that always fires, and what it falls back to is a Tier 1 figure that is MODELED for two of the three client modes. A MODELED Tier 1 figure copied into a Tier 2 table labeled MEASURED is how a modeled number gets relabeled by transitivity, which is the one thing section 4.3 exists to prevent, and the row said it in the table above 4.3's own warning.

The harness never recorded this metric, so no published figure moves and no summary changes. The row now reads `NOT CAPTURED` and names what would be needed. This is the same class of defect as the GH-05 entry in 1.0.0: the spec claiming something the runner never did.

*Decided by the session under the operator's standing instruction; veto window open.*

**1.0.1, 2026-08-19. PATCH: section 4.1 now states how a `client_error` is detected.** No prompt, fixture or success criterion moves, so every 1.0.0 result stays comparable across the bump.

Section 4.1 has always said that a trial whose client errored out before answering is `client_error` and is excluded from every capability reading. It never said how that is detected, and the runner's detector was narrower than the sentence: it asked only whether the client's output parsed as JSON. Gemini CLI reports an API-side failure as a *valid* JSON document with a populated `error` object, so that route into the bucket was invisible.

It fired on the first re-shakedown against the upgraded client. PW-02 returned four successful tool calls on the wire, a zero tool-call gap, an empty response string, and `error: {type: INVALID_STREAM, message: "The model returned an empty response with no text or thoughts. This may be a transient API issue; please try again."}`. The transcript check failed against the empty string and the trial was bucketed `tool_use_failed`, which publishes as a capability claim about `@playwright/mcp` on the strength of a transient API fault. Section 4.1 now states the two-part detection rule and the runner implements it; the re-run of that cell passed on its merits.

This is the same class of defect as the GH-05 entry in the 1.0.0 bump below, in the opposite direction: there the spec claimed a check the runner never ran, here the runner ran a narrower check than the spec described.

**The 2026-08-19 Gemini re-shakedown rows carry suite version 1.0.0**, because they were produced before this bump. PATCH leaves them comparable with 1.0.1 rows by definition. Note that the runner's resume key and the aggregator's cell key both include the suite version (sections 6 and 7.5), so a later 1.0.1 run lands its rows in separate cells rather than merging with them. That is the cell key being conservative, not a comparability statement.

*Decided by the session under the operator's standing instruction; veto window open.*

**Section 7.2 was re-verified end to end on 2026-08-19 against Gemini CLI 0.55.1** and four invocation details changed. Section 7 is evidence with a decay rate by its own opening line, so a re-verification is not itself a version bump; the changes are recorded in 7.2 and summarised in OPEN 14.

**1.0.0, 2026-08-18. MAJOR: FS-02 and FS-04 success criteria changed.** This is the OPEN 9 resolution.

The two criteria fired on materially correct answers during the 2026-08-18 shakedown, in a way that varied run to run, so an FS-02 or FS-04 failure count could not be read as a capability result without opening the transcript. Both are section 2 text, so changing either is a MAJOR bump under section 5.

- **FS-02** no longer asserts that the final message omits `docs/roadmap.md`. The negative clause tested phrasing, not capability: one shakedown trial listed the two matching files correctly and then named the non-matching files in an explanatory sentence and was recorded as a failure, while a later trial of the same task and client phrased the same correct finding without that sentence and passed. The criterion is now the positive clause alone, which is the whole of what the task measures.
- **FS-04** compares after normalization instead of byte for byte. The old check failed a Gemini trial that wrote `./logs/access.log` with the correct count, which is a path-format artifact. The normalization rule is stated in section 2.1 and is narrow by construction: it cannot turn a wrong answer into a right one, and the genuine miscount the same client produced in another trial (`100`) still fails.

Two smaller criterion changes ride along in the same bump, both aligning the spec with what the runner has always done and both the same class of defect as FS-02:

- **GH-05** no longer asserts that the final message contains no other filename under `docs/`. The runner never implemented that clause, so the spec was claiming a check that was not being run. The clause is also vacuous against the baseline, where `docs/` holds exactly the two files, and phrasing-sensitive in exactly the way FS-02 was.
- **GH-03 and GH-04** state checks are polled rather than read once. This is not a change to what counts as success; it is stated here because the polling window is now part of the criterion's text.

**Justification for taking the comparability cost.** Zero published Tier 2 runs exist. Nothing is invalidated by this bump because there is nothing yet to invalidate, which makes today the cheapest possible moment to fix a criterion and the most expensive moment to defer one. Deferring would mean the first published run carries known-bad criteria and the fix costs a real comparability break later.

*Decided by the session under the operator's standing instruction; veto window open.*

**Two hard rules from PRD.md 2.4 govern everything below and are restated here because they bound every design choice in this document:**

1. If Tier 2 stops running, the whole leaderboard pauses. A cost-only ranking with no capability axis is worse than no ranking.
2. Results are never compared across harness versions. Harness variance can exceed model variance by a wide margin.

A third constraint, from PRD.md 5.2 and `methodology-v0.md` section 10.6, bounds how results get reported rather than how they get produced: Tier 2 covers 3 servers, enough to keep Tier 1 honest on a capability axis, not enough to generalize. Findings are absolute counts against named servers, never rates. This project has already taken public criticism on small-n framing in a sibling benchmark (jaeger-mcp-bench); section 3.3 below states the rule up front so a future release cannot repeat that mistake by omission.

---

## 1. Server selection

Tier 2 selection is a separate, smaller, capability-driven decision from Tier 1's selection rule (`server-selection.md` A.1). All three proposed servers are drawn from the ratified Tier 1 pool for continuity: the same server versions, provenance, and auth notes already researched in `server-selection.md` apply. A Tier 2 finding does not by itself move any MODELED label on that server's Tier 1 row: the label comes off only when a run measures the mode's own context footprint directly (`methodology-v0.md` 3.2 and 3.3, as of 0.3.1), and this suite measures wire traffic, not context (section 4.3).

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
| **FS-02** | "Search the fixture directory tree for every file that contains the string `TODO` and list their paths, relative to the tree root." | Seed tree, unmodified. | Transcript check: final message contains both `docs/intro.md` and `docs/changelog.md`. Nothing is asserted about what else the message contains; see the 1.0.0 changelog entry. | Search (`search_files` / grep-equivalent tool) over a multi-file tree. |
| **FS-03** | "Create a new file at `output/{trial_id}.txt` containing exactly the text `tier2 probe {trial_id}` and nothing else." | Seed tree, unmodified; `output/` does not exist yet. | State check: after the trial, `output/{trial_id}.txt` exists in the scratch copy with byte-exact content `tier2 probe {trial_id}` (no trailing content beyond a single trailing newline, if any, which is tolerated). | Write (`write_file`, and implicitly directory creation). |
| **FS-04** | "Get the full directory tree of the fixture root, find the file with the most lines, and write a file at `summary.txt` containing exactly two lines: the relative path of that file, then the line count." | Seed tree, unmodified. `logs/access.log` (137 lines) is the fixed correct answer; no other file in the tree exceeds it. | State check: `summary.txt` exists in the scratch copy and, after the normalization below, holds exactly two lines, `logs/access.log` and `137`. | Multi-tool chaining: `directory_tree` or `list_directory` recursively, `read_file` or `read_multiple_files` to compare line counts, `write_file` to record the result. |

**FS-04 normalization rule (1.0.0).** The comparison is made after the rewrite below, not on the raw bytes. Trailing blank lines are dropped first, and exactly two lines must remain; a file with one line or three fails before normalization is reached.

| Line | Rewrite applied before comparison |
| --- | --- |
| Path | Leading and trailing whitespace trimmed. One layer of surrounding backticks, double quotes or single quotes stripped. One trailing comma or period stripped. One leading `./` stripped. A path whose final segments are `logs/access.log` reduced to `logs/access.log`, which is the absolute-path form. |
| Count | Leading and trailing whitespace trimmed. One layer of surrounding backticks, double quotes or single quotes stripped. Comma thousands separators removed. The result must be bare digits. |

The rule is narrow on purpose, and the property that makes it safe is that neither rewrite has any knowledge of the expected answer: it can only turn an equivalent spelling of the right answer into the canonical spelling, never a wrong answer into a right one. `logs/other.log` normalizes to `logs/other.log` and fails. `100` normalizes to `100` and fails. `137 lines` is not bare digits and fails. The implementation is `fs04_normalize_path` and `fs04_normalize_count` in `tier2/run-suite.sh`.
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

Fixture: a dedicated throwaway repository, `lopster568/loadline-tier2-fixture`, owned by the same account the harness's credential authenticates as. It is private, not public as earlier drafts of this line said; see OPEN 5 for why and for the launch-day decision that remains open. Baseline state, tagged `tier2-baseline` and re-applied whenever the repo needs resetting:

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

Write and multi-tool-chaining tasks (GH-03, GH-04) use `{trial_id}` in every created object's name so repeated trials never collide.

**`reset.sh` runs before every GitHub trial, not once per suite.** The original once-per-suite rule was wrong on both halves. Keying created objects on `{trial_id}` stops trials from colliding with each other, but it does nothing to stop a read-only task from reading a repo that an earlier mutating trial left dirty, and GH-02 and GH-05 are both stated against a baseline: "the open issue labeled `tier2-fixture`" is unambiguous only while exactly one issue is open. Second, the cleanup path in `tier2/fixtures/github/reset.sh` was the one part of the GitHub fixture that had never executed (OPEN 5), because the `--check` verification of 2026-08-18 found nothing to clean. Running it every trial exercises it every trial. It costs roughly a second of API round trips per trial and it deletes every `tier2-probe-*` issue, closes pull requests from `tier2/*` branches and deletes those branches, and reopens and relabels the fixture issue if a trial closed it.

**The GH-03 and GH-04 state checks poll.** GitHub's issue and pull request list endpoints are served from a cache that lags creation. Measured on 2026-08-18: an issue created through the REST API was invisible to `repos/:owner/:repo/issues?state=open` for about six seconds, and a shakedown trial failed on exactly that window with the issue sitting correctly in the repo and its URL in the client's own transcript. Each check re-reads the state up to `LOADLINE_TIER2_GH_CHECK_ATTEMPTS` times (default 15) at `LOADLINE_TIER2_GH_CHECK_INTERVAL` second intervals (default 2), and records which poll the state became visible on. This is not the retry that section 6 forbids: the trial is over and is not re-run, and what is being waited on is the remote API making visible a state the trial already produced.

| Task | Prompt (client-facing) | Initial state | Success criterion | Exercises |
| --- | --- | --- | --- | --- |
| **GH-01** | "In the `lopster568/loadline-tier2-fixture` repo, read `NOTES.md` on the default branch and tell me the value on the line starting with `TARGET:`." | Baseline. | Transcript check: final message contains the exact substring `42`. | Read-heavy, single file read (`get_file_contents`). |
| **GH-02** | "In `lopster568/loadline-tier2-fixture`, find the open issue labeled `tier2-fixture` and report its exact title." | Baseline (exactly one issue exists, so the search is unambiguous). | Transcript check: final message contains the exact substring `Fixture issue: do not close`. | Search (issue search/list filtered by label). |
| **GH-03** | "In `lopster568/loadline-tier2-fixture`, create a new issue titled exactly `tier2-probe-{trial_id}` with body `automated probe, safe to close`." | Baseline. | State check: after the trial, querying the repo's issues for title `tier2-probe-{trial_id}` returns exactly one open issue with that title. | Write (issue creation). |
| **GH-04** | "In `lopster568/loadline-tier2-fixture`, create a new branch named `tier2/{trial_id}` from `main`, add a file at `probe/{trial_id}.txt` containing exactly `ok` on that branch, and open a pull request from it into `main` titled `tier2 probe {trial_id}`." | Baseline. | State check: after the trial, branch `tier2/{trial_id}` exists, contains `probe/{trial_id}.txt` with content `ok`, and an open pull request from that branch into `main` exists titled `tier2 probe {trial_id}`. | Multi-tool chaining: `create_branch`, `create_or_update_file`, `create_pull_request`, three distinct write calls in sequence. |
| **GH-05** | "In `lopster568/loadline-tier2-fixture`, list the files under `docs/` and report their names." | Baseline (`docs/alpha.md`, `docs/beta.md`, nothing else). | Transcript check: final message contains both `alpha.md` and `beta.md`. Nothing is asserted about what else the message contains; see the 1.0.0 changelog entry. | Read-heavy listing, a different call shape (directory listing) from GH-01's single-file read. |

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
| Cache figures | Client's own JSON output | Claude Code: `usage.cache_creation_input_tokens` and `usage.cache_read_input_tokens`. Gemini CLI: `stats.models.<id>.tokens.cached`, summed across models. Per the OPEN 3 decision below. |
| Trial ordinal | Runner | Two numbers: the trial's index within its (task, client) cell, and its execution order within the suite invocation. Cache effects track execution order, so a cache figure without it is uninterpretable. |
| Classification | Runner, per section 4.1 | The tool-use classification of the trial, which is not the same fact as pass/fail. |
| Fixture extra paths | Runner, filesystem tasks | Anything in the scratch copy that is neither a seed-tree member nor the task's own expected output. Empty is the evidence that the trial measured the fixture and nothing else. |
| Tool-call gap | Runner, from the client's own per-tool counts against the interposer's frames | Calls the client says it attempted to a tool the server advertised, minus calls the interposer saw. Positive means the client meant to reach the server and the call never left the client. Section 4.1. |
| Models billed | Client's own JSON | Every model the trial consumed tokens on, not just the pinned one. Claude Code runs a small auxiliary model alongside the pinned one in the same trial; folding it away would understate what the task cost. |

**Cache state is recorded, not controlled.** Trials are not forced cold. Two reasons, and they are the whole of the OPEN 3 decision: a cold session is not how anyone actually uses either client, so forcing it measures a configuration nobody runs; and neither client exposes a reliable, documented way to guarantee a cold session, so an attempt to force one would produce a claim the harness cannot back. Instead every trial publishes its own cache figures alongside its ordinal, which makes a cache effect visible in the data rather than hidden by it. A reader who wants the cold number reads trial 1 of a cell; a reader who wants to know how much the cache moved the figure compares it against trials 2 and 3.

### 3.3 The small-n rule

**n = 3 per cell supports per-cell reporting only.** A cell is one (server, task, client) triple. Three trials tell you what happened three times; they do not support a percentage, a confidence interval, or a claim that one client is "more reliable" than another in any statistical sense. Per-cell results are reported as what they are: 3 trials, this many succeeded, these are the per-trial numbers.

**Cross-server or cross-client comparisons require pooling trials across the relevant cells**, and even pooled, they are reported per the honest-metrics rule already in force for this project (PRD.md Success Metrics section): absolute counts, never small-n percentage framing. "2 of 3 trials failed" is reportable. "33% failure rate" is not, at this n, regardless of how many cells get pooled to produce it, unless the pooled n is large enough that a rate framing stops being misleading, a threshold this document does not set and a future release must justify explicitly if it crosses it.

This rule is stated here, not left implicit, because this project has already taken public criticism on exactly this point in a sibling benchmark (jaeger-mcp-bench, referenced in PRD.md 2.4 as the source of the interposer's own instrument debt). Restating the rule in the spec itself, rather than trusting it to survive into every downstream writeup, is the pre-emption.

---

## 4. Metrics extracted

Every metric Tier 2 records is labeled **MEASURED**. Nothing in Tier 2 is MODELED; that distinction exists in Tier 1 (`methodology-v0.md` section 3) because Tier 1 projects client behavior from a published figure. Tier 2 runs the actual client against the actual server and records what happened, which is the entire reason Tier 2 exists: to validate Tier 1's modeled figures against real call traffic, not to model anything of its own (`methodology-v0.md` 3.3).

One row of the table is `NOT CAPTURED` rather than MEASURED. A metric this document specifies but the harness cannot record is listed with that label rather than dropped, so a reader can see the hole and so no downstream consumer can read the table as an inventory of available figures.

| Metric | Definition | Source | Label |
| --- | --- | --- | --- |
| Tokens per completed task, schema-attributable | Tokens consumed by tool-definition schemas loaded into the client's context for this session, not by call traffic | Would have to be a per-session figure from the client's own usage output that isolates the tool-definition footprint. Neither client exposes one, so the harness records nothing for this row; see 4.3 and the 1.0.2 changelog entry | NOT CAPTURED |
| Tokens per completed task, call-response | Tokens consumed by the actual `tools/call` request and response traffic during the task | Computed from the interposer JSONL: `params_full` of each `tools/call` request plus `result_summary` (or `result_full` under `--full-results`) of each response, counted with the `o200k_base` tokenizer per `methodology-v0.md` 1.6, so the number is comparable to Tier 1's token basis | MEASURED |
| Tool calls per task | Count of `tools/call` request frames in the trial's JSONL | Interposer JSONL, `method == "tools/call"` frames | MEASURED |
| Argument sizes | Byte size of `params_full` per `tools/call` request; reported per call and as a per-task total | Interposer JSONL, `size_bytes` and `params_full` on request frames | MEASURED |
| Retry/error counts | Count of JSON-RPC error responses (`error` field present), plus count of `tools/call` requests with identical method and equivalent `params_full` to an immediately preceding request for the same trial (a same-trial repeat is treated as a retry) | Interposer JSONL | MEASURED |
| Decline count | Trials classified `declined` by the rule in section 4.1: zero `tools/call` frames, a failed success check, and a refusal marker in the final message. Its two siblings, `answered_without_tools` and `failed_no_tool_use`, are counted and published alongside it and are never folded into it | Interposer JSONL (call count) cross-checked against the client's final transcript (refusal text) | MEASURED, reported as an absolute count per section 3.3, never a rate at n = 3 |

### 4.1 Trial classification, and how a decline is told from a hallucination

This is the OPEN 7 rule, stated mechanically because the metric it feeds is otherwise a judgment call. It applies to a trial that produced a client response at all; a trial whose client errored out before answering is `client_error`, a setup or infrastructure fault that is recorded and excluded from every capability reading rather than being counted as a decline.

**How `client_error` is detected** (added in 1.0.1). Two conditions, either of which is sufficient:

1. The client's `--output-format json` output does not parse as JSON. The client died before it could serialise a result.
2. The output parses and reports its own failure: for Gemini CLI, a non-null top-level `error` object. Claude Code has no verified counterpart today, so this half of the rule is Gemini-only, the same asymmetry section 4.2 records for the tool-call gap and OPEN 12 owns.

The second condition is the one that was missing until 2026-08-19. A client can return a well-formed document, a full set of successful tool calls, correct token accounting, and an empty answer because the API stream failed; on JSON validity alone that reads as a trial the model got wrong. The distinction matters most exactly where it is hardest to see, which is a trial that did reach the server and did everything right up to the last step.

Condition 2 changes the classification only. The trial's token, cache and tool-call figures are still recorded in full, because they describe traffic that genuinely happened and are the evidence for calling it an infrastructure fault in the first place.

**A passed success check outranks `client_error`** (added in 1.0.3). The success check runs first. A trial whose check passed demonstrably did answer, so it is not "errored out before answering" no matter what either detection condition above found. `client_error` applies only when the check did not pass; a trial with a passed check and a client-reported error is classified exactly as it would be with no reported error: `tool_use_success` with at least one `tools/call` frame, `answered_without_tools` with zero (below). The finding behind condition 1 or 2 is not discarded when it is outranked: it is recorded on the trial as `client_reported_error`, so a trial can carry `client_reported_error: true` alongside a passing classification, and a reader auditing the `client_error` count is not missing the cases where the client reported a failure anyway.

**A trial with zero `tools/call` frames in the interposer log is classified by its success check.**

- If the check **passes**, the trial is **`answered_without_tools`**. This is a flagged anomaly, not a success: the client produced the right answer without consulting the server, so the trial measured nothing about the server. It counts as a failure for every tool-use metric and is reported separately from a decline, because the two say opposite things about the client.
- If the check **fails** and the final message contains a refusal marker from the list below, matched case-insensitively, the trial is **`declined`**.
- Otherwise the trial is **`failed_no_tool_use`**: the hallucination bucket, a client that answered wrongly without calling anything.

All three are published as absolute counts per section 3.3. A trial with at least one `tools/call` frame is `tool_use_success` or `tool_use_failed` on its check.

**The refusal marker list**, matched as case-insensitive substrings of the client's final message:

```
can't            cannot           can not          unable to
not able to      don't have access                 do not have access
no access to     not permitted    not allowed
don't have permission             do not have permission
i'm sorry        i am sorry       i apologize
isn't available  is not available unavailable
no tools         no such tool     tool is not
```

**This list is a heuristic and is labeled as one.** It is consulted only for a trial that both made zero tool calls and failed its check, which is the only place where the difference between a refusal and a wrong answer is not already decided by harder evidence. Inside that narrow window it will occasionally mislabel: a wrong answer phrased apologetically reads as a decline, and a refusal worded outside the list reads as a hallucination. The markers matched are recorded per trial alongside the classification, so any published decline count can be audited back to the exact string that produced it. The list is part of the task-suite version: changing it is a PATCH if no recorded trial changes bucket and a MAJOR if one does.

The `answered_without_tools` bucket is not hypothetical. In the 2026-08-18 shakedown, Claude Code answered FS-01 correctly with zero `tools/call` frames, having used its own built-in `Read` tool: correct answer, no measurement. The runner now denies the built-in tool surface for exactly that reason (section 7.1), and the bucket stays as the detector for the next version of the same problem.

### 4.2 The tool-call gap, and why a classification is not enough

A trial's classification is decided by what reached the wire. That is the right basis for every case above, and it is the wrong basis for one case the shakedown found: a client that tried to call the server and could not.

**The gap is defined mechanically.** For every tool name the server advertised in its `tools/list` response, take the number of calls the client's own usage output attributes to that tool, subtract the number of `tools/call` frames the interposer logged for it, and sum the positive differences. Zero is the normal state. Positive means the client formed calls to tools the server offered and those calls never left the client.

The restriction to *advertised* tool names is what makes the measure mean anything. A naive comparison of the client's total call count against the wire frame count is not a fault signal at all: both clients run with their built-in tool surface switched off (section 7.1, section 7.2), so a model regularly tries a built-in that no longer exists and the client counts the attempt. Both filesystem trials of the 2026-08-18 Gemini rerun showed a positive naive gap that was entirely `run_shell_command`, which is the isolation working exactly as designed and is not a fault. Restricted to advertised names, the same two trials read zero.

**What it caught.** Gemini CLI 0.18.4 validates a tool call's arguments against the schema the server advertised, and its bundled validator has no JSON Schema draft 2020-12 meta-schema registered. `@modelcontextprotocol/server-filesystem` and `github-mcp-server` declare draft-07 or no `$schema` at all; `@playwright/mcp` declares 2020-12 on all 24 tools. So the same client that ran the filesystem and github suites clean failed most playwright calls client-side with `no schema with key or ref "https://json-schema.org/draft/2020-12/schema"`, before any frame was written. On the wire that reads as a small number of calls and a wrong answer, and the classifier buckets it `tool_use_failed`, or on one trial `declined`, because the model apologised. Neither is true. The gap field is what told a reader that. The defect is fixed in the client as of the 2026-08-19 upgrade and OPEN 11 is closed; this paragraph stays because it is the worked example of what the measure is for.

**The measure is only as good as the name matching, and the client upgrade broke it silently once.** The wire name in the log is the server's own bare tool name, because that is what travels in a `tools/call` frame. The client keys its usage output by the name the *model* called, and that spelling changed between the two Gemini versions: 0.18.4 used the bare name, 0.55.1 uses the fully qualified `mcp_<server>_<tool>`. Matched on the bare name alone, every advertised tool scored zero client calls against a positive wire count, every difference came out negative, and the sum-of-positive-differences definition above discarded all of them. The gap would have read a clean zero on every trial while detecting nothing. It is worth stating plainly because of the shape of the failure rather than its size: a detector that reports the healthy value when it is broken cannot be distinguished from a detector that is working, and this one is the only thing standing between a client-side interop fault and a published capability number. The runner now sums both spellings per advertised tool. Any future change to how a client names a server's tool has to be checked against this function, not assumed.

**The gap is recorded, not acted on.** It does not currently change any classification, because whether a client-side interop failure deserves a bucket of its own is a change to the section 4.1 taxonomy and therefore a spec decision, not one the runner makes on its own. Filed as OPEN 12. A cell with any `harness_suspect` trial in it is not a capability reading about that server until that is settled.

**It is a Gemini-only measure today.** Claude Code's `--output-format json` object carries no per-tool call count to compare against, so the field is null for every Claude trial. That is a real hole: the same class of fault on the Claude side would currently be invisible. Also filed under OPEN 12.

### 4.3 Why the split between schema-attributable and call-response tokens matters

The interposer measures wire traffic on the MCP stdio pipe: `tools/call` requests and responses, plus the `tools/list` exchange if the trial's client re-issues it. It does not, by itself, see whatever the client injected into the model's context as tool definitions before the first call, because that happens inside the client, not on the MCP wire. Session-start context footprint has to come from the client's own usage reporting (or, failing that, be cross-referenced against Tier 1's measured figure for that exact server, client mode, and version). Conflating the two would misattribute Tier 1's schema cost as if it were something Tier 2 measured directly, which it does not.

---

## 5. Versioning and comparability

Per `methodology-v0.md` section 9, results are comparable only within an identical (harness, methodology/suite) version pair. Tier 2 carries its own version axis for the same reason:

| Stamp | Recorded | Bump trigger |
| --- | --- | --- |
| `interposer_version` | Per `interposer/README.md`, from the JSONL header line of every trial | Interposer's own semver; any change to framing, logged fields, or `result_summary` computation is a version bump per its README |
| Task-suite version (this document) | This document's header field, recorded per trial | See below |
| Client version | Captured at trial start via the client's own `--version` (or equivalent) output | Not versioned by this project; recorded as observed |
| Container image digest | For a server launched via docker, resolved once per run before the first trial and re-resolved after the last; recorded per trial as `server_image_digest` alongside the existing `server_pkg` field | Not versioned by this project; recorded as observed, per the 1.0.4 changelog entry |

**Task-suite version scheme**, mirroring the MAJOR/MINOR/PATCH discipline `methodology-v0.md` 9 applies to the methodology itself:

- **MAJOR**: a task's prompt, fixture, or success criterion changes in any way that changes what counts as success, a task is added or removed, or a server is substituted (section 1.1). Prior Tier 2 results for the affected task or server are not comparable across the bump.
- **MINOR**: a new task is added for an existing server without touching any existing task's prompt, fixture, or criterion. Prior results stay valid.
- **PATCH**: a wording fix, a typo correction, or a documentation clarification with no effect on the rendered prompt or the success check. Prior results stay valid.

**A run holds its client still while it runs.** Added 2026-08-18, after the github shakedown recorded `version_drift: true` on its own footer because Claude Code updated itself from 2.1.234 to 2.1.235 between that run's first and last trial. The runner now launches every client invocation with `DISABLE_AUTOUPDATER=1` alongside the API-key stripping, for the same reason: it is a thing the client would otherwise do to itself mid-run. This is not the pin the next paragraph correctly refuses to promise. It holds nothing across runs, it claims nothing about which version the operator has, and drift detection stays on regardless, because an operator-driven update between runs is still what the footer has to catch. It only stops a run already in flight from becoming uncomparable with itself.

**Client versions are recorded and drift is detected, not pinned.** This is the OPEN 8 decision. Both clients auto-update on their own schedule and neither offers a supported way to hold a released version in place for the duration of a run, so a pin this project announced would be a pin it could not enforce. What the runner does instead is mechanical: it records `claude --version` and `gemini --version` into the run manifest's header before the first trial, re-reads both after the last one, and if either changed it stamps `version_drift: true` on the run footer. A run carrying that stamp is not comparable, neither internally across its own trials nor against any other run, and it is republished only by being run again. Both clients are recorded on every run regardless of which one the suite invoked, because a mid-run update to the other client is the thing that makes the next suite's numbers quietly incomparable with this one's.

**A container server's pin is now the digest, not the tag.** Added 2026-08-20 (task-suite 1.0.4). A tag like `v1.9.0` or the implicit `latest` a docker image reference resolves to when no tag is given is mutable: the same tag can point at a different build tomorrow, so recording the tag alone does not pin the run to a build the way the npm package version pin does for the filesystem and playwright servers. `run-suite.sh` now resolves the digest the run actually used (`docker pull` then `docker image inspect`) once before the first trial and records it as `server_image_digest`, and re-resolves it after the last trial the same way section 5 already re-reads the client version: if the two differ, `version_drift: true` is stamped on the run footer and the run is not comparable, for the same reason a client update mid-run is not comparable. **The 2026-08-18 rows (`data/tier2-published.json`, `suite_version` 1.0.1) carry a tag only**, recorded before this field existed: `ghcr.io/github/github-mcp-server (container, no digest recorded by the runner)`. That is a real gap in what those rows pin, not a comparability break under this PATCH; nothing about them is republished or corrected, and a reader comparing them against a later digest-bearing run should read the earlier ones as pinned only to "whatever `latest` was on 2026-08-18," not to a specific build.

Every published Tier 2 result carries the first three stamps from the table above; a result missing any of those is withheld, the same rule `methodology-v0.md` 1.7 applies to Tier 1 cells. The fourth, container image digest, applies only to a server launched via docker (github, by default), and is `null` rather than withheld for filesystem and playwright. Results are never compared across interposer versions or across task-suite MAJOR versions; a season-over-season Tier 2 trend line is only drawn within one (interposer MAJOR, suite MAJOR) pair.

---

## 6. Runbook stub

**Prerequisites.** Interposer built (`cd interposer && go build -o ../tier2/bin/loadline-interposer ./cmd/loadline-interposer`; the interposer is its own Go module, so building it from the repo root fails, and `run-suite.sh` builds both binaries itself on every invocation anyway), both clients installed and authenticated (Claude Code, Gemini CLI), `jq`, `go`, and for the github suite `docker` and an authenticated `gh`. The playwright fixture pages must be reachable at `127.0.0.1:${LOADLINE_TIER2_HTTP_PORT}` via `tier2/fixtures/playwright/serve.sh` started before the run, and the Playwright browser must be installed (section 7.3; the runner preflights it).

**The GitHub credential is not exported.** Earlier drafts of this line said to export the PAT into the run environment. That is wrong under the rule of section 7.4: the token must reach the MCP server and nothing else, and an exported variable reaches the client process first. The runner takes it from `gh auth token` (or `LOADLINE_TIER2_GITHUB_TOKEN`) at suite start and hands it to the container through a private `--env-file`. Nothing needs to be exported by hand.

**Running one suite end to end** (per server, per client, all 5 tasks, 3 trials each):

1. Reset fixtures: fresh scratch copy of the filesystem seed tree per trial (section 2.1); confirm the playwright static server is up and serving the pinned fixture pages unmodified, and that the browser is installed; confirm the GitHub fixture repo is at `tier2-baseline` with no leftover `tier2-probe-*` / `tier2/*` objects from a prior run. The runner does all three itself.
2. For each of the 15 tasks, for each client, for each of 3 trials: generate `{trial_id}`, render the prompt, point the client's MCP config at the interposer wrapping the target server (per `interposer/README.md`'s Claude Code / Gemini CLI config examples), invoke the client non-interactively with the rendered prompt, and capture wall time, the interposer JSONL, client-reported usage, and client version per section 3.2.
3. Evaluate the trial's success criterion (section 2) immediately, before the next trial starts, so a write-task's mutated state does not leak into a later trial's baseline assumption.
4. `tier2/fixtures/github/reset.sh` runs before every github trial (section 2.3), and `reset.sh --check` after the last one confirms the repo was left at baseline.
5. `tier2/summarize.sh` aggregates the day into `summary.json` (section 6, below).

**Expected duration.** 90 trials. A filesystem or playwright trial (local, no network round trip beyond loopback) should complete in well under a minute of client think-and-call time; a GitHub trial, authenticated and remote, budget one to two minutes. Budgeting generously for client startup and variance, a full serial run is roughly 2 to 4 hours. Trials against different servers may run concurrently, since each server's log is a separate file and `interposer/README.md` only warns against interleaving frames from concurrent sessions against the *same* log; trials against the same server should stay serial to keep that log's frame order attributable to one session at a time.

**Where outputs land.** As the runner writes them, all under `data/tier2/<date>/`:

| Path | Contents |
| --- | --- |
| `manifest.jsonl` | One run header per suite invocation, one row per trial carrying every section 3.2 field, one run footer with the drift check of section 5. Append-only, and the resume key: a trial whose `(server, client, task, trial)` row is already present is skipped, so an interrupted suite is resumed by re-invoking it with the same arguments. |
| `<client>/<task>-t<n>.jsonl` | That trial's interposer frame log. |
| `<client>/<task>-t<n>.client.json` | The client's own `--output-format json` object, unedited. |
| `<client>/<task>-t<n>.analyze.json` | `loadline analyze` output for the log. |
| `<client>/work/<task>-t<n>/` | The trial's MCP config or Gemini settings file, the rendered prompt, the extracted transcript, client stderr, and PW-05's output directory. |
| `<client>/scratch/<task>-t<n>/` | The per-trial fixture copy for filesystem tasks, kept after the trial so a state check can be re-audited. |
| `summary.json` | The aggregated, publishable result for the day, written by `tier2/summarize.sh`. |

Task ids carry their server (`FS-`, `PW-`, `GH-`), so the server does not need a directory level of its own.

**`summary.json` and the aggregator.** `tier2/summarize.sh [--date YYYY-MM-DD] [--out DIR] [--stdout]` reads `manifest.jsonl` and writes `summary.json` beside it. Its input is the manifest and nothing else, deliberately: the frame logs are secrets and are deleted once a run has been aggregated and spot-checked, so the summary has to be reconstructible without them, and every analyze figure it reports was already copied into the manifest row at trial time.

| Level | Contents |
| --- | --- |
| Run | Every `run_header` merged with its `run_footer`: run id, server, client, model, suite and interposer versions, server package, isolation settings, playwright browser, keys stripped, client versions at start and end, drift, trials executed and skipped, interrupted. |
| Day | Suite, interposer and analyzer versions in play; `mixed_suite_versions` if more than one suite MAJOR appears, since section 5 forbids reading those together; `version_drift` and the list of drifted runs; `models_billed`; `superseded_trial_rows`. |
| Cell | One entry per (suite version, server, task, client). Trials, successes, failures, the section 4.1 classification counts, `harness_suspect_trials`, timeouts, permission-denial trials, fixture extra paths, and the distributions below. |

Every distribution in a cell (tool calls, wall time, `tool_call_arg_tokens`, `tool_call_result_tokens`, argument and result bytes, wire bytes, JSON-RPC errors, tool errors, retries, the tool-call gap, and both clients' cache figures) is emitted as `{n, median, min, max, values}`. The raw `values` array is not a debugging convenience, it is the section 3.3 rule made structural: at n = 3 the per-trial numbers *are* the result and the median is the convenience on top. Each cell also carries a `per_trial` array with both ordinals, the trial id, the classification, the matched refusal markers, the check evidence, and the trial's own cache block, because a cache figure without its execution order is uninterpretable (section 3.2).

**No rates, structurally.** The aggregator emits no percentage, no rate, and no success ratio anywhere, and will not be extended to. Counts and distributions only, per section 3.3.

**`version_drift` is day-level and deliberately conservative.** The day flag is true if any run footer in the manifest recorded drift, including a run whose trials were later superseded by a forced re-run. It is not narrowed to the runs whose rows stand, because a client that changed version once during a day is a fact about the whole day's comparability and hiding it behind a re-run would be the wrong default. Per-run drift is in the `runs` array for a reader who needs the finer answer.

**Deduplication.** Trial rows are deduplicated on (suite version, server, client, task, trial), keeping the last. The manifest is append-only, so a cell re-run with `run-suite.sh --force` after a harness fix leaves both rows in the file: the superseded one stays as the record of what the broken setup produced, and `superseded_trial_rows` puts the count of them on the face of the summary rather than leaving a reader to find it. `--force` exists for re-running a cell after a harness fix. It is not for retrying a trial that failed on its merits, which the rule below forbids.

One consequence to know before reading a superseded row: per-trial artifact paths are keyed on `<task>-t<n>` and nothing else, so a forced re-run overwrites the frame log, client JSON, analyze report, work directory and scratch copy of the row it supersedes. The superseded manifest row keeps its own recorded fields, which is the evidence that matters, but its `artifacts` paths point at the re-run's files. A summary is therefore auditable against raw logs only for the rows that stand.

`data/tier2/` is gitignored, which is the one exception to this repo's rule that `data/` is committed. The reason is the interposer security note below, not tidiness.

**The rule: a failed trial publishes as a failed trial.** Per the failure-as-data posture already in force for Tier 1 (`pc-sweep-runbook.md` section 5, `methodology-v0.md` section 7), a trial that times out, errors, or fails its success criterion is kept in `summary.json` exactly as it happened. It is not retried and silently replaced, not hand-edited, and not dropped from the count. If a genuine setup mistake is found mid-run (wrong port, stale credential, fixture not reset), the fix applies to the next run, not to reclassifying a trial that already happened under the broken setup.

**Interposer log handling.** Per `interposer/README.md`'s security section, every trial's JSONL contains complete tool-call arguments and, for the GitHub trials, potentially credential-adjacent material. Logs are treated as secrets: kept out of git, not attached to any published artifact, and deleted once the run they support has been aggregated into `summary.json` and the aggregation has been verified against a sample of the raw logs.

---

## 7. Runner invocation

Verified live on **2026-08-18** against the installed clients and against current vendor documentation, and section 7.2 re-verified end to end on **2026-08-19** against a Gemini CLI upgraded from 0.18.4 to 0.55.1. Both clients ship fast, so this section is evidence with a decay rate: re-verify the flags below before a suite run, and record the client version per trial (section 3.2) either way. The 2026-08-19 re-verification is the decay rate proving itself, and it is worth reading before trusting anything else here: four of the flags and settings keys section 7.2 had verified the day before had changed behaviour, two of them into a dead run and one of them into a measurement that reports the healthy value while detecting nothing.

Installed versions at verification time: **Claude Code 2.1.234**, updating to **2.1.235** during the day, and **Gemini CLI 0.18.4**, upgraded to **0.55.1** on 2026-08-19. Server versions exercised: `@modelcontextprotocol/server-filesystem@2026.7.10`, `@playwright/mcp@0.0.79`, `ghcr.io/github/github-mcp-server` v1.9.0.

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

| Flag | Behavior that matters here |
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

**Four things the shakedown of 2026-08-18 added to this invocation.** Each was found by running it, and each is in the runner now.

1. **`--disallowedTools` is required; the allowlist alone is not isolation.** Under `--permission-mode dontAsk` the read-only built-ins still run without a prompt, so FS-01 came back correct with zero `tools/call` frames and an empty `permission_denials` array: Claude Code had answered it with its own `Read` tool and the MCP server never saw the task. The runner passes `--disallowedTools "Bash,Read,Write,Edit,MultiEdit,NotebookEdit,Glob,Grep,LS,WebFetch,WebSearch,Task,TodoWrite,SlashCommand"`, after which the same trial calls `mcp__filesystem__read_text_file` and still records no permission denials. This changes the client's context footprint, which is a recorded harness fact, not a hidden one: the disallowed list is stamped in the run header.
2. **The client's working directory is the fixture root for a filesystem trial.** Claude Code advertises the MCP roots capability and sends its working directory as the only root, and `@modelcontextprotocol/server-filesystem` replaces its command-line allowed directories with the client's roots. Launching the client anywhere else makes the server deny every path in the scratch copy. Nothing the runner writes goes into that directory: the MCP config, prompt, transcript and client output live in a sibling work directory, and the runner records any file the client leaves behind in the fixture tree (section 3.2, fixture extra paths).
3. **One trial produces two interposer sessions.** Claude Code spawns the server once for a `server/discover` probe and again for the session that does the work, so the log carries two header lines and `loadline analyze` warns that the log holds two sessions. It is the client's launch behavior, not a log-handling mistake, and the traffic is not double counted: only the second session issues `initialize`, `tools/list` and the calls.
4. **Cost safety: the trial environment must not carry an API key.** Claude Code bills a metered API wallet instead of the operator's plan when `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN` is present in its environment, and PRD.md 6 has this project riding plan quota. The runner strips both from every client invocation and prints a notice naming what it stripped, and it never sources this repo's `.env`. Claude Code itself does not read `.env` files: verified on 2.1.234 by planting an invalid key and an unreachable base URL in a parent directory's `.env` and watching the run succeed anyway. Gemini CLI does read them, which is section 7.2's problem.

**Three more the github shakedown of 2026-08-18 added.** All three were found by running the suite against a real credentialed server, which is the part of the matrix nothing before it had exercised.

5. **`--setting-sources ""` is required, and it is a measurement fix rather than hygiene.** GH-03 came back with zero tool calls and this final message: *"I have a standing instruction (from your global CLAUDE.md) that I never send outbound communications, including creating issues, myself."* The trial measured the operator's memory file rather than the client against the server, and the classifier bucketed it `failed_no_tool_use`, which reads as a hallucination and is not what happened. Any operator instruction touching writes, credentials, or outbound actions can do this to any GH-03, GH-04 or FS write task, and it will look like a capability result. The runner passes `--setting-sources ""`, which loads no user, project or local settings source. Verified on 2.1.234: with no flag the client answers a question whose only source is `~/.claude/CLAUDE.md`; with the flag the same question comes back `NONE`, and the server passed by `--mcp-config` still loads. The value is stamped in the run header.

    **`--bare` is the flag that sounds like the answer and is not.** It does skip `CLAUDE.md`, but its own help text says Anthropic auth then becomes strictly `ANTHROPIC_API_KEY` or an `apiKeyHelper`, with OAuth and the keychain never read. It would move every trial onto the metered wallet item 4 exists to avoid. That, not the project `.mcp.json` point this section made before, is the real reason `--bare` is out.

6. **The pinned model is not the only model billed, and `modelUsage` does not name it first.** Claude Code runs a small auxiliary model in the same trial as the pinned one. On a GH-04 trial at `--model claude-sonnet-5`, `modelUsage` held `claude-haiku-4-5` at 19 output tokens and `claude-sonnet-5` at 812. Taking the first key of that object sorts alphabetically and reports haiku as the model that answered, which is wrong on the face of every published cell. The runner takes the entry with the most output tokens as `resolved_model` and records the whole list as `models_used`, so the auxiliary pass is visible rather than folded away.

7. **One trial produces two interposer sessions, and the read is still attributable.** Restated from item 3 above because the github server made it visible again: the discovery probe issues `server/discover`, `resources/list` and `prompts/list` and no `tools/call`, so a trial that makes no real call still leaves a populated log.

### 7.2 Gemini CLI

Sources: `docs/cli/headless.md`, `docs/cli/cli-reference.md`, `docs/cli/settings.md`, `docs/reference/configuration.md`, `docs/tools/mcp-server.md`, `docs/cli/telemetry.md`, `docs/cli/tutorials/automation.md` under `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/`, fetched live at HEAD on 2026-08-18 and re-read at the `v0.55.1` tag on 2026-08-19 alongside `docs/cli/trusted-folders.md`, `docs/reference/policy-engine.md` and `docs/reference/tools.md`; plus `gemini --help` on both 0.18.4 and 0.55.1, and live probe runs on each.

```sh
cd "$TRIAL_DIR"   # per-trial cwd; settings come from the system layer, see below
gemini -p "<rendered prompt>" \
  --output-format json \
  --approval-mode yolo \
  --skip-trust \
  --allowed-mcp-server-names filesystem \
  --model gemini-2.5-flash \
  > "$TRIAL_DIR/client.json"
```

| Flag | Behavior that matters here |
| --- | --- |
| `-p` / `--prompt` | Forces non-interactive mode. Deprecated in favor of a bare positional prompt but still functional on 0.55.1 and still used in current doc examples. A bare positional prompt is interactive in a TTY and only goes headless when stdin or stdout is not a TTY, so `-p` is the form a runner should use: it does not depend on how the harness happens to be attached. |
| `-i` / `--prompt-interactive` | Not this. It runs the prompt and then drops into the REPL. |
| `-o` / `--output-format <text\|json\|stream-json>` | Same three shapes as Claude Code. Note that `stream-json` carries a *different*, simplified stats object from `json`; they are not interchangeable sources for section 3.2. |
| `--approval-mode <default\|auto_edit\|yolo\|plan>` | `yolo` auto-approves everything. `-y` / `--yolo` is the deprecated spelling. Silently downgraded to `default` in an untrusted directory; see `--skip-trust`. |
| `--skip-trust` | Required from 0.55 on. Marks the working directory trusted for the session. Its entire implementation is to set `GEMINI_CLI_TRUST_WORKSPACE=true` at argument-parse time, so the flag and the variable are the same switch; the variable is checked first and unconditionally, ahead of every other trust source. Without one of them a per-trial directory is untrusted and the run dies; see item 6 below. |
| `--allowed-mcp-server-names <names...>` | Restricts which of the configured MCP servers load. This is the nearest equivalent to `--strict-mcp-config`, and it is weaker: it filters a configured set rather than replacing it. |
| `--allowed-tools <names...>` | **Deprecated on 0.55.1** in favour of the policy engine, and not what this suite wants anyway: it marks tools as auto-approvable, it does not restrict which are registered. Tool isolation is a settings key; see item 2 below. |
| `-m` / `--model` | Defaults to `auto`. Pin it. See the note on `auto` below. |
| `--policy` / `--admin-policy <paths...>` | New on 0.55. Loads additional policy-engine rule files. Not used by this runner; item 7 explains why the allowlist form was preferred over a deny-by-name policy. |

**MCP wiring is by settings file, not by flag.** The `mcpServers` block lives in `~/.gemini/settings.json` or a project `.gemini/settings.json`, in the shape `interposer/README.md` already shows. Per-key detail from `docs/reference/configuration.md`: at least one of `command`, `url`, `httpUrl` is required, precedence is `httpUrl` then `url` then `command`; `timeout` defaults to 600000 ms; `trust` defaults to `false`; `excludeTools` wins over `includeTools`; and server aliases must not contain underscores, which break the `mcp_<server>_<tool>` name parsing.

**There is no flag or environment variable that redirects the user or project settings file.** `GEMINI_CLI_SYSTEM_DEFAULTS_PATH` and `GEMINI_CLI_SYSTEM_SETTINGS_PATH` redirect the system-level layers only. The consequence for the runner is concrete: a Gemini trial must run with its working directory set to a per-trial directory containing a `.gemini/settings.json` that names exactly one server, pointed at the interposer with that trial's log path. Claude Code takes the equivalent file as a `--mcp-config` argument and needs no such directory. This is the single largest asymmetry between the two runners and it belongs in the runner's design, not in a wrapper script written the morning of a run.

**The turn cap is a settings key, not a flag.** `model.maxSessionTurns` (default `-1`, unlimited) is set in the same per-trial `settings.json`. Headless mode has a dedicated exit code `53` for "turn limit exceeded". To keep the trial matrix even, set `maxSessionTurns` to the same number passed to Claude Code's `--max-turns`.

**What `--output-format json` gives section 3.2 and section 4.** Verified against live runs on both 0.18.4 and 0.55.1. Every field the runner reads survived the upgrade; the additions below are marked:

```json
{
  "session_id": "<uuid>",
  "response": "<final message>",
  "error": null,
  "stats": {
    "models": {
      "<model-id>": {
        "api":    {"totalRequests": 1, "totalErrors": 0, "totalLatencyMs": 2875},
        "tokens": {"prompt": 1367, "candidates": 46, "total": 1503,
                   "cached": 0, "thoughts": 90, "tool": 0, "input": 1367},
        "roles":  {}
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

New or newly relevant on 0.55.1: `session_id` and a top-level `error` object are present at the document root, `tokens.input` appears alongside `tokens.prompt` (it is `prompt` minus `cached`, not a second prompt figure, so do not sum them), and each model carries a `roles` breakdown. A top-level `warnings` array can also appear. The `error` object is the one the runner has to act on rather than merely record: it is populated on a document that is otherwise perfectly well formed, and section 4.1's `client_error` detection rule turns on it.

**The keys of `stats.tools.byName` are not the server's tool names, and the spelling changed under the upgrade.** On 0.18.4 an MCP tool appeared under its bare server-side name; on 0.55.1 it appears under the fully qualified `mcp_<server>_<tool>`, for instance `mcp_playwright_browser_navigate` for a tool the wire calls `browser_navigate`. Anything correlating this object against the interposer log has to reconcile the two spellings. Section 4.2 records what happened when the runner did not.

**Gemini CLI reports no cost figure.** There is no dollar field anywhere in the output, only token counts and latencies. Cross-client cost comparison therefore cannot be built from the clients' own cost reporting, and must not be: the comparable basis is the wire token count from the interposer under one tokenizer, per `methodology-v0.md` 1.6, which is what Tier 2 measures anyway.

**Do not leave `--model` at `auto`.** In the verification run, `auto` resolved to two models for a single prompt, a lite routing pass plus the answering model, each with its own token block in `stats.models`. Client-reported usage is then not attributable to one model, which breaks the section 3.2 record before it reaches section 4. This is distinct from OPEN 8 (which asks whether client versions are pinned) and is settled here: the model is pinned per trial, by flag, and recorded.

**Telemetry is configuration, not a flag.** `telemetry.enabled`, `telemetry.target`, `telemetry.outfile`, `telemetry.otlpEndpoint` and friends are settings keys with `GEMINI_TELEMETRY_*` environment overrides; the OTel stream includes `gemini_cli.token.usage`. No `--telemetry` or `--telemetry-outfile` CLI flag was found in `gemini --help` on 0.18.4 or 0.55.1, or in the current `cli-reference.md`; treat any claim that those flags exist as unverified. The suite does not need telemetry: `--output-format json` already carries the per-model token counts.

Client version comes from `gemini --version` (`-v` works too).

**Four things the shakedown of 2026-08-18 added to this invocation.** The Gemini side needed more than the Claude side, and every item below is a fault that produced either a dead run or a fake measurement before it was fixed.

1. **The per-trial settings file goes in the system layer, not the working directory.** A filesystem trial has to run with its working directory set to the fixture root (section 7.1, item 2, and Gemini honors roots the same way), and dropping a `.gemini/settings.json` there would put harness files inside the tree the task is measured against. `GEMINI_CLI_SYSTEM_SETTINGS_PATH` redirects the system settings layer to any path, which is the one layer that can be pointed at a per-trial file, and the user layer still merges underneath so the operator's auth selection survives. Combined with `--allowed-mcp-server-names`, the trial loads exactly one server and no `.gemini` directory is created in the fixture.
2. **Built-in tools are switched off with `tools.core`, never with `tools.exclude`.** Exclusion matches a bare tool name, and the filesystem server's surface collides with Gemini's built-ins on `read_file`, `write_file` and `list_directory`, so an exclusion list naming those removed the *server's* tools as well: FS-03 and FS-04 failed with the model reporting that `write_file` did not exist while `create_directory` and `edit_file` did. `tools.exclude` is deprecated on 0.55.1 besides, in favour of the policy engine. The `tools.core` allowlist is what the runner uses. **The value changed on 2026-08-19 from `[]` to `["mcp_*"]`; see item 7 below, which is the reason and is not optional.**
3. **A project `.env` anywhere above the working directory hijacks auth.** Gemini CLI walks up from its working directory, takes the first `.env` it finds, and only falls back to `~/.gemini/.env` when it finds none. The operator's `GOOGLE_CLOUD_PROJECT` lives in that fallback file and this repo has a `.env` of its own, so every trial run from inside the repo loaded the repo file, never loaded the fallback, and died with `ProjectIdRequiredError` before issuing a single tool call. The runner passes `GOOGLE_CLOUD_PROJECT` explicitly in the subprocess environment, taken from the environment or from `~/.gemini/.env`, which reproduces what the client would have loaded on its own outside the repo. An environment variable already set wins over the `.env` file, so this holds whichever file the client finds.
4. **The same `.env` is a billing hazard.** That file also carries `GEMINI_API_KEY`, and a loaded API key can switch the client off the operator's OAuth session onto metered billing. The runner strips the key variables from the subprocess environment and sets `advanced.excludedEnvVars` so a project `.env` cannot reintroduce them, the same cost-safety rule section 7.1 states for Claude Code. Re-probed on 0.55.1 on 2026-08-19 with an invalid `GEMINI_API_KEY` planted in a parent directory's `.env`: the trial completed on OAuth with `advanced.excludedEnvVars` both set and cleared, because the operator's user-layer `security.auth.selectedType: "oauth-personal"` pins the auth path ahead of a stray key. The exclusion is therefore belt-and-braces on this machine rather than the only thing standing between the suite and a metered wallet. It stays: it is the half of the guard that does not depend on the operator's settings file staying as it is.

    One doc correction while re-reading this: the documented final fallback for `.env` resolution is `~/.env`, not `~/.gemini/.env` as item 3 says. It makes no difference to the runner, which reads `~/.gemini/.env` by name because that is where this operator's `GOOGLE_CLOUD_PROJECT` actually lives, and passes the value explicitly either way.

**One more, from the playwright shakedown of 2026-08-18. It was a blocker, and the 2026-08-19 client upgrade cleared it.**

5. ~~**Gemini CLI 0.18.4 cannot call a server whose tools declare JSON Schema draft 2020-12.**~~ **Fixed by the client upgrade. Kept for the record.** On 0.18.4 the client validated a call's arguments against the advertised schema and its bundled validator had no 2020-12 meta-schema registered, so the call failed inside the client with `no schema with key or ref "https://json-schema.org/draft/2020-12/schema"` and never reached the wire. It was a property of the dialect, not of the server: `@modelcontextprotocol/server-filesystem` declares draft-07 and `github-mcp-server` declares no `$schema` at all, and both suites ran clean on the same client, while `@playwright/mcp@0.0.79` declares 2020-12 on all 24 tools and every playwright trial lost calls to it. Upstream fixed it in `schemaValidator.ts` well before the version this estate was running: a dedicated draft-2020-12 validator instance is dispatched on the schema's own `$schema`, with a lenient skip-and-warn fallback for any dialect it still does not know (gemini-cli issue #14970, PR #15060, shipped in 0.28.0). Confirmed live on 0.55.1 on 2026-08-19: PW-01 through PW-05 all pass, tool-call gap zero on every trial, no schema error anywhere in client stderr. See OPEN 11.

**Five things the 2026-08-19 client upgrade changed, 0.18.4 to 0.55.1.** The upgrade was not a drop-in. Items 6 and 7 each produced a run that died before issuing a single call, item 8 turned a working detector into one that reports the healthy value forever, and item 9 is the classification fault that produced the 1.0.1 bump. Every one of them is in the runner now.

6. **A per-trial working directory is untrusted, and an untrusted directory is not runnable.** From 0.55 the client refuses a headless run outside a trusted folder: `FatalUntrustedWorkspaceError`, exit code **55**, and nothing at all on stdout, so the trial records as `client_error` and measures nothing. Two lesser effects fire first on the same check and either one alone would corrupt a trial quietly rather than loudly: `--approval-mode yolo` is downgraded to `default`, and **every MCP server is disabled**, including the one the suite exists to measure (`gemini mcp list` says so in as many words: "MCP servers are configured but disabled because this folder is untrusted"). Every trial directory this runner creates is new, so none of them is ever trusted. Four mechanisms exist; the runner uses the first and cheapest.

    | Mechanism | Note |
    | --- | --- |
    | `--skip-trust` | What the runner passes. Sets `GEMINI_CLI_TRUST_WORKSPACE=true` internally at parse time. |
    | `GEMINI_CLI_TRUST_WORKSPACE=true` | The same switch as an environment variable, checked first and unconditionally. Both verified working on 0.55.1. |
    | `security.folderTrust.enabled: false` | Disables folder trust wholesale. Heavier than needed, and it trips the client's own "this project attempts to disable folder trust" security warning. |
    | `~/.gemini/trustedFolders.json` | A path-to-`TRUST_FOLDER`/`TRUST_PARENT`/`DO_NOT_TRUST` map, redirectable with `GEMINI_CLI_TRUSTED_FOLDERS_PATH`. Persistent machine state for a directory that exists for one trial; rejected for that reason. |

7. **`tools.core: []` is no longer a working isolation and takes the whole trial down with it.** On 0.55.1 any truthy `tools.core` makes the client append a wildcard `*` DENY rule to the policy engine at a priority just under the allowlist's own ALLOW rules, and that DENY carries no MCP exemption. With the array empty nothing matches the allowlist, the wildcard strips the MCP tools too, the request goes out with no tool declarations, and the API rejects it: `tools[0].tool_type: required one_of 'tool_type' must have one initialized field`. Reproduced on 0.55.1 both with and without an MCP server configured, so the empty array is the whole cause. This is upstream's open issue #28361; three attempted fixes were closed unmerged, so it is live in the shipped version.

    **The runner sets `tools.core: ["mcp_*"]`,** which is the fail-closed form of the same intent. Gemini names an MCP tool `mcp_<server>_<tool>`, so every server tool matches the allowlist explicitly and outranks the wildcard DENY, while no built-in name matches and all of them fall through to it. Verified on 0.55.1: a read task calls `mcp_filesystem_read_text_file` with exactly one frame on the wire, and a prompt demanding a shell command comes back "I do not have a tool to execute shell commands" with `run_shell_command` recorded in `stats.tools.byName` as an attempted-and-failed call the server never saw. It also survives the upstream fix, which is why it was preferred to the alternative: **a policy file denying each built-in by name** would work today and is the mechanism upstream recommends, but it is a list that has to be kept current against a client that adds tools, and a built-in added after the list was written would silently answer a task instead of the server. The allowlist fails closed on that same event. Note the corollary: the server alias must keep matching `mcp_*`, and per the settings note above it must not contain an underscore.

8. **The client renamed MCP tools in its usage output, which silently killed the tool-call gap.** `stats.tools.byName` keys moved from the bare server-side name to `mcp_<server>_<tool>`. Section 4.2 has the full account; the short version is that the gap's definition sums positive differences, the rename made every difference negative, and the detector would have read a clean zero forever while detecting nothing. The runner now sums both spellings per advertised tool.

9. **A failed trial can come back as a valid document that says so.** Gemini reports an API-side failure by populating the top-level `error` object rather than by dying, so a trial can carry a well-formed result, successful tool calls, correct token accounting and an empty answer. PW-02 did exactly that on the first re-shakedown pass, with `error.type: "INVALID_STREAM"`. Classifying on JSON validity alone bucketed it `tool_use_failed`, a capability claim about the server on the strength of a transient API fault. Section 4.1's detection rule and the 1.0.1 bump are the fix.

10. **What did not change.** `GEMINI_CLI_SYSTEM_SETTINGS_PATH` still redirects the system settings layer and the user layer still merges underneath it, so item 1 holds unaltered and no `.gemini` directory is created in a trial's working directory (re-checked on 0.55.1). `model.maxSessionTurns`, `advanced.excludedEnvVars`, and the `mcpServers.<name>` keys `command`, `args`, `trust`, `includeTools`, `excludeTools` and `timeout` all still exist under those names. There is still no way to redirect the user or workspace settings layer; `GEMINI_CLI_HOME` moves the whole `~/.gemini` directory and is not a settings-path override. `-p`, `--output-format json`, `--approval-mode yolo`, `--allowed-mcp-server-names` and `-m` all behave as section 7.2 already described.

**The model is pinned by full id, not by alias.** The runner passes `gemini-2.5-flash`, not the `flash` shorthand. An alias is free to resolve to a different model between runs, which is the same failure mode the `auto` note above rejects. The id the client actually billed is read back out of `stats.models` and recorded per trial either way.

**The pin is not absolute, and the recording is what catches that.** One trial of the 2026-08-18 shakedown, PW-02 on Gemini, billed both `gemini-2.5-flash` (2408 output tokens) and `gemini-2.5-pro` (1682) despite `--model gemini-2.5-flash`. It was also the slowest trial of the day at 151 seconds, so the second model looks like a fallback after repeated failures rather than a routing decision. Whatever the cause, a trial that consumed two answering models does not have a single attributable per-model figure, which is exactly the condition the `auto` note above rejects, reached by a different route. The `models_used` field is the detector: it lists every model the trial billed with its token counts, so a cell carrying more than one answering model is visible rather than averaged away. See OPEN 13.

On 0.55.1 the pin held on all 15 trials of the 2026-08-19 re-shakedown: every trial recorded exactly one entry in `models_used`, `gemini-2.5-flash`, and `resolved_model` matched the pinned id every time. That is 15 trials against the one that misbehaved on the old client, which is evidence and not a guarantee; the detector stays and OPEN 13's reporting question is untouched by it.

### 7.3 The playwright server

Verified live on 2026-08-18 against `@playwright/mcp@0.0.79`, which reports itself as Playwright 1.63.0-alpha-2026-08-05 and advertises 24 tools.

```sh
npx -y @playwright/mcp@<pinned> --headless --isolated \
  --browser chromium --output-dir "$TRIAL_OUT"
```

**`--browser` is not optional, and the fault it repairs is quiet.** The server defaults to the `chrome` channel, meaning branded Google Chrome at `/opt/google/chrome/chrome`, which is not installed on this estate. The server still starts, still completes `initialize`, and still lists all 24 tools; only the first tool call that needs a page fails, with `Chromium distribution 'chrome' is not found`. A run without the fix therefore reads as five task failures rather than as a setup error, which is the worst possible shape for a benchmark fault. `--browser chromium` resolves to the Playwright-managed chrome-for-testing build under `~/.cache/ms-playwright/`. `LOADLINE_TIER2_PW_BROWSER` overrides it.

**The system chromium is deliberately not used.** `/usr/bin/chromium-browser` exists on this box but is a shim for the chromium snap, and a snap-confined browser cannot reliably write a file to an arbitrary path, which is exactly what PW-05 requires.

**The browser is a preflight, not a hope.** `run-suite.sh` runs `npx @playwright/mcp@<pinned> install-browser <browser>` once before the first playwright trial. A missing browser then stops the run instead of consuming five trials of plan quota producing failures that mean nothing.

**`--output-dir` points at the trial's output directory** so the server's own artifacts (page snapshot files, console logs) and PW-05's screenshot land there rather than in a `.playwright-mcp/` directory beside the working directory.

**`browser_navigate` does not return the page inline.** It returns a link to a `.yml` snapshot file. A client that needs the page content calls `browser_snapshot` afterwards, which does return it inline. That extra call is real cost the tasks measure, not a fixture problem, and it is why a PW read task costs two calls rather than one.

### 7.4 The github server

Verified live on 2026-08-18 against `ghcr.io/github/github-mcp-server` v1.9.0, which advertises 44 tools and declares no `$schema` on any of them.

```sh
docker run -i --rm --env-file "$GH_TOKEN_FILE" \
  ghcr.io/github/github-mcp-server stdio
```

**Packaging: the default ghcr container.** Docker is available on this machine and the container is the vendor's own distribution, so it is what the runner uses by default. `LOADLINE_TIER2_GITHUB_CMD` overrides the whole command for a machine that has the Go binary instead, and whatever is used is recorded in the run header as `server_pkg`.

**The digest, not the tag, is the pin from the next run on.** Added 2026-08-20 (task-suite 1.0.4, section 5). Neither the command above nor `server_pkg` names a tag, so this image reference resolves to whatever `latest` currently is, and even a stated tag like `v1.9.0` is mutable. `run-suite.sh` now resolves the digest the run actually used, once before the first trial (`docker pull ghcr.io/github/github-mcp-server` then `docker image inspect --format '{{index .RepoDigests 0}}' ghcr.io/github/github-mcp-server`), and records it as `server_image_digest` on the run header and every trial row, re-resolving it again after the last trial to catch a tag that moved mid-run (section 5). A docker fault at either point (missing binary, failed pull, no repo digest reported) is recorded as a `"digest unavailable: ..."` string rather than stopping the run. `LOADLINE_TIER2_GITHUB_CMD` overriding this server to a non-container command leaves `server_image_digest` `null`, since there is no image to pin.

**The credential reaches the server and nothing else.** The token is written to a `0600` file in a `700` directory created for the suite and removed by an exit trap, and the container reads it with `--env-file`. It is never in the client's environment, never in an argv that a `ps` listing can read, never in the MCP config file the client parses, and never in the manifest. `-e GITHUB_PERSONAL_ACCESS_TOKEN`, which the earlier draft of this section used, forwards the variable from the docker CLI's own environment, and the docker CLI is a grandchild of the client process, so that spelling requires the token to be in the client's environment. That is the thing the rule forbids.

**Which token.** `LOADLINE_TIER2_GITHUB_TOKEN` if the operator sets one, otherwise `gh auth token`. Taking it from `gh` keeps one credential in play across the server, `reset.sh`, and the GH-03 and GH-04 state checks, so there is one thing to revoke. Note that the fixture repo is private (OPEN 5), so the credential needs `repo` scope; the operator's gh token has it.

### 7.5 The runner

Built as `tier2/run-suite.sh`. One invocation is one (server, client) pair, trials serial within it per section 6.

```sh
tier2/run-suite.sh --server filesystem|playwright|github \
                   --client claude|gemini \
                   [--trials N] [--tasks FS-01,FS-03] [--date YYYY-MM-DD] \
                   [--out DIR] [--timeout SECONDS] [--max-turns N] \
                   [--model ID] [--no-full-results] [--dry-run] [--force]
```

Defaults: three trials, all five tasks for the server, today's UTC date, `data/tier2`, a 300 second per-trial timeout, a 20 turn cap on both clients, and `--full-results` on so section 4's call-response result tokens are measured rather than left null.

Per trial it generates `{trial_id}`, prepares the fixture (fresh scratch copy for filesystem, a served-pages check for playwright, `tier2/fixtures/github/reset.sh` before every trial for github), writes the per-trial client config with the interposer wrapping the server, invokes the client per this section with wall-clock timing around it, evaluates the section 2 success criterion immediately, runs `loadline analyze` on the frame log, and appends the manifest row of section 6. `--dry-run` prints the plan and every rendered prompt without contacting anything. `--force` re-runs cells already present in the day's manifest, for use after a harness fix; see section 6.

**The resume key includes the suite version.** A trial is skipped as already done on (suite version, server, client, task, trial), not on the four fields section 6 originally named. A task-suite MAJOR bump makes prior rows incomparable by definition (section 5), so a re-invocation after a bump must re-run the cell rather than resume over rows recorded under the criteria the bump replaced.

Aggregation into `summary.json` is written and is `tier2/summarize.sh` (section 6). All three suites have now been shaken down on both clients at one trial per task, on 2026-08-18, and the Gemini half was re-shaken down in full on 2026-08-19 against the upgraded client under `--date 2026-08-19`. No full 90-trial run has been executed.

**A client upgrade is not a harness fix and the two are not run the same way.** The 2026-08-19 re-shakedown was written to a new date directory rather than forced over the 2026-08-18 rows. Nothing about the older rows became wrong; they are a correct record of Gemini CLI 0.18.4, which is what the run header stamps them with. `--force` supersedes a row on the claim that the setup which produced it was broken, and that claim would be false here. The version stamps in the run header are the mechanism section 5 provides for exactly this, and using them is cheaper and more honest than overwriting history.

---

## 8. OPEN items

None of these blocks writing the spec. The unresolved ones block actually running a suite.

1. **RESOLVED 2026-08-18. Claude Code non-interactive automation.** Confirmed against client 2.1.234 and current docs; see section 7.1.
2. **RESOLVED 2026-08-19. Gemini CLI equivalent.** First confirmed against client 0.18.4 on 2026-08-18, reopened by the client upgrade the next day, and re-confirmed end to end against 0.55.1 and the docs at the `v0.55.1` tag; see section 7.2, whose items 6 through 9 are what the upgrade changed.
3. **RESOLVED 2026-08-19. Cost attribution under client-side caching: record, do not control.** Trials are not forced cold. Each trial captures its own cache figures from the client's own JSON output, `usage.cache_creation_input_tokens` and `usage.cache_read_input_tokens` for Claude Code and `stats.models.<id>.tokens.cached` for Gemini CLI, and publishes them as per-trial fields alongside the trial's ordinal within its cell and within the suite invocation. Rationale: a forced-cold session is not how either client is actually used, so it would measure a configuration nobody runs, and neither client offers a documented, reliable way to guarantee one, so forcing it would be a claim the harness cannot back. Recording makes the cache effect visible in the published data instead of hiding it inside an average. See section 3.2. *Decided by the session under the operator's standing instruction; veto window open.*
4. **RESOLVED 2026-08-18. The analyze step exists.** It is `loadline analyze --log <trial.jsonl> [--out <report.json>]`, implemented in `internal/t2analyze`, not a subcommand of the interposer. Keeping it out of the interposer preserves that binary's stdlib-only property, which `interposer/README.md` names as the reason per-call token counting was a v0.1 non-goal in the first place; analysis runs offline on a finished log and has no reason to inherit the constraint. It correlates requests to responses by id, counts `params_full` under `o200k_base` via the same tokenizer Tier 1 uses (`methodology-v0.md` 1.6), and emits the section 4 per-trial metrics stamped with both the log's `interposer_version` and its own analyzer version. One limit is worth stating up front: call-response **result** tokens are only measured when the log was produced with the interposer's `--full-results`, because without it the log carries a byte count and a text length for each result but not the text. The report leaves that figure null and says so rather than estimating it.
5. **RESOLVED 2026-08-18 (one deviation). GitHub fixture repo exists.** `lopster568/loadline-tier2-fixture` was created and seeded to the section 2.3 baseline via the operator's authenticated gh CLI: `NOTES.md` with `TARGET: 42`, `docs/alpha.md` and `docs/beta.md` only, exactly one open issue titled `Fixture issue: do not close` labeled `tier2-fixture`, default branch `main`, baseline tagged `tier2-baseline`. Deviation: the repo is **private**, not public as this section specified, to avoid leaking the project before launch; flip to public at launch day alongside the main repo, or keep private and grant the trial credential access, whichever the operator prefers. The reset/cleanup automation is now `tier2/fixtures/github/reset.sh`: it deletes `tier2-probe-*` issues, closes pull requests from `tier2/*` branches and deletes those branches, reopens and relabels the fixture issue if a trial closed it, and then verifies the baseline by comparing `main` against the `tier2-baseline` tag and re-reading the `NOTES.md` TARGET line and the `docs/` listing. Content drift on a tracked file is reported and exits non-zero rather than being force-repaired, because repairing it would destroy the evidence of how it happened. `reset.sh --check` was run against the live repo on 2026-08-18 and reported it at baseline. The mutating path is no longer unexercised: it now runs before every github trial (section 2.3) and during the 2026-08-18 shakedown it deleted probe issues, closed pull requests and deleted `tier2/*` branches on every pass following a GH-03 or GH-04 trial, with `--check` confirming baseline afterwards. What remains open in this item is only the visibility question: flip the repo to public at launch day alongside the main repo, or keep it private and keep granting the trial credential access. The runner works either way; the credential needs `repo` scope while it stays private.
6. **RESOLVED 2026-08-18. Playwright fixture pages and local server are built.** They live in `tier2/fixtures/playwright/`, with the pages under `site/` rather than directly under the directory as section 2.2's path line says, and `serve.sh` starting `python3 -m http.server` bound to `127.0.0.1` on `${LOADLINE_TIER2_HTTP_PORT:-8930}`. python3 is the tooling choice: present on both machines, no dependency added to the repo, static and read-only with no configuration. The filesystem seed tree of section 2.1 is built alongside it by `tier2/fixtures/filesystem/setup.sh`, which has a `--verify` mode that rebuilds into a temp directory and fails on drift from the committed tree. Both fixture directories carry a README mapping each task id to its prerequisites and a mechanical success-check command.
7. **RESOLVED 2026-08-19. Decline-rate parsing rule.** A trial with zero `tools/call` frames is classified by its success check: passing is `answered_without_tools` (a flagged anomaly, counted as a failure for tool-use metrics and reported separately from a decline), failing with a refusal marker in the final message is `declined`, and failing without one is `failed_no_tool_use`, the hallucination bucket. All three publish. The rule, the documented marker list, and the heuristic caveat are stated in full in section 4.1, which is the normative text; this entry is the pointer. The shakedown proved the rule earns its keep: Claude Code answered FS-01 correctly with zero tool calls on the first attempt. *Decided by the session under the operator's standing instruction; veto window open.*
8. **RESOLVED 2026-08-19. Client version pinning: detect drift, do not pin.** Neither client can be held at a released version by any supported mechanism, so the suite records instead of pinning. `claude --version` and `gemini --version` go into the run manifest header before the first trial and are re-read after the last one; if either changed, the run footer carries `version_drift: true` and the run is not comparable, internally or against any other run. Both clients are recorded on every run, whichever one the suite invoked. See section 5. *Decided by the session under the operator's standing instruction; veto window open.*
9. **RESOLVED 2026-08-18 (task-suite 1.0.0). The two misfiring success criteria are fixed.** FS-02 drops its negative clause, which tested phrasing rather than capability. FS-04 compares after a stated normalization of path spelling and numeric adornment, which cannot turn a wrong answer into a right one. Two smaller criterion changes of the same class ride along: GH-05 drops its never-implemented negative clause, and the GH-03 and GH-04 state checks poll rather than read once. The full rationale, including why the comparability cost is nil (zero published Tier 2 runs exist), is the changelog entry at the head of this document. Revalidated on 2026-08-18: FS-02 and FS-04, both clients, one trial each, 4 of 4 pass. *Decided by the session under the operator's standing instruction; veto window open.*
10. **RESOLVED 2026-08-18 (one cell was blocked then; unblocked 2026-08-19, see OPEN 11). The playwright and github suites have now been run.** Both suites were shaken down on both clients, one trial per task, on 2026-08-18, and the whole 24-cell shakedown was then re-run once more under the final harness so that every standing row carries the same fixes. Standing results as of that day: filesystem 4 of 4, playwright/Claude 5 of 5, github/Claude 5 of 5, github/Gemini 4 of 5, playwright/Gemini 0 of 5 with every trial flagged `harness_suspect` and therefore not a capability reading (OPEN 11).

    **Superseded on the Gemini side by the 2026-08-19 re-shakedown against Gemini CLI 0.55.1** (OPEN 14), one trial per task, written to its own date directory because the older rows are a correct record of the older client and not a broken setup: filesystem 4 of 5, playwright 5 of 5, github 5 of 5, 14 of 15 overall, tool-call gap zero and `harness_suspect` false on every trial. The one failure is FS-04 and it is a capability result, not a fault: the model reported `logs/access.log` as the longest file, which is correct, and 100 lines, which is not, against an actual 137. That is the same miscount the 1.0.0 changelog names as the case its FS-04 normalization deliberately still fails, so the criterion behaved as designed. The Claude rows are unchanged and were not re-run; the Claude client did not move. Four further faults came out of that day and are documented in section 7.2 items 6 through 9.

    Seven harness faults were found and fixed on 2026-08-18 and are documented in sections 2.3, 4.2 and 7.1 through 7.4: the playwright browser channel, the Claude Code settings-source leak, the GitHub state-check race, the mid-run auto-update, the `resolved_model` misattribution, the suite-version-blind resume key, and `reset.sh` missing comments on the fixture issue. The GitHub packaging decision is settled: the default ghcr container, credential delivered by `--env-file` (section 7.4). `reset.sh`'s cleanup path is no longer unexercised; it ran before every github trial and closed pull requests, deleted branches, deleted probe issues and deleted a stray comment, and `reset.sh --check` reported the repo at baseline afterwards.

    One fault is worth stating in full because it is the fixture's own, not a client's. Gemini CLI answered GH-02 by calling `add_issue_comment` with the answer in the body and then saying only "I have completed the request." The transcript check failed it correctly, on its merits. But the answer stayed in the repo, and `reset.sh --check` reported the fixture at baseline anyway, because nothing was comparing comment counts. A GH-02 trial run after that one would have found the previous trial's answer sitting in the comment thread. `reset.sh` now deletes comments on the fixture issue and reports them as drift under `--check`.
11. **RESOLVED 2026-08-19. Fixed by the client upgrade; the playwright x Gemini cell is measurable and the full run is 90 trials.** Of the three options this item listed, the middle one was taken and it cost one afternoon rather than the task-suite MAJOR the third would have. Gemini CLI was upgraded 0.18.4 to 0.55.1 and the draft 2020-12 defect is gone: upstream had already fixed it in 0.28.0 (issue #14970, PR #15060), by dispatching a dedicated 2020-12 validator on the schema's own `$schema` and falling back to skip-and-warn for unknown dialects rather than failing the call. Re-shakedown on 0.55.1, one trial per task: **playwright 5 of 5, tool-call gap zero on every trial, no schema error anywhere in client stderr**, against 0 of 5 with every trial `harness_suspect` on the old client. The server substitution is off the table and Chrome DevTools MCP stays where it was, on the reserve bench. What the upgrade did cost is section 7.2 items 6 through 9: two changed mechanisms that each killed a run outright, one that killed the gap detector silently, and one classification fault that produced the 1.0.1 bump. OPEN 2 is reopened and re-closed by the same re-verification; section 7.2 is current as of 2026-08-19.
12. **Should a client-side tool-call failure get its own classification bucket, and how is it detected on Claude Code?** Section 4.2 records the tool-call gap and deliberately does not act on it, because adding a bucket changes the section 4.1 taxonomy. Two questions have to be answered together. First, whether a trial with a positive gap should be classified out of the capability buckets the way `client_error` already is, or stay in them with the flag as a caveat. Second, the gap is Gemini-only today, because Claude Code's JSON output carries no per-tool call count to compare against the wire; the same fault on the Claude side would currently be invisible, which makes any cross-client reading of the flag asymmetric. A `--output-format stream-json` run exposes per-tool-use events and may close that hole at the cost of a second output format in the runner.
13. **A pinned model is not guaranteed to be the only model that answers.** One shakedown trial billed `gemini-2.5-pro` alongside the pinned `gemini-2.5-flash` (section 7.2). Claude Code separately bills a small auxiliary model on every trial (section 7.1 item 6). Both are now recorded per trial in `models_used`, so nothing is hidden, but the reporting question is unanswered: does a published Tier 2 cell state the pinned model, the model that produced the most output, or every model billed, and is a trial whose answering model was not the pinned one excluded from a per-model reading or published with a flag. This has to be settled before a figure is published against a model name, because that name is what a reader will take the number to be about.

    **Checked on the new client, 2026-08-19, and the answer changes nothing.** All 15 trials of the Gemini re-shakedown on 0.55.1 recorded exactly one entry in `models_used`, `gemini-2.5-flash`, with `resolved_model` matching the pinned id every time. No second answering model, no routing pass, no fallback. That is 15 clean trials against one dirty one on the old client, which moves the prior and settles nothing: the reason the earlier trial pulled in `gemini-2.5-pro` looked like a fallback after repeated tool-call failures, and tool calls no longer fail that way, so the most likely reading is that the upgrade removed *this* trigger rather than the mechanism. The Claude Code half of the item is untouched and is the half that fires on every single trial. Both detectors stay and the reporting question stands.
14. **RESOLVED 2026-08-19. The installed Gemini CLI is current.** Upgraded 0.18.4 to 0.55.1, the version on npm the same day, so a published Tier 2 figure for Gemini CLI now describes the client people are actually running and the representativeness objection this item raised is closed. Section 7.2 was re-verified end to end against 0.55.1 and four invocation details changed; all four are documented there as items 6 through 9 and all four are in the runner. The re-shakedown was run on the new client and its rows carry `0.55.1` in the run header, which is OPEN 8's machinery doing precisely what it was designed for: no pin was claimed, the version was recorded, and the change between the two days is legible in the manifest rather than hidden inside a comparison.

    Two things this does not buy. It is a snapshot, not a standing property: 0.55.1 was current on 2026-08-19 and the same gap can reopen at this client's release cadence, so the version delta is worth re-checking before any publication run rather than assumed. And the upgrade cost is real and is the argument against treating "stay current" as free, since two of the four changes it forced would have killed a run outright and one would have disabled a detector without saying so. The rule that follows for the next upgrade is section 7's own: re-verify before the run, not after it.
