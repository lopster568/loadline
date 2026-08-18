# PC Sweep Runbook: First Full 15-Server Tier 1 Sweep

Status: operator handoff, first full run.

**Dev-machine onboarding pass, 2026-08-17.** The no-external-account subset of
this list (playwright, context7, postgres, jaeger) was worked on the dev
laptop, not the PC, ahead of the full 15-server run. This machine has `npx`
(v11.2.0) and `uvx` (0.9.18) but no `docker`/`podman` on PATH. Result:
- **playwright**: resolved and scanned live (`ok`, 24 tools). See section 2.
- **context7**: resolved and scanned live (`ok`, 2 tools, keyless). See
  section 2; this closes item 12 in section 4.
- **postgres**, **jaeger**: `servers.yaml` package specs resolved and
  verified live (PyPI names, versions, exact CLI invocation), but NOT
  scanned, because this machine has no Docker to stand up the required local
  container. Scanning them requires the PC run. See their entries in section
  2 for what was resolved and what still needs the container.
`data/latest.json` as of this pass is a 4-server coherent run (filesystem,
fetch, context7, playwright), not the full 15; the remaining 11 (including
postgres and jaeger) stay on this runbook's list for the PC.

**Docker Desktop WSL integration pass, 2026-08-18.** Docker Desktop's WSL
integration was enabled on the dev machine, closing the docker/podman gap
noted above without needing the PC. `docker ps` confirmed working (server
28.3.2). This pass completed the three container-backed servers:
- **postgres**: throwaway `postgres:alpine` container, scanned, publishes
  `status: unreachable` -- genuine server rot (unbounded `mcp[cli]>=1.5.0`
  dependency pulls incompatible `mcp` 2.0.0), not an infra failure. See
  section 2 and section 4 item 13.
- **jaeger**: throwaway `jaegertracing/all-in-one` container, scanned,
  `status: ok`, 11 tools. See section 2 and section 4 item 14.
- **kubernetes**: `token_env`/package TODOs resolved (KUBECONFIG env var,
  npm `kubernetes-mcp-server@0.0.66`), scanned against a local `kind`
  cluster, `status: ok`, 20 tools. See section 2 and section 4 items 4-5.

A coherent combined run followed with `--only
filesystem,fetch,playwright,context7,notion,github,stripe,linear,sentry,cloudflare,figma,postgres,jaeger,kubernetes`
(slack excluded, its OAuth app/token still an open operator task per item 8)
with all three containers/cluster up together and `GITHUB_PERSONAL_ACCESS_TOKEN`
from `gh auth token`. `data/latest.json` now holds 14 servers: 11 `ok`, 2
`unreachable` (fetch, postgres, both genuine upstream server rot, not
setup mistakes), 1 `auth` (figma, OAuth-only, no static-token convention
exists to declare). This does not close out the full 15-server run: slack
is still pending its OAuth operator task (item 8), and this pass's `--only`
list did not include it. All throwaway containers and the `kind` cluster
were torn down after the combined run completed.

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
   servers: fetch, postgres, jaeger (`opentelemetry-mcp`, resolved 2026-08-17,
   see section 2).
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

**github**: env var `GITHUB_TOKEN`.
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

**filesystem**: no auth. Already validated (`cmd/loadline/README-validation.md`,
14 tools, `ok`). Nothing to do here.

**fetch**: no auth, but broken upstream. See section 3 below before
touching this row.

**kubernetes**: RESOLVED and scanned live, 2026-08-18. Verified via
https://github.com/containers/kubernetes-mcp-server README (checked live):
kubeconfig resolution is either the `--kubeconfig` CLI flag or, when not
provided, automatic resolution ("in-cluster, default location, etc."). That
automatic path is client-go's standard clientcmd loading rules, which honor
the `KUBECONFIG` environment variable ahead of the default `~/.kube/config`
path -- the same convention every kubectl-family tool uses, not something
specific to this server. `internal/sweep/acquire.go`'s `credential()` helper
only passes one env var through to the launched subprocess, so `KUBECONFIG`
is the convention that fits it cleanly; no `--kubeconfig` flag needed.
`servers.yaml` filled in accordingly: `auth.required: true`, `token_env:
KUBECONFIG`, package npm `kubernetes-mcp-server` version `0.0.66` (verified
via https://registry.npmjs.org/kubernetes-mcp-server, matches the GitHub
release tag), launched via `npx`. This closes items 4 and 5 in section 4.

Local `kind` cluster, no cloud account needed:
```
kind create cluster --name loadline
kind get kubeconfig --name loadline > ~/.kube/loadline-config
export KUBECONFIG=~/.kube/loadline-config
```
Live scan (`npx -y kubernetes-mcp-server@0.0.66`, KUBECONFIG pointed at a
fresh single-node `kind` cluster): `status: ok`, 20 tools, naive 4430 tokens
`o200k_base`, Claude 7552 total / 6734 native tools-param tokens, Gemini
4590 tokens, hygiene grade C (74.17), retrievability top3 1.00 / MRR 0.8325,
`serverInfo.version` matches the pinned `v0.0.66`. Confirms `KUBECONFIG` is
the correct credential-passing convention end to end; nothing further open
here for Tier 1.

**cloudflare**: env var `CLOUDFLARE_API_TOKEN`.
- Create a free Cloudflare account, then an API token (dashboard, My
  Profile, API Tokens), scoped to the products you intend to exercise.
- Known open question (not yet a credential blocker, but a scope-of-run
  decision): this server is 16 separate product-scoped remote servers at
  16 different subdomains, no single endpoint. Decide which product
  server(s) to hit before the run, per the TODO in section 4.

### Productivity / SaaS

**notion**: env var `NOTION_TOKEN`.
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

**slack**: env var `SLACK_MCP_XOXP_TOKEN` (user OAuth token).

**DECISION MADE, 2026-08-18: OAuth, not stealth browser-cookies.** A neutral
public benchmark cannot rest on session-cookie extraction that violates the
platform's terms of service and that third parties cannot reproduce;
governance promises the harness is re-runnable by anyone with their own
credentials, which stealth-mode cookie extraction is not. If OAuth's tool
surface turns out smaller than stealth mode's, the slack row publishes as
`partial_surface` or is excluded outright under the selection rule's
partial-surface gate (same rule that excluded grafana and atlassian,
`docs/server-selection.md` B.2.1). That is the accepted cost of the
principle, not a reason to reconsider it.

Verified live 2026-08-18 via
https://github.com/korotovsky/slack-mcp-server README:
- Four auth modes exist: `SLACK_MCP_XOXC_TOKEN` + `SLACK_MCP_XOXD_TOKEN`
  (paired browser-session "stealth" cookies), `SLACK_MCP_XOXP_TOKEN` (user
  OAuth token, described in the README as "alternative to xoxc/xoxd"), and
  `SLACK_MCP_XOXB_TOKEN` (bot token, "alternative to xoxp/xoxc/xoxd").
- The tool surface is documented to differ by mode: bot tokens (`xoxb`)
  cannot use `search.messages` (search tool unavailable); the unread-messages
  tool falls back to a slower per-channel method on `xoxp` and is unavailable
  on `xoxb`; the `saved_list`/`saved_update`/`saved_clear_completed` tools
  require browser-session tokens (`xoxc`/`xoxd`) and are unavailable on
  `xoxp`/`xoxb`; user search uses a local cache on OAuth tokens versus the
  Slack edge API on browser-session tokens.
- This corpus uses `SLACK_MCP_XOXP_TOKEN` (user OAuth), not the bot token,
  because the bot token has the additional documented loss of
  `search.messages`; the user OAuth token does not carry that specific loss,
  making it the closer of the two OAuth-family modes to the stealth-mode
  surface.

**Operator task:** create a Slack app in a free/personal dev workspace,
install it, generate a user OAuth token (`xoxp-...`), export it as
`SLACK_MCP_XOXP_TOKEN`, then run the mandatory surface-comparison check from
section 2 above: compare the live `tool_count` against the server's
documented full tool list before publishing this row as a ranking rather
than `partial_surface`.

Separately, `docs/server-selection.md` flags the repo's last-commit date
as unresolved (default branch shows 2026-05-14, but `pushed_at` shows
2026-07-16). Re-check the actual default branch's last commit before
trusting Gate 4 (active maintenance) on this server. This is unrelated to
the auth-mode decision above and remains open.

**linear**: env var `LINEAR_API_KEY`.
- Free Linear workspace, then a personal API key from Linear settings.
- REQUIRED VERIFICATION (B.3, resolvable): use the full `mcp.linear.app/mcp`
  endpoint with a write-scoped credential. Do NOT point the harness at
  `mcp.linear.app/mcp/readonly` or a read-only-scoped OAuth grant; both
  truncate the tool surface by design. Confirm the endpoint the harness
  actually calls before the run.
- `servers.yaml` also leaves the auth mechanism itself as a TODO (OAuth 2.1
  vs. API key vs. enterprise SAML/Okta); pick API key (matches the declared
  `LINEAR_API_KEY` env var) unless there's a reason to prefer OAuth.

**stripe**: env var `STRIPE_API_KEY`.
- Free Stripe account, then a TEST-MODE restricted API key (starts
  `sk_test_`). Do not use a live key. Stripe's own docs recommend a
  restricted key to limit what the agent can actually do; the doc notes
  this affects capability, not the 11-tool `tools/list` count, so a
  restricted test key is fine for Tier 1 measurement.

**figma**: everything is an unresolved TODO in `servers.yaml`: auth,
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

**sentry**: env var `SENTRY_ACCESS_TOKEN`.
- Free Sentry developer account, then an access token via OAuth (broad
  scope) or a personal auth token.
- REQUIRED VERIFICATION (B.3): source-confirmed partial-surface risk. 54
  total tools, most gated by `requiredScopes`. Get a BROAD-scope grant, not
  a narrow default one, and check the row's `tool_count` against 54 after
  the run. One tool, `analyze_issue_with_seer`, may additionally be gated
  to a paid Sentry plan; this is unconfirmed in the source doc, so note
  whether it's present or absent in the live run rather than assuming
  either way.

**jaeger**: no credential needed for the Jaeger backend path
(`BACKEND_URL=http://localhost:16686`; a token is only needed for
Traceloop's own backend, which this corpus does not use). Stand up a local
Jaeger instance:
```
docker run --rm -d --name loadline-jaeger \
  -p 16686:16686 -p 4317:4317 -p 4318:4318 \
  jaegertracing/all-in-one:latest
```
**Correction, 2026-08-17**: `BACKEND_URL`/`BACKEND_TYPE` are NOT environment
variables for this server; that was this runbook's original assumption and
it does not match the live package. Verified via
https://pypi.org/pypi/opentelemetry-mcp/json (latest 0.2.2) and the
traceloop/opentelemetry-mcp-server README: the package is `opentelemetry-mcp`
on PyPI, run via `uvx opentelemetry-mcp --backend jaeger --url
http://localhost:16686` (backend selection is a CLI flag pair, not env vars).
`servers.yaml`'s jaeger package block is filled in accordingly (type `pypi`,
name `opentelemetry-mcp`, version `0.2.2`, `args: ["--backend", "jaeger",
"--url", "http://localhost:16686"]`); this closes item 14 in section 4 on the
package-spec side.

**Scanned live, 2026-08-18** (Docker Desktop WSL integration enabled this
run): `docker run -d --name loadline-jaeger -p 127.0.0.1:16686:16686
jaegertracing/all-in-one:latest`, then `uvx opentelemetry-mcp==0.2.2
--backend jaeger --url http://localhost:16686`. Result: `status: ok`, 11
tools, naive 4063 tokens `o200k_base`, Claude 6883 total / 6535 native
tools-param tokens, Gemini 4500 tokens, hygiene grade C (61.11),
retrievability top3 1.00 / MRR 0.9798, `serverInfo.version` `2.14.7`. This
closes the tool-count-inflation question from item 14 below: the live
surface is 11 tools, not the ~5 previously assumed, and includes both
Jaeger-native tools (`get_trace`, `list_services`, `search_traces`,
`find_errors`, `search_spans_tool`) and Traceloop's own LLM-observability
tools (`get_llm_expensive_traces`, `get_llm_model_stats`,
`get_llm_slow_traces`, `get_llm_usage`, `list_llm_models`,
`list_llm_tools_tool`) that are not Jaeger-specific, confirming the corpus
note that non-Jaeger tools inflate the apparent count. The published row
should call this out rather than presenting all 11 as Jaeger capability.
Container torn down after the individual scan; brought back up for the
combined coherent run and torn down again afterward.

### Data

**postgres**: env var `DATABASE_URI`. Local container, no external account:
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

**Package spec resolved, 2026-08-17**: verified via
https://pypi.org/pypi/postgres-mcp/json (latest 0.3.0) and the
crystaldba/postgres-mcp README: `uvx postgres-mcp --access-mode=unrestricted`
(access mode is a CLI flag, `DATABASE_URI` stays an env var).
`servers.yaml`'s postgres package block is filled in accordingly (type
`pypi`, name `postgres-mcp`, version `0.3.0`, `args:
["--access-mode=unrestricted"]`); this closes item 13 in section 4.

**Scanned live, 2026-08-18** (Docker Desktop WSL integration enabled this
run): `docker run -d --name loadline-pg -e POSTGRES_PASSWORD=<random> -p
127.0.0.1:5433:5432 postgres:alpine`, confirmed ready via `pg_isready`, then
`DATABASE_URI=postgresql://postgres:<pw>@127.0.0.1:5433/postgres uvx
postgres-mcp==0.3.0 --access-mode=unrestricted`. Result: **`status:
unreachable`**, not a container/credential problem -- the container was
healthy and reachable throughout. `postgres-mcp` 0.3.0 declares `mcp[cli]
>=1.5.0` with no upper bound
(https://pypi.org/pypi/postgres-mcp/0.3.0/json); a clean `uvx` resolve pulls
`mcp` 2.0.0, whose package layout postgres-mcp's own code doesn't match
(`ModuleNotFoundError: No module named 'mcp.server.fastmcp'`, confirmed by
running the same resolve directly outside the harness). This is the same
class of server rot as the fetch server in section 3 below: an unbounded
dependency pin resolving to a breaking major version. As a diagnostic check
only (not applied to `servers.yaml`), `uvx --with "mcp<2" postgres-mcp==0.3.0
--access-mode=unrestricted --help` was confirmed to start and print its
help text cleanly, mirroring fetch's rejected Option B. Per this runbook's
Option-A precedent for fetch (no pin; the unreachable row is honest data
about the published default, and the row heals itself once postgres-mcp
ships a compatible `mcp` bound), `servers.yaml`'s postgres args are left
unpinned. Row publishes `unreachable`. Container torn down after the
individual scan; brought back up for the combined coherent run (still
`unreachable`, same reason) and torn down again afterward.

**context7**: RESOLVED and scanned live, 2026-08-17. Verdict: keyless. The
README (https://github.com/upstash/context7, checked 2026-08-17) says only
"API Key Recommended ... for higher rate limits," which is ambiguous about
whether `tools/list` itself needs a key. Resolved empirically instead of by
inference: `npx -y @upstash/context7-mcp@4.0.2` with no
`CONTEXT7_API_KEY` set answered `tools/list` with `status: ok`, 2 tools
(`resolve-library-id`, `query-docs`), naive 1052 tokens, `o200k_base`. 2 tools
is the server's full documented surface (no evidence of a larger gated set),
so the API key gates query rate limits, not tool enumeration. `servers.yaml`
filled in: `auth.required: false`, `token_env: CONTEXT7_API_KEY` (present but
optional), package npm `@upstash/context7-mcp` version `4.0.2` (verified via
https://registry.npmjs.org/@upstash/context7-mcp, latest dist-tag). Scanned
into `data/latest.json` alongside filesystem, fetch, and playwright as a
4-server dev-machine run.

### Browser automation

**playwright**: no auth. RESOLVED and scanned live, 2026-08-17. Verified via
https://registry.npmjs.org/@playwright/mcp (dist-tags.latest = `0.0.79`) and
https://github.com/microsoft/playwright-mcp README (stdio default, no
auth/API key). `servers.yaml` package block pinned to version `0.0.79`. Live
scan (`npx -y @playwright/mcp@0.0.79`, no browsers pre-installed): `status:
ok`, 24 tools, naive 4024 tokens `o200k_base`, `serverInfo.version`
`1.63.0-alpha-2026-08-05`. Confirms `tools/list` answers without a browser
download; Tier 1 needs nothing further here.

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

**DECISION MADE, 2026-08-18: no pin (Option A).** The unreachable row
stands as honest data. `servers.yaml`'s fetch package block stays unpinned
by decision, not by omission; a code comment there records this. The
harness continues to check upstream on each monthly run, and the row heals
itself with no `servers.yaml` edit needed once `mcp-server-fetch` ships a
version compatible with `mcp` 2.x. Option B (a version pin, previously
documented below for reference) was considered and rejected: pinning would
change what "the fetch server" means for this dataset (a constrained
environment rather than the published default) for a transient upstream
break that is expected to resolve on its own.

For reference, the rejected Option B: the harness supports a constrained
pin via a `with` list on a `pypi` package entry (`internal/sweep/acquire.go`,
`stdioLaunch`, the `pkg.With` loop), and `uvx --with "mcp<2"
mcp-server-fetch==2026.7.10` was confirmed to start and measure cleanly
(1 tool, `fetch`, 238 tokens naive). Not taken.

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
3. **fetch, package. RESOLVED 2026-08-18.** PyPI package name confirmed
   (`mcp-server-fetch`). No version pin: section 3's decision, ratified
   2026-08-18, is Option A (unpinned). `servers.yaml`'s fetch package block
   carries a comment recording this instead of a version.
4. **kubernetes, auth.token_env. RESOLVED 2026-08-18.** `token_env: KUBECONFIG`.
   client-go's clientcmd honors the `KUBECONFIG` env var for automatic
   kubeconfig resolution when no `--kubeconfig` flag is passed, and
   `internal/sweep/acquire.go`'s `credential()` already handles a plain
   env-var-name/value pair with no code change needed. Verified with a live
   scan against a `kind` cluster, not just `--dry-run`; see section 2's
   kubernetes entry.
5. **kubernetes, package. RESOLVED 2026-08-18.** npm package
   `kubernetes-mcp-server` v0.0.66 (matches the GitHub release tag),
   launched via `npx`, per the README's primary documented method. No
   separate docker image reference was needed or used. Recorded in
   `servers.yaml` and confirmed live: `serverInfo.version` on the wire
   matches the pinned `0.0.66`.
6. **cloudflare, endpoint.** 16 product-scoped remote servers, no single
   endpoint. Decide which product server(s) this corpus entry actually
   measures, or how multiple are aggregated. Verification: a live
   `tools/list` pass against the chosen endpoint(s), tool count recorded.
7. **cloudflare, tool count.** Not aggregated anywhere upstream.
   Verification: same live `tools/list` pass as above.
8. **slack, token_env / auth mode. RESOLVED 2026-08-18.** OAuth
   (`SLACK_MCP_XOXP_TOKEN`), not stealth browser-cookies. See section 2's
   slack entry for the full rationale and the remaining operator task
   (create the OAuth app/token, then run the surface-comparison check).
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
12. **context7, everything. RESOLVED 2026-08-17.** Keyless, npm
    `@upstash/context7-mcp` v4.0.2, stdio (also remote HTTP at
    mcp.context7.com/mcp, not used here), 2 tools live-confirmed. See section
    2's context7 entry for the empirical verification. Scanned into
    `data/latest.json`.
13. **postgres, package. Spec resolved 2026-08-17, SCANNED 2026-08-18.**
    Confirmed PyPI package `postgres-mcp` v0.3.0
    (https://pypi.org/pypi/postgres-mcp/json); install/run via `uvx
    postgres-mcp --access-mode=unrestricted`. Recorded in `servers.yaml`.
    Scanned live against a throwaway `postgres:alpine` container: publishes
    `status: unreachable`. Not a container or credential problem -- the
    server itself fails to import because its unbounded `mcp[cli]>=1.5.0`
    dependency resolves to an incompatible `mcp` 2.0.0. Same server-rot
    pattern as fetch (section 3); no pin applied, by the same Option-A
    reasoning. See section 2's postgres entry for the full finding.
14. **jaeger, package. Spec resolved 2026-08-17, SCANNED 2026-08-18.**
    Confirmed PyPI package `opentelemetry-mcp` v0.2.2
    (https://pypi.org/pypi/opentelemetry-mcp/json), install/run via `uvx
    opentelemetry-mcp --backend jaeger --url http://localhost:16686`
    (correcting this runbook's original `BACKEND_TYPE`/`BACKEND_URL` env-var
    assumption, which does not match the live package; backend selection is
    a CLI flag pair). Recorded in `servers.yaml`. Scanned live against a
    `jaegertracing/all-in-one` container: `status: ok`, 11 tools. The
    tool-count-inflation question is resolved: 11, not ~5, with 5 tools
    Jaeger-native and 6 Traceloop LLM-observability tools that are not
    Jaeger-specific (named in section 2's jaeger entry); the published row
    should call this split out.
15. **fetch pin decision. RESOLVED 2026-08-18.** Section 3, Option A (no
    pin). Monitor upstream each monthly run; the row heals itself once
    `mcp-server-fetch` ships a fix, no `servers.yaml` edit needed.

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
export SLACK_MCP_XOXP_TOKEN=...       # user OAuth token; decision ratified 2026-08-18, see section 2
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
