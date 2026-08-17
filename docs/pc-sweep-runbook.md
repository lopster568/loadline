# PC Sweep Runbook: First Full 15-Server Tier 1 Sweep

Status: operator handoff, first full run.
Governs: `servers.yaml` as amended 2026-08-17 (15 ratified servers, `docs/server-selection.md`).
Machine: run this on Roshan's PC, not the dev laptop. The dev machine is too
light for a 15-server run with live containers, npm/uvx cold installs, and
two token-counting API calls per server; standing practice is heavy runs on
the PC, prepared and handed off rather than run locally.

This document assumes `servers.yaml`, `docs/server-selection.md`, and
`cmd/loadline/README-validation.md` as they stand on 2026-08-17. If any of
those files change before the run, re-read them before following the steps
below.

---

## 1. Prerequisites

Install and confirm on the PC before starting:

1. Go 1.24 or later (`go version`). The validation run used 1.24.6 linux/amd64.
2. Node.js with `npx` on PATH (`npx --version`). Validation used v22.14.0.
   Needed for the npm-packaged stdio servers: filesystem, notion, slack, sentry, playwright.
3. `uv` / `uvx` on PATH (`uvx --version`). Needed for the pypi-packaged stdio
   servers: fetch, postgres.
4. Docker, running (`docker ps`). Needed for: kubernetes (via a local `kind`
   cluster, itself backed by Docker) and postgres (local container). Install
   `kind` separately if not already present (`kind version`); it is not a
   Docker feature, it is a separate binary that uses Docker.
5. Two environment variables, both optional but recommended:
   - `ANTHROPIC_API_KEY`: enables the `counts.claude` cell (native Claude
     tools-param token count via `count_tokens`) for every server row.
   - `GEMINI_API_KEY` (or `GOOGLE_API_KEY`): enables the `counts.gemini` cell
     (native Gemini `countTokens`) for every server row.
   - Without either key, the harness does not estimate or fake a number. The
     cell publishes `available: false` with the missing-key reason recorded,
     exactly as it did in the 2026-08-17 validation run
     (`cmd/loadline/README-validation.md`). The `counts.openai_o200k` cell
     needs no key; it is a local tokenizer and is always available.
6. Confirm the corpus and PRD haven't moved since this doc was written:
   `servers.yaml`, `docs/server-selection.md`, `PRD.md`.

---

## 2. Credential checklist, per server

15 servers, driven by `servers.yaml`'s `auth.token_env` fields and the
auth-risk table in `docs/server-selection.md` B.3. Four servers (github,
linear, sentry, slack) carry a mandatory verification step beyond "get a
token": confirm the free-tier credential actually exposes the server's FULL
tool surface, not a scope-filtered subset. Do this by comparing the live
`tool_count` the harness measures against the server's own documented tool
list (README, docs page, or source, as cited in `docs/server-selection.md`
B.2). If the live count is lower than the documented full count, the
credential is partial. Do not run the sweep on a partial credential and
publish it as a normal ranked row; the row publishes as `partial_surface`
(see `internal/report/report.go`'s `StatusPartialSurface`), not a ranking.

### Dev tools

**github** — env var `GITHUB_TOKEN`.
- Create a free GitHub account token: classic Personal Access Token, broad
  scopes (repo, workflow, read:org, and the other scopes the toolsets in
  `github/github-mcp-server`'s docs require), or go through the server's
  OAuth flow if you can stand one up.
- REQUIRED VERIFICATION (B.3): the maintainer's own docs confirm a classic
  PAT silently hides tools it lacks scope for (`X-OAuth-Scopes` filtering at
  startup). The documented full surface is roughly 113 tools across 21
  toolsets. After the run, check the github row's `tool_count`: if it is
  well under 113, the classic PAT is scope-filtering and you need the OAuth
  flow instead, not the classic PAT, before this row can publish as a
  ranking.

**filesystem** — no auth. Already validated (`cmd/loadline/README-validation.md`,
14 tools, `ok`). Nothing to do here.

**fetch** — no auth, but broken upstream. See section 3 below before
touching this row.

**kubernetes** — no single env var; `servers.yaml` leaves the credential
convention as a TODO. Use a local `kind` cluster, no cloud account needed:
```
kind create cluster --name loadline
kind get kubeconfig --name loadline > ~/.kube/loadline-config
export KUBECONFIG=~/.kube/loadline-config
```
Confirm the harness's actual credential-passing convention (env var vs
mounted kubeconfig path vs in-cluster service account) against
`internal/sweep/acquire.go` before the run; it is not yet resolved there
either (see section 4).

**cloudflare** — env var `CLOUDFLARE_API_TOKEN`.
- Create a free Cloudflare account, then an API token (dashboard, My
  Profile, API Tokens), scoped to the products you intend to exercise.
- Known open question (not yet a credential blocker, but a scope-of-run
  decision): this server is 16 separate product-scoped remote servers at
  16 different subdomains, no single endpoint. Decide which product
  server(s) to hit before the run, per the TODO in section 4.

### Productivity / SaaS

**notion** — env var `NOTION_TOKEN`.
- Free Notion workspace, then an internal integration token at
  notion.so/my-integrations.
- Share at least one page or database with the integration. This affects
  what data the tools can see, not the tool count, but the server may
  behave oddly if the integration has zero connections; share something
  trivial.
- Not flagged as a partial-surface risk in `docs/server-selection.md`, but
  the doc notes Notion has not published an explicit statement that the
  tool list is plan-tier-invariant. Worth a quick tool_count sanity check
  against the documented 22 tools (v2.0.0) after the run.

**slack** — env var `SLACK_MCP_XOXC_TOKEN`.
- THIS IS A DECISION FOR ROSHAN, NOT SOMETHING TO RESOLVE UNILATERALLY
  (B.3, `docs/server-selection.md`: "Roshan's decision: stealth mode risks a
  ToS question, scoped OAuth mode is a genuine partial surface"). Two
  incompatible modes:
  - Scope-limited OAuth (standard bot/user scopes): straightforward, ToS-clean,
    but scope-gates the 18-tool list, i.e. fails Gate 2 outright on its own terms.
  - "Stealth mode" (`xoxc-`/`xoxd-` browser-session cookies pulled from a
    logged-in Slack web session): reaches the fuller surface, bypasses
    Slack's own OAuth scope system, and the README does not claim it is
    ToS-compliant.
  Get Roshan's call before creating either credential. Do not default to
  stealth mode to "get more coverage" without that call.
- If Roshan picks OAuth: create a free Slack workspace (a personal dev
  workspace is fine), create a Slack app, install it, grab the bot token.
- If Roshan picks stealth mode: extract `xoxc-`/`xoxd-` values from an
  authenticated browser session per the server's own README.
- Separately, `docs/server-selection.md` flags the repo's last-commit date
  as unresolved (default branch shows 2026-05-14, but `pushed_at` shows
  2026-07-16). Re-check the actual default branch's last commit before
  trusting Gate 4 (active maintenance) on this server.

**linear** — env var `LINEAR_API_KEY`.
- Free Linear workspace, then a personal API key from Linear settings.
- REQUIRED VERIFICATION (B.3, resolvable): use the full `mcp.linear.app/mcp`
  endpoint with a write-scoped credential. Do NOT point the harness at
  `mcp.linear.app/mcp/readonly` or a read-only-scoped OAuth grant; both
  truncate the tool surface by design. Confirm the endpoint the harness
  actually calls before the run.
- `servers.yaml` also leaves the auth mechanism itself as a TODO (OAuth 2.1
  vs. API key vs. enterprise SAML/Okta); pick API key (matches the declared
  `LINEAR_API_KEY` env var) unless there's a reason to prefer OAuth.

**stripe** — env var `STRIPE_API_KEY`.
- Free Stripe account, then a TEST-MODE restricted API key (starts
  `sk_test_`). Do not use a live key. Stripe's own docs recommend a
  restricted key to limit what the agent can actually do; the doc notes
  this affects capability, not the 11-tool `tools/list` count, so a
  restricted test key is fine for Tier 1 measurement.

**figma** — everything is an unresolved TODO in `servers.yaml`: auth,
transport, package, tool count. Pre-run research task, not a quick
credential grab:
1. Resolve the primary repo first: official `figma/mcp-server-guide` vs.
   community `GLips/Figma-Context-MCP`. `docs/server-selection.md` B.2 item
   10 names `figma/mcp-server-guide` as presumptive primary (official
   outranks stars per rule A.4) but flags this as "pending onboarding
   verification," not settled.
2. Once the repo is fixed, read its docs for the actual auth model (Figma
   personal access token is the likely shape, free Figma account) and
   transport, and get a free-tier token.
3. Only after that, run a `tools/list` pass to fill in the tool count.

### Observability

**sentry** — env var `SENTRY_ACCESS_TOKEN`.
- Free Sentry developer account, then an access token via OAuth (broad
  scope) or a personal auth token.
- REQUIRED VERIFICATION (B.3): source-confirmed partial-surface risk. 54
  total tools, most gated by `requiredScopes`. Get a BROAD-scope grant, not
  a narrow default one, and check the row's `tool_count` against 54 after
  the run. One tool, `analyze_issue_with_seer`, may additionally be gated
  to a paid Sentry plan; this is unconfirmed in the source doc, so note
  whether it's present or absent in the live run rather than assuming
  either way.

**jaeger** — no credential needed for the Jaeger backend path
(`BACKEND_URL=http://localhost:16686`; a token is only needed for
Traceloop's own backend, which this corpus does not use). Stand up a local
Jaeger instance:
```
docker run --rm -d --name loadline-jaeger \
  -p 16686:16686 -p 4317:4317 -p 4318:4318 \
  jaegertracing/all-in-one:latest
```
Set `BACKEND_URL=http://localhost:16686` and `BACKEND_TYPE=jaeger`
explicitly (the server also speaks Tempo and Traceloop's own backend by
default; pin Jaeger). The package block itself is a TODO in `servers.yaml`
(distribution channel, pinned version); see section 4.

### Data

**postgres** — env var `DATABASE_URI`. Local container, no external account:
```
docker run --rm -d --name loadline-pg \
  -e POSTGRES_PASSWORD=loadline -e POSTGRES_DB=loadline \
  -p 5432:5432 postgres:16
export DATABASE_URI="postgres://postgres:loadline@localhost:5432/loadline"
```
Launch the server in Unrestricted Mode for measurement, not Restricted Mode.
This is an operator-set launch flag, not a credential property; Restricted
Mode truncates the tool surface by flag, and would masquerade as a
partial-surface finding when it is actually just the wrong launch flag.
Confirm the harness (or its args) actually passes the unrestricted flag
before the run.

**context7** — everything is an unresolved TODO in `servers.yaml`: auth,
transport, package, tool count. Same treatment as figma: research the
`upstash/context7` repo's docs first (a free API key is the likely shape,
Context7 has historically offered both an unauthenticated mode and an
API-key mode for higher rate limits; confirm which the harness needs to
reach the full tool surface), get a free-tier credential if one is
required, then run a `tools/list` pass to fill in the tool count.

### Browser automation

**playwright** — no auth. Nothing to do here.

---

## 3. Known issue: mcp-server-fetch is broken upstream

Confirmed in `cmd/loadline/README-validation.md`, 2026-08-17: the published
`mcp-server-fetch` 2026.7.10 declares `mcp>=1.1.3` with no upper bound. A
clean `uvx mcp-server-fetch` resolve pulls `mcp` 2.0.0, which renamed
`McpError` to `MCPError`, and the server fails to import:
```
ImportError: cannot import name 'McpError' from 'mcp.shared.exceptions'.
Did you mean: 'MCPError'?
```
This is server rot, exactly the kind methodology 7 keeps in the dataset as
data, not something to route around silently.

Two options, both documented, neither decided here:

**Option A, leave as-is (recommended).** The row publishes `status:
unreachable` with the traceback in `error`. This is an honest, real finding
about the state of a widely-used reference server, and it costs nothing to
publish. `servers.yaml` was deliberately left untouched during validation
for exactly this reason.

**Option B, ratify a version pin.** The harness already supports a
constrained pin via a `with` list on a `pypi` package entry
(`internal/sweep/acquire.go`, `stdioLaunch`, the `pkg.With` loop), and
`uvx --with "mcp<2" mcp-server-fetch==2026.7.10` is confirmed to start and
measure cleanly (1 tool, `fetch`, 238 tokens naive). Taking this option
means editing `servers.yaml`'s fetch package block to declare the pin, and
it is a CORPUS DECISION, not a harness bugfix: it changes what "the fetch
server" means for this dataset (a constrained environment rather than the
published default), so it must be recorded as a changelog event under rule
A.9 in `docs/server-selection.md`, dated, with the prior and new wording and
the reason stated, same as any other Part A edit.

This is Roshan's call. Do not silently pick one before the run; if the run
needs to proceed before he decides, run with Option A (unpinned, honest
unreachable) and revisit the pin afterward.

---

## 4. Pre-run task list: resolve the servers.yaml TODOs

Every remaining "TODO verify at onboarding" in `servers.yaml`, as a concrete
task with its verification step. Some overlap with section 2's credential
tasks; listed again here so this is a complete checklist on its own.

1. **github, token_env.** Verify whether a classic PAT scope-filters
   `tools/list` on the live server, or whether the OAuth flow returns the
   full list. Verification: compare live `tool_count` against the ~113
   documented tools. (Same as section 2.)
2. **github, package.** Pin the exact release (v1.9.0 at time of research)
   and decide binary vs. Docker image distribution. Verification: record
   the chosen version and source in `servers.yaml`'s package block.
3. **fetch, package.** Confirm the PyPI package name and pin a version
   (`mcp-server-fetch` on PyPI). Bound to the section 3 decision: if Option
   B is taken, this is also where the `mcp<2` constraint gets recorded.
4. **kubernetes, auth.token_env.** Not a single env var; document the
   harness's actual convention (kubeconfig path vs. in-cluster service
   account) here and in `internal/sweep/acquire.go` if the harness doesn't
   already handle it. Verification: a `--dry-run` scan resolves the
   kubernetes row without error.
5. **kubernetes, package.** Pin the exact release (v0.0.66 at time of
   research) and image reference. Verification: record in `servers.yaml`.
6. **cloudflare, endpoint.** 16 product-scoped remote servers, no single
   endpoint. Decide which product server(s) this corpus entry actually
   measures, or how multiple are aggregated. Verification: a live
   `tools/list` pass against the chosen endpoint(s), tool count recorded.
7. **cloudflare, tool count.** Not aggregated anywhere upstream.
   Verification: same live `tools/list` pass as above.
8. **slack, token_env / auth mode.** Roshan's stealth-mode-vs-OAuth call
   (section 2). Verification: none, this is a decision, not a fact-check.
9. **slack, last-commit date.** Discrepancy between default-branch last
   commit (2026-05-14) and repo `pushed_at` (2026-07-16). Verification:
   check the actual default branch's commit log directly before trusting
   Gate 4.
10. **linear, auth mechanism.** OAuth 2.1 vs. API key vs. enterprise
    SAML/Okta; `servers.yaml` currently declares `LINEAR_API_KEY`.
    Verification: confirm API key reaches the full `/mcp` endpoint's tool
    list (not `/mcp/readonly`) before locking this in.
11. **figma, everything.** Auth, transport, package, tool count all
    unresearched past the adoption-evidence pass. Verification: per section
    2's three-step figma task above.
12. **context7, everything.** Same as figma. Verification: per section 2's
    context7 task above.
13. **postgres, package.** Confirm PyPI package `postgres-mcp` and pin an
    exact version; confirm pip/uv install path. Verification: record in
    `servers.yaml`.
14. **jaeger, package.** Confirm exact distribution channel and pinned
    version; explicitly select `BACKEND_TYPE=jaeger`. Verification: a live
    run against the local Jaeger container from section 2 returns Jaeger's
    5 core tools without unrelated Tempo/Traceloop tools inflating the
    count (or, if they can't be suppressed, that inflation is called out on
    the published row, per `docs/server-selection.md` B.2 item 12's own note).
15. **fetch pin decision.** Section 3, Option A vs. B. Not a fact to verify,
    a decision to make and, if B, to log as a changelog event.

None of these are guessed in `servers.yaml` today; each is marked with a
TODO comment rather than a placeholder value. Keep it that way. If a TODO
gets resolved, replace the TODO comment with the resolved value AND a
one-line note of how it was verified (URL, date), matching the discipline
already used in `docs/server-selection.md`.

---

## 5. The run

Once the credentials and TODOs above are as resolved as they're going to be
for this run (some, like slack's compliance call, may legitimately still be
open; that's fine, that server sweeps with whatever was decided or gets
skipped via `--skip slack` if undecided):

```
cd ~/loadline   # on the PC, wherever this repo lives there

go build ./...
go vet ./...
go test ./...

# export every credential env var resolved in section 2, e.g.:
export GITHUB_TOKEN=...
export CLOUDFLARE_API_TOKEN=...
export NOTION_TOKEN=...
export SLACK_MCP_XOXC_TOKEN=...       # only if the stealth-mode call was made this way
export LINEAR_API_KEY=...
export STRIPE_API_KEY=...             # sk_test_...
export SENTRY_ACCESS_TOKEN=...
export DATABASE_URI=postgres://postgres:loadline@localhost:5432/loadline
export BACKEND_URL=http://localhost:16686
export KUBECONFIG=~/.kube/loadline-config
export ANTHROPIC_API_KEY=...
export GEMINI_API_KEY=...

# sanity check the corpus resolves without contacting anything first
go run ./cmd/loadline scan --servers servers.yaml --out data/ --dry-run

# the real run
go run ./cmd/loadline scan --servers servers.yaml --out data/ \
  --step-timeout 180s --server-timeout 360s
```

Adjust `--step-timeout` / `--server-timeout` upward if the OAuth-based
servers (github, cloudflare, linear, stripe, sentry) are slow to negotiate,
or if kind/docker containers are still warming up. The validation run used
these same values against filesystem and fetch only.

**Expected duration.** The 2-server validation run completed in a few
seconds once packages were cached. A cold 15-server run, with first-time
`npx`/`uvx` downloads for five npm-packaged and two pypi-packaged servers,
Docker container startup for postgres and the kind cluster, and OAuth token
exchanges for five servers, will run longer: budget 20 to 40 minutes for a
first cold run. A rerun with warm caches and running containers should be
well under 10 minutes.

**What a healthy `data/latest.json` looks like:**
- `schema_version: "0.1"`, `sample: false` (real data, not the site's
  placeholder sample).
- `run.date` matching the run date.
- 15 entries under `servers`, one per id in `servers.yaml`'s `servers:` list
  (github, filesystem, fetch, kubernetes, cloudflare, notion, slack, linear,
  stripe, figma, sentry, jaeger, postgres, context7, playwright).
- Each entry's `status` is one of `ok`, `unreachable`, `auth`,
  `protocol_error`, `timeout`, `partial_surface`, or `schema_invalid`
  (`internal/report/report.go`). Not every row needs to be `ok`; a mix is
  expected and honest, especially given fetch (section 3) and any TODOs
  still open at run time.
- `counts.openai_o200k.available: true` on every reachable row (no key
  needed); `counts.claude.available` and `counts.gemini.available` are
  `true` only if the corresponding key was exported and the call succeeded,
  `false` with a reason otherwise.

**The rule: failures are kept, never hand-fixed to make the run look
clean.** If a server comes back `unreachable`, `auth`, or `partial_surface`,
that is the data point. Do not retry with a different credential to make a
row pass and then discard the failed attempt, do not silently swap in a
different package version to dodge an error, and do not edit the output
JSON by hand. If a genuine mistake in the run setup is found (wrong env var
name, container not started), fix the setup and rerun the whole sweep, not
just the one row, and keep both artifacts if there's any ambiguity about
which run is canonical.

---

## 6. After the run

1. Copy `data/runs/<date>/` (all per-server JSON files) and the new
   `data/latest.json` back from the PC to wherever the canonical repo copy
   lives (dev machine, or push from the PC if it has its own git remote
   access). Do not just copy `latest.json` alone; the per-server run
   artifacts under `data/runs/<date>/` are the reproducibility record.
2. Do NOT overwrite `site/public/data.json` automatically. As of this
   writing that file is placeholder sample data (`"sample": true`). Only
   replace it with real sweep output once Roshan has ratified that
   specific `sample: false` data for display. Copying `latest.json` over
   `site/public/data.json` the moment a run finishes, without that
   ratification step, is not this runbook's call to make.
3. Before any number from this run appears anywhere public (site, a post, a
   README, a claim in conversation), it goes through the `~/PR/CLAIMS.md`
   ledger discipline first (`PRD.md` line 168: "Every public number flows
   through Roshan's `~/PR/CLAIMS.md` ledger discipline"). This is not
   optional and not a formality to skip because the harness already did the
   measuring; the ledger step is what catches things the harness can't,
   like a number being technically correct but misleadingly framed.
