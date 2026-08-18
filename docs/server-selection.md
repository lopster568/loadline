# Server Selection: Rule v0 and Initial 15-Server List

| Field | Value |
| --- | --- |
| Status | Draft v0, for Roshan's ratification (Phase 0 gate per PRD.md 7.2) |
| Date | 2026-08-17 |
| Governs | Tier 1 static sweep scope (PRD.md 2.3) |
| Changes to Part A | Changelog events, per PRD.md 4 (Governance) |

This document has two parts. Part A is the published selection rule: the mechanical test that decides which MCP servers the standing measurement covers. Part B is the first application of that rule: the initial 15-server candidate list plus 5 reserves, with evidence, presented for ratification per PRD.md 10 (Open Decision 3).

---

## Part A: Server Selection Rule v0

### A.1 Purpose and scope

This rule decides which servers Tier 1 (the monthly static sweep) measures. It does not decide Tier 2 inclusion (the 3-server dynamic tier), which is a separate, smaller, capability-driven choice made after Tier 1 data exists.

The rule exists so a reader can predict inclusion without asking us. A server operator should be able to read this page and know, before submitting, whether their server qualifies.

We are not chasing PolicyLayer's 6,686-server coverage (PRD.md 3). Fifteen is a ceiling by design, not a starting point we intend to grow indefinitely. Coverage is explicitly not a claim we make (PRD.md 9).

### A.2 Mandatory gates (all must pass)

A candidate must clear every gate below. Failing any one gate is disqualifying regardless of popularity. These are binary, not scored, so the rule stays mechanical.

**Gate 1, Protocol compliance.** The server must respond to a standard MCP handshake and `tools/list` (or the current spec's equivalent discovery call) using a protocol revision our harness speaks. The harness commits to speaking at least the three most recent MCP protocol revisions (PRD.md 2.3), including the 2026-07-28 revision that replaced `initialize`/sessions with `server/discover`. A server on an unsupported revision fails this gate until the harness is updated; that is logged as a harness gap, not held against the server's standing.

**Gate 2, Full tool-surface enumerability under a free-tier credential.** We must be able to obtain the server's complete `tools/list` response using a credential we can obtain at no cost (a free tier, a free developer account, or an unauthenticated/public mode). If the tool surface changes shape by permission scope, plan tier, or workspace role, and our free-tier credential cannot reach the maximal surface, the server is refused a ranking under PRD.md 4's partial-surface rule. This is checked, not assumed: we run the actual handshake with the actual free-tier credential before onboarding, and record the result. A server that fails this gate is not excluded from the candidate pool forever; it is marked **blocked, auth-gated** and re-tested each time its vendor changes its free-tier terms.

**Gate 3, Verifiable adoption evidence.** At least one of the following, each with a URL and an observed date:
- Listed as the official server by the product vendor (vendor's own docs, repo under the vendor's GitHub org, or vendor-operated remote endpoint), or
- A install/usage count from a third-party registry (official MCP registry, Smithery, PulseMCP, mcp.so, Glama), or
- GitHub stars, used only as a secondary, corroborating signal, never as the sole basis for inclusion.

Official-vendor status and registry install counts outrank stars because stars are gameable and lag real usage; this ordering is fixed by this rule, not decided per server.

**Gate 4, Active maintenance.** At least one commit, release, or (for remote-hosted servers with no public repo) a vendor changelog entry within the trailing 6 months of the sweep date. Six months is chosen against the PRD's own abandonment baseline: 2026 audits found roughly 52% of MCP projects abandoned and 17.2% of remote servers unreachable (PRD.md 2.3), so a 6-month window is loose enough to not punish stable, low-churn reference implementations, tight enough to exclude the abandoned majority. A server that last shipped 6 to 12 months ago is marked **stale, watch** and stays in the candidate pool one more sweep before removal; past 12 months it is removed.

**Gate 5, No official replacement supersedes it.** If a server is a deprecated or archived predecessor to a still-maintained successor from the same vendor (for example, a reference implementation later superseded by an official first-party server), the successor is measured, not the deprecated one, even if the deprecated one currently has more stars.

### A.3 Category diversity requirement

The 15 slots are allocated across five categories, informed by the servers people actually wire into agents today: dev tools, observability, productivity/SaaS, data, browser automation. No category may claim more than 5 of the 15 slots, and every category must have at least 1 slot filled, gates permitting. If a category cannot produce a single candidate that clears all mandatory gates, that slot rolls to the category with the next-strongest bench (ranked by Gate 3 evidence) rather than being left empty, and the shortfall is noted in the changelog.

### A.4 Ranking within a category (tie-breaking)

Where a category has more gate-clearing candidates than slots, rank by, in order:
1. Official-vendor maintainer status (Gate 3) over community maintainer status.
2. Higher registry install count where comparable counts exist across candidates.
3. Overlap with the six 2026 vendor one-shot studies (Cyclr, Scalekit, getunblocked, OnlyCLI, MindStudio, StackOne), because reusing their subjects makes our numbers comparable to studies people have already seen (per the brief governing this doc).
4. GitHub stars, last and lowest-weighted.

### A.5 Self-submission path

A server maintainer or user may submit a server for consideration by opening an issue against the project repo (path published at first public release) stating: server name, repository or vendor docs URL, and which category it belongs to. We run the candidate through Gates 1 to 5 and publish the result, pass or fail, with the specific gate it failed if it fails. A submission that passes all gates is added at the next monthly sweep if it wins its category's ranking, or otherwise recorded as **qualified, on the bench** and reconsidered every time a category slot opens.

This is also the mechanism a rejected server uses to re-apply once its blocking issue is fixed (for example, a vendor ships a free tier that reaches full tool-surface enumerability).

### A.6 Removal triggers

A server already in the 15 is removed from Tier 1 on any of the following, checked at every monthly sweep:
- **Unreachable across 2 consecutive monthly runs.** One failed run auto-publishes as a data point per PRD.md 2.3 ("unreachable, [date]") and does not trigger removal by itself. A second consecutive failure triggers removal, logged in the changelog with both failure dates.
- **Abandonment**, per Gate 4: no commit, release, or changelog entry for 12 months.
- **Auth-model change that breaks Gate 2**: the vendor moves full tool-surface enumerability behind a paid tier with no free-tier equivalent. The server is marked **blocked, auth-gated** and removed if not restored within one sweep cycle.
- **Deprecation by its own vendor** in favor of a successor server (Gate 5 applied retroactively).

A removed server's historical data stays published (it is not deleted from the record), but it stops appearing in the current-month leaderboard and calculator defaults.

### A.7 What is explicitly not a criterion

Task-completion accuracy, schema-hygiene score, and retrievability score (PRD.md 2.1) do not gate inclusion; they are metrics we publish about included servers, not filters for which servers we include. Conflating "measures well" with "gets measured" would let servers game their way out of coverage by writing bad descriptions, which is the opposite of what section 2.1's Goodhart defense is for.

### A.8 Conflict of interest

Any server built or majority-controlled by Roshan or a party he has a financial or employment relationship with is subject to the recusal policy (PRD.md 4), published separately before the first ranking. Such a server, if otherwise gate-eligible, is either excluded from ranking entirely or ranked with an explicit, permanent conflict-of-interest disclosure attached, whichever the recusal policy specifies. See Part B, HydraDNS note, below. This is Roshan's decision, not a default this rule sets.

### A.9 Changelog discipline

Any edit to Part A (a gate, a threshold, a category boundary, a removal trigger) is a changelog event: dated, with the prior wording and the new wording both visible, and with the reason for the change stated. Silent edits to this rule are exactly the kind of thing PRD.md risk #7 ("you benchmarked a harness, not the protocol") is defending against, so this rule protects itself the same way it protects the measurements.

---

## Part B: Initial 15-Server Candidate List

Research conducted live 2026-08-17 via GitHub API, npm/PyPI download APIs, the official MCP registry API, Smithery's public API, and direct fetches of vendor docs and READMEs. Every claim below carries the URL and the date it was observed. Where a fact could not be independently verified, that is stated explicitly rather than filled in.

### B.1 Landscape cross-check, for comparability

- **Official MCP registry** (registry.modelcontextprotocol.io): confirmed to expose no install-count or popularity field of any kind in its schema, checked 2026-08-17 (`server.schema.json` and a live `/v0/servers` query). It is useful for confirming a server is registered and for version tracking, not for popularity evidence. Ruled out as an adoption signal.
- **Smithery** (registry.smithery.ai, public API queried directly, 2026-08-17): total registry size 8,784 servers. Smithery's `useCount` metric measures calls through its own hosted gateway and skews heavily toward remote/consumer tools (Brave Search 104,186, Gmail 57,738, Google Sheets 56,138) over the local stdio dev-tool servers in our pool. Of our candidates, only Slack (12,110), GitHub (6,059), Notion (3,841), and Linear (3,816) show meaningful Smithery useCount; Grafana, Jaeger, filesystem, fetch, and Playwright returned no clean match. This is a real methodology finding: Smithery useCount and GitHub stars rank servers very differently, and this doc does not treat them as interchangeable.
- **PulseMCP** (pulsemcp.com/servers): a fetched "top by weekly visitors" list names Playwright (5.5M), Filesystem (443k), Fetch (514k), and Grafana (176k) among its highest-traffic pages, which corroborates (does not independently confirm) their inclusion here. The exact visitor figures are single-source and could not be cross-checked against a second endpoint; PulseMCP's own v0beta API has no popularity field and is being sunset (per its own deprecation notice, checked 2026-08-17). Treat the visitor numbers as directional, not as a cited statistic.
- **PolicyLayer** (policylayer.com/token-cost, checked 2026-08-17): 6,686 servers measured, 141,577 tool schemas, median 2,023 tokens/server. No per-server traffic ranking is exposed (`/token-cost/servers` returns 404), so "highest-traffic per-server pages" could not be identified. PolicyLayer's own worked example uses GitHub (86 tools, 14,406 tokens), Linear (66 tools, 7,149 tokens), Supabase, and Filesystem (14 tools, 1,666 tokens) as its comparison set, which is a soft signal that these are the servers PolicyLayer considers recognizable to its own readers.
- **The six 2026 vendor one-shot studies.** Only one held up under verification: **StackOne**, "MCP Token Optimization: 4 Approaches to Fix Context Window Bloat" (stackone.com/blog/mcp-token-optimization/, published 2026-03-31, checked 2026-08-17), which measured GitHub MCP (94 tools, 17,600 tokens for the full schema), Atlassian MCP (Jira and Confluence, roughly 10,000 tokens), and used Cloudflare's 2,500-endpoint Code Mode case (1,170,000 to roughly 1,000 tokens) as a worked example. Cyclr's post on context bloat (checked 2026-08-17) uses only hypothetical, unnamed servers, not a measurement study. Scalekit's, getunblocked's, and MindStudio's blogs were checked in full and contain no server-specific token-cost study. OnlyCLI could not be located at all; onlycli.com/.io/.dev all fail to resolve, and no alternate domain was found. **This doc does not claim comparability to five of the six studies named in the brief.** Only StackOne's numbers (GitHub, Atlassian, Cloudflare) are cited as a real point of comparison; the other five names should be re-verified against primary sources before appearing in any public-facing text, per the evidence-debt discipline in PRD.md 5.2.

### B.2 The 15-server list

Categories: dev tools 5, productivity/SaaS 4, observability 2, data 2, browser automation 2. Kubernetes and Cloudflare are classified under dev tools (infrastructure and platform tooling) rather than productivity/SaaS, which is what keeps productivity/SaaS from running over the 5-slot category cap in rule A.3.

**Ratified 2026-08-17.** Grafana and Atlassian/Jira are excluded under the partial-surface rule (Gate 2); see B.2.1. The top 2 reserves from B.5, Figma and Context7, are promoted to backfill the vacated slots, classified under productivity/SaaS and data respectively to keep every category at or under the 5-slot cap. Observability drops from 3 to 2 slots and data rises from 1 to 2, both still within the 1-to-5 range rule A.3 requires.

**Amended 2026-08-18.** Figma is excluded under the partial-surface rule (Gate 2); see B.2.1. Reserve #3 from B.5, Chrome DevTools MCP, is promoted to backfill the vacated slot, classified under browser automation (its natural category per B.5, with room under the 5-slot cap, so no reclassification judgment call was needed the way Context7's was). Productivity/SaaS drops from 5 to 4 slots and browser automation rises from 1 to 2, both still within the 1-to-5 range rule A.3 requires. All 15 items below are renumbered accordingly.

#### Dev tools

**1. GitHub MCP server**
- Maintainer: official, GitHub itself. Repo: github.com/github/github-mcp-server.
- Adoption: 32,298 GitHub stars (github.com/github/github-mcp-server, checked 2026-08-17). Not cleanly listed under its canonical name in the official MCP registry search (returned unrelated third-party packages instead, checked 2026-08-17).
- Last commit: 2026-08-14 (commit 0ea1f77, github.com/github/github-mcp-server/commits/main). Latest release v1.9.0, published 2026-08-10 (github.com/github/github-mcp-server/releases).
- Auth and free-tier surface: **confirmed partial-surface risk.** Per the maintainer's own docs (github.com/github/github-mcp-server/blob/main/docs/scope-filtering.md, checked 2026-08-17): "Tools that require scopes your token doesn't have are automatically hidden" for classic PATs. The server reads the `X-OAuth-Scopes` header at startup and filters `tools/list` accordingly. The same docs state OAuth tokens behave differently: all tools are listed, with scopes requested on demand. This is resolvable, in principle, by onboarding via the OAuth flow rather than a scope-limited classic PAT, but that must be verified against the live server before onboarding, not assumed.
- Transport: stdio (binary/Docker) and remote streamable HTTP at api.githubcopilot.com/mcp/.
- Tool count: roughly 113 individually named tools across 21 documented toolsets (Context, Repositories, Issues, Pull Requests, Actions, Code Security, Copilot, and others), counted from the README bullet list; the actual count visible to any one credential is lower per the scope filtering above.

**2. Filesystem MCP server (reference implementation)**
- Maintainer: official Anthropic/MCP reference implementation, still active (confirmed present, not archived, in the live modelcontextprotocol/servers repo, checked 2026-08-17).
- Adoption: parent monorepo 89,622 stars (github.com/modelcontextprotocol/servers, checked 2026-08-17); `@modelcontextprotocol/server-filesystem` npm downloads 1,908,093 in the trailing month (api.npmjs.org, window 2026-07-17 to 2026-08-15), the single highest adoption figure of any candidate in this list.
- Last commit to `src/filesystem`: 2026-07-29. Latest monorepo release `2026.7.10`, published 2026-07-10 (github.com/modelcontextprotocol/servers/releases).
- Auth and free-tier surface: none required. Access is controlled by a directory allowlist passed at launch, not by any credential, so the full tool list is always visible. No partial-surface risk.
- Transport: stdio (Docker/NPX deployment shown in README; no SSE/HTTP explicitly documented).
- Tool count: 13 (`read_text_file`, `write_file`, `edit_file`, `list_directory`, `search_files`, `directory_tree`, and others), per the raw README.

**3. Fetch MCP server (reference implementation)**
- Maintainer: official Anthropic/MCP reference implementation, still active (same live repo as filesystem).
- Adoption: same 89,622-star parent repo; `mcp-server-fetch` PyPI downloads 1,499,381 in the trailing month (pypistats.org, checked 2026-08-17), second-highest of any candidate.
- Last commit to `src/fetch`: 2026-07-29 (dependency bump).
- Auth and free-tier surface: none required, stateless URL fetcher. No partial-surface risk. README does flag a security caveat (it can reach local/internal IPs, an SSRF-style concern) that belongs in methodology notes, not in the auth-risk column.
- Transport: stdio (pip/uv deployment; no SSE/HTTP explicitly documented).
- Tool count: 1 (`fetch`).

**4. Kubernetes MCP server (containers/kubernetes-mcp-server)**
- Maintainer: community/vendor. The `containers` GitHub org is Red Hat's umbrella for container tooling (podman, buildah, skopeo); this is not a CNCF or Kubernetes-SIG deliverable, and no such official server exists (verified by enumerating the `kubernetes` GitHub org, no MCP repo present, checked 2026-08-17).
- Adoption: 1,973 stars (github.com/containers/kubernetes-mcp-server, checked 2026-08-17), listed in the official MCP registry as `io.github.containers/kubernetes-mcp-server` (registry.modelcontextprotocol.io, checked 2026-08-17), which no competing Kubernetes server was. A community alternative, Flux159/mcp-server-kubernetes, has fewer stars (1,566) but higher npm downloads (73,680/month vs this server's 48,892/month) and is not in the official registry; noted as an alternate, not selected as primary, on the strength of registry presence and vendor backing.
- Last commit: 2026-08-17 (hours before observation). Latest release v0.0.66, 2026-07-31.
- Auth and free-tier surface: kubeconfig file or in-cluster service account. `--read-only` and `--disable-destructive` restrict what actions succeed, not which tools are enumerable. No partial-surface risk.
- Transport: stdio and streamable HTTP (Docker/OCI on port 8080).
- Tool count: 70+ across 8 optional toolsets (Core ~25 default, Config, Helm, Tekton, Kiali, KubeVirt, NetObserv, KCP); non-default toolsets need an explicit enable flag but are not paywalled.

**5. Cloudflare MCP server (cloudflare/mcp-server-cloudflare)**
- Maintainer: official, Cloudflare. Cloudflare runs two distinct official offerings, both live: this one (16 separate product-scoped remote servers, e.g. Workers Bindings, Observability, Radar, each at its own `*.mcp.cloudflare.com` subdomain), and a separate "Code Mode" server (cloudflare/mcp) that collapses 2,500+ API endpoints into 2 generic tools (`search`, `execute`) at roughly 1,000 tokens. This doc measures the domain-specific server as the primary candidate because it is the older, more-established, higher-star repo and what "the Cloudflare MCP server" most often refers to; the Code Mode server is a natural Tier 2/v2-season subject in its own right (PRD.md 7.3 already plans a progressive-disclosure referee season comparing Code Mode against Tool Search), not a second Tier 1 slot.
- Adoption: 4,082 stars for mcp-server-cloudflare vs 737 for the Code Mode repo (both github.com/cloudflare, checked 2026-08-17).
- Last commit: 2026-08-11 (mcp-server-cloudflare); the Code Mode repo last committed 2026-08-07.
- Auth and free-tier surface: OAuth via Cloudflare account, or API tokens (user- or account-scoped) as bearer tokens. No plan-tier gating language found on the tool-surface itself; because these are product-scoped servers, an account that does not use a given Cloudflare product will presumably return empty results from that product's tools even though the tool schemas remain listed. This is a data-availability risk, not a tool-enumeration risk, and does not trigger the partial-surface flag.
- Transport: streamable HTTP at `/mcp` (recommended), legacy `/sse` retained as an alias.
- Tool count: not aggregated into one published number across the 16 product-scoped servers; each has its own tool set. Flagged as not fully verified, needs a live `tools/list` pass at onboarding.

#### Productivity / SaaS

**6. Notion MCP server**
- Maintainer: official, Notion (makenotion). Repo: github.com/makenotion/notion-mcp-server.
- Adoption: 4,592 stars, 624 forks (checked 2026-08-17); npm `@notionhq/notion-mcp-server` 753,273 downloads in the trailing month (api.npmjs.org, checked 2026-08-17).
- Last commit: 2026-07-25 (commit 1d38420, github.com/makenotion/notion-mcp-server/commits).
- Auth and free-tier surface: an internal-integration API token (`NOTION_TOKEN`), or a bearer header for the HTTP transport. Workspace admins control which pages/databases an integration can see via Settings, Connections (developers.notion.com/docs/mcp, checked 2026-08-17), but this scopes *data* visibility, not the *tool list* itself; the full 22-tool schema is returned regardless. Not flagged for partial-surface risk, though worth a live check at onboarding since Notion has not published an explicit statement that the tool list is plan-tier-invariant.
- Transport: stdio (default) and streamable HTTP with an optional bearer token.
- Tool count: 22 (v2.0.0, up from 19 in v1.x).

**7. Slack MCP server**
- Maintainer: **no official first-party server exists.** Checked api.slack.com/docs/mcp (redirects to a 404), docs.slack.dev/mcp (no MCP mention), and slack.com/help/articles/mcp (404), all 2026-08-17. The best-adopted community option is korotovsky/slack-mcp-server.
- Adoption: 1,783 stars, 356 forks (checked 2026-08-17); npm `slack-mcp-server` 108,697 downloads in the trailing month.
- Last commit: default branch last commit observed 2026-05-14, though the repo's overall `pushed_at` shows 2026-07-16, so this needs re-verification against the actual default branch before the first sweep; flagged as not fully resolved.
- Auth and free-tier surface: **confirmed compliance risk, distinct from an enumeration risk.** The server's headline feature is a "stealth mode" using browser-session cookies (`xoxc-`/`xoxd-`) that bypasses Slack's OAuth scope system entirely, requiring no app install or granted scopes, which the README does not claim is Slack-ToS-compliant. The alternative is standard OAuth with bot/user scopes, which does scope-gate the 18-tool list in the ordinary way. Because this is a community server built around bypassing the vendor's own permission model, this is a decision for Roshan before onboarding: use scope-limited OAuth (partial surface, fails Gate 2 outright) or stealth mode (fuller surface, but a ToS question this doc does not resolve).
- Transport: stdio, SSE, and HTTP.
- Tool count: 18.

**8. Linear MCP server**
- Maintainer: official, Linear. Remote-hosted only, no public repository: mcp.linear.app/mcp (primary), mcp.linear.app/mcp/readonly (read-only variant), deprecated SSE fallback at mcp.linear.app/sse. Source: linear.app/docs/mcp, checked 2026-08-17.
- Adoption: no GitHub stars possible (no public repo); no npm package found (`@linear/mcp` 404s on the npm registry, checked 2026-08-17); Smithery shows useCount 3,816 for a "linear" listing (registry.smithery.ai, checked 2026-08-17). Maintenance evidence instead comes from Linear's public changelog (linear.app/changelog, checked 2026-08-17), which shows MCP-specific entries on 2026-08-13 (Okta enterprise-managed auth), 2026-07-30 (Zapier added to the MCP connector directory), and several more through June 2026.
- Last commit: not applicable, no repo. Changelog cadence above stands in as the maintenance signal.
- Auth and free-tier surface: **confirmed partial-surface risk, resolvable.** OAuth 2.1, Linear API keys, or enterprise SAML/Okta. The dedicated `/mcp/readonly` endpoint, or requesting only the `read` OAuth scope, returns a reduced tool surface by design. This is resolvable for our purposes by deliberately using the full `/mcp` endpoint with a write-scoped credential rather than the readonly variant, but that choice has to be made explicitly at onboarding, not left to a default.
- Transport: streamable HTTP (primary); deprecated SSE fallback.
- Tool count: not published as an exact figure; docs describe a growing set covering issues, projects, documents, and comments. Flagged as not fully verified.

**9. Stripe MCP server**
- Maintainer: official, Stripe. Repo renamed from stripe/agent-toolkit to github.com/stripe/ai.
- Adoption: 1,748 stars, 324 forks (checked 2026-08-17); npm `@stripe/agent-toolkit` 93,164 downloads in the trailing month, though the package's latest published version (0.9.0) is dated 2026-02-12, six months stale relative to the repo, consistent with Stripe shifting primary distribution to the remote server.
- Last commit: 2026-08-15 (commit f4abb44), two days before observation.
- Auth and free-tier surface: primary path is OAuth via the remote server at mcp.stripe.com, admin-enabled per workspace; the alternative is a restricted API key as a bearer token. Stripe's own docs recommend restricted keys to limit "exactly the functionality" an agent can reach. This affects what the generic `stripe_api_read`/`stripe_api_write` proxy tools can actually do (roughly 90 underlying API operations), not how many named tools appear in `tools/list` (11 tools, stable regardless of key scope). Because Tier 1 measures schema tokens for the 11 listed tools, not the ~90 underlying operations they proxy, this does not block Gate 2; it is a capability note relevant to Tier 2, not a Tier 1 enumeration blocker.
- Transport: remote HTTP/JSON-RPC at mcp.stripe.com. No stdio path documented on the current docs page.
- Tool count: 11 named tools (`stripe_api_read`, `stripe_api_write`, `create_refund`, `search_stripe_documentation`, and others) fronting roughly 90 underlying API operations.

#### Observability

**10. Sentry MCP server**
- Maintainer: official, Sentry (getsentry). Repo: github.com/getsentry/sentry-mcp.
- Adoption: 819 stars (checked 2026-08-17); npm `@sentry/mcp-server` 436,434 downloads in the trailing month; listed in the official MCP registry as `io.github.getsentry/sentry-mcp` v0.25.0.
- Last commit: 2026-08-16, the day before observation. Latest tagged release 0.37.0 is from 2026-07-02; trunk is well ahead of the last formal tag, which is a maintenance-cadence quirk worth noting rather than a red flag.
- Auth and free-tier surface: **confirmed partial-surface risk, source-verified.** Pulled directly from `packages/mcp-core/src/toolDefinitions.json` (github.com/getsentry/sentry-mcp, checked 2026-08-17): 54 total tools, the large majority declaring explicit `requiredScopes` (combinations of org/project/team/event read and write). Sentry's own docs at mcp.sentry.dev state that "when scoped, tools automatically default to the constrained org/project and unnecessary discovery tools are hidden." One tool, `analyze_issue_with_seer`, invokes an AI feature that may additionally be gated to a paid Sentry plan; this could not be confirmed and is flagged as unverified rather than asserted. Resolvable in principle with a broad-scope OAuth grant or access token, which should be obtainable on Sentry's free/developer tier, but that needs verification against the live server before onboarding.
- License note: GitHub reports the repo's license as "Other"/`NOASSERTION` (a non-standard `LICENSE.md` file GitHub could not classify); the actual terms were not verified in this pass and should be checked before any redistribution-adjacent use of the harness output.
- Transport: remote MCP with OAuth at mcp.sentry.dev; local stdio via `SENTRY_ACCESS_TOKEN`, documented by Sentry as still work-in-progress for self-hosted installs.
- Tool count: 54.

**11. Jaeger, via traceloop/opentelemetry-mcp-server (no dedicated official server exists)**
- **No Jaeger-project or CNCF-official MCP server exists.** Confirmed by enumerating all 36 repositories in the `jaegertracing` GitHub org, checked 2026-08-17: no MCP repo present. The official MCP registry's only Jaeger-named entry, `io.github.mshegolev/jaeger-mcp`, has 1 star and is too new/small to be a credible primary candidate. A dedicated community server, serkan-ozal/jaeger-mcp-server (18 stars), last committed 2025-05-13, more than a year stale, and fails the Gate 4 active-maintenance threshold.
- Maintainer of the candidate actually proposed here: community/vendor. traceloop/opentelemetry-mcp-server is built by Traceloop, an LLM-observability company, and is not Jaeger-specific: it is a unified server across Jaeger, Tempo, and Traceloop's own backend, selected by a `BACKEND_TYPE` config flag.
- Adoption: 198 stars (github.com/traceloop/opentelemetry-mcp-server, checked 2026-08-17). This is real, verifiable evidence and clears Gate 3's minimum bar, but it is thin, and this doc does not overstate it.
- Last commit: 2026-06-21, which clears the Gate 4 six-month active-maintenance window. Latest tagged release 0.2.2 is from 2026-02-08 and lags trunk, same pattern as Sentry above.
- Auth and free-tier surface: no credential required for the Jaeger backend specifically (`BACKEND_URL=http://localhost:16686`); a token is only needed for Traceloop's own backend option, which we would not use. No partial-surface risk for the Jaeger use case.
- Transport: stdio and HTTP/SSE.
- Tool count: 10 total (`search_traces`, `search_spans`, `get_trace`, `list_services`, `find_errors`, plus several Traceloop-specific LLM-usage tools not relevant to a pure-Jaeger deployment, which would inflate this server's apparent tool count relative to a Jaeger-only server and should be called out on the published row).
- **This category is an honest gap, not a clean win.** Roshan has direct upstream history with Jaeger (PRD context, and per the estate's own record of 7 merged Jaeger PRs), which is exactly why this doc is not dressing this up: there is no server here that would clear our gates on adoption strength alone. traceloop/opentelemetry-mcp-server is the least-bad pick because it is active, small, and needs no auth for the Jaeger path, not because it is a strong candidate on its own merits. Ratification should treat this slot as provisional and revisit it if a better-adopted, Jaeger-specific server appears via the self-submission path (rule A.5).

#### Data

**12. Postgres MCP server, via crystaldba/postgres-mcp ("Postgres MCP Pro")**
- **The original reference implementation is archived and excluded.** `modelcontextprotocol/servers`'s postgres server moved to `modelcontextprotocol/servers-archived` (confirmed: `src/postgres` 404s on the live repo, checked 2026-08-17), frozen since 2025-05-28, carrying an explicit "NO SECURITY GUARANTEES ARE PROVIDED FOR THESE ARCHIVED SERVERS" banner. It fails Gate 4 (last activity well over 12 months before this sweep) and is not eligible regardless of its residual download volume; see B.4 for the full accounting of why raw downloads on an archived package would otherwise be misleading.
- Maintainer of the candidate actually proposed here: community (Crystal DBA / Johann Schleier-Smith). Repo: github.com/crystaldba/postgres-mcp.
- Adoption: 3,196 stars (checked 2026-08-17); PyPI `postgres-mcp` 619,780 downloads in the trailing month (pypistats.org, checked 2026-08-17).
- Last commit: 2026-08-16 (merged PR #201). Latest tagged release is v0.3.0 from 2025-05-16, over a year stale, but trunk activity (commits, open PRs) is current within 24 to 48 hours of observation; this is the same tags-lag-trunk pattern seen on Sentry and the Jaeger candidate, and this doc uses commit history, not release tags, to judge Gate 4.
- Auth and free-tier surface: a Postgres connection URI. Tool surface changes by an operator-set launch flag ("Restricted Mode" vs "Unrestricted Mode"), not by the credential's own grants, so this is resolvable cleanly: we launch in Unrestricted Mode for measurement and the full 9-tool schema is returned. No partial-surface risk.
- Transport: stdio and SSE.
- Tool count: 9 (`list_schemas`, `execute_sql`, `explain_query`, `analyze_workload_indexes`, and others).

**13. Context7 MCP server**
- Promoted from the reserve bench (B.5) on 2026-08-17, backfilling the observability slot vacated by Grafana's exclusion under the partial-surface rule (see B.2.1). Classified under data (documentation/knowledge retrieval) rather than observability or dev tools, which keeps dev tools at its 5-slot cap; this is a judgment call in the same spirit as the Kubernetes/Cloudflare classification note in B.2, not a strong categorical fit, and should be revisited if a cleaner category emerges at onboarding.
- Maintainer: official, Upstash. Repo: github.com/upstash/context7.
- Adoption: 60,868 stars, pushed 2026-08-17, checked same day (per B.5); independently strong on both Smithery and PulseMCP's traffic list, the single strongest multi-source adoption signal of any server surfaced in the B.5 research pass.
- Last commit: see adoption line above (pushed 2026-08-17).
- Auth and free-tier surface: not researched in this pass. # TODO verify at onboarding
- Transport: not researched in this pass. # TODO verify at onboarding
- Tool count: not researched in this pass. # TODO verify at onboarding

#### Browser automation

**14. Playwright MCP server**
- Maintainer: official, Microsoft. Repo: github.com/microsoft/playwright-mcp.
- Adoption: 36,197 stars (checked 2026-08-17); npm `@playwright/mcp` 25,444,935 downloads in the trailing month, by far the highest raw download figure of any candidate; listed in the official MCP registry as `io.github.microsoft/playwright-mcp` v0.0.79.
- Last commit: 2026-08-07 (commit 7e0457a). Latest release v0.0.79, 2026-08-06.
- Auth and free-tier surface: none required to run the server. No account or token gates any tool. No partial-surface risk. Structural note: this repo's `src/` directory now only contains a redirect notice, the implementation moved into the main microsoft/playwright monorepo (`packages/playwright-core/src/tools/mcp`), so this repo functions as the packaging/release surface; this should be documented in the harness's provenance record so a future contributor does not mistake it for a dead repo.
- Transport: stdio (default), SSE, streamable HTTP.
- Tool count: 50+ across roughly 10 categories (core automation, tabs, network, storage, DevTools, coordinate/vision, PDF, test assertions).

**15. Chrome DevTools MCP server**
- Promoted from the reserve bench (B.5) on 2026-08-18, backfilling the 15th slot vacated by Figma's exclusion under the partial-surface rule (see B.2.1). Unlike Context7's backfill of the observability vacancy into data, this one needs no reclassification judgment call: browser automation is this server's natural category per B.5 (item 3), and it had room under the 5-slot cap (1 of 5 filled by Playwright), so the vacated slot count is restored to 15 without borrowing across categories.
- Maintainer: official, Google (ChromeDevTools org, the Chrome DevTools team). Repo: github.com/ChromeDevTools/chrome-devtools-mcp.
- Adoption: 49,349 stars (github.com/ChromeDevTools/chrome-devtools-mcp, checked live 2026-08-18 via `gh repo view`), up from 49,287 at the B.5 research pass a day earlier; higher raw star count than Playwright itself (36,197), the server already in this category.
- Last commit: 2026-08-17 (checked live 2026-08-18 via `gh api repos/ChromeDevTools/chrome-devtools-mcp/commits`). npm package `chrome-devtools-mcp` latest dist-tag `1.7.0`, published 2026-08-10 (registry.npmjs.org, checked live 2026-08-18).
- Auth and free-tier surface: **none required, no partial-surface risk of the credential kind.** The server automates a locally-launched Chrome/Chromium instance via CDP; no account, API key, or OAuth grant of any kind gates `tools/list` or tool execution. Verified live 2026-08-18 with a raw JSON-RPC probe (`npx -y chrome-devtools-mcp@1.7.0`, no credential set): `tools/list` answers cleanly. Several tool categories are opt-in behind free CLI flags rather than a credential (`--experimentalVision`, `--memoryDebugging`, `--categoryExtensions`, `--categoryExperimentalThirdParty`, `--categoryPwa`, `--categoryExperimentalWebmcp`, `--experimentalScreencast`, the last needing only a local `ffmpeg` binary); this is the same class of thing as postgres's `--access-mode=unrestricted` launch flag, an operator-set launch choice, not a permission-scope, plan-tier, or workspace-role gate, so it does not trigger Gate 2.
- Tool count: **52, live-verified 2026-08-18** with every category/experimental flag above enabled (`click, click_at, close_heapsnapshot, close_page, compare_heapsnapshots, drag, emulate, evaluate_script, execute_3p_developer_tool, execute_webmcp_tool, fill, fill_form, get_console_message, get_heapsnapshot_class_nodes, get_heapsnapshot_details, get_heapsnapshot_dominators, get_heapsnapshot_duplicate_strings, get_heapsnapshot_edges, get_heapsnapshot_object_details, get_heapsnapshot_retainers, get_heapsnapshot_retaining_paths, get_heapsnapshot_summary, get_network_request, handle_dialog, hover, install_extension, lighthouse_audit, list_3p_developer_tools, list_console_messages, list_extensions, list_network_requests, list_pages, list_webmcp_tools, navigate_page, new_page, performance_analyze_insight, performance_start_trace, performance_stop_trace, press_key, reload_extension, resize_page, screencast_start, screencast_stop, select_page, take_heapsnapshot, take_screenshot, take_snapshot, trigger_extension_action, type_text, uninstall_extension, upload_file, wait_for`). The README at HEAD documents 57 tools across 11 categories, including a Progressive Web Apps category (4 tools) and a 13th Memory tool, `query_heapsnapshot_objects`; none of those 5 appeared live even with `--categoryPwa` and `--memoryDebugging` both set. The CHANGELOG for the pinned `1.7.0` release does not mention shipping either, so this doc's working conclusion is that the HEAD README documents unreleased-at-1.7.0 functionality, not a credential or plan-tier gate; 52 is the empirically verified figure this corpus records, not the docs figure, per this doc's discipline of preferring live verification over documentation where they disagree.
- Transport: stdio (npx, default; documented alternates for connecting to an already-running Chrome via `--browser-url`/`--ws-endpoint` were not exercised in this pass).

### B.2.1 Excluded under the partial-surface rule

Decision ratified 2026-08-17, applying rule A.2 Gate 2 and the partial-surface refusal commitment in PRD.md section 4. Both servers cleared every other gate but fail Gate 2: their full tool surfaces sit behind a paid product tier, not just a permission scope, so no free-tier credential can reach them. Per the three options B.3 originally raised, the decision is (c): exclude outright, rather than publish a "measured subset" caveat (a) or provision paid test access outside the $0 Tier 1 budget (b). The two vacated slots, one productivity/SaaS and one observability, are backfilled from the reserve bench (B.5) into B.2 above. The research below is retained as the record supporting exclusion, in keeping with this document's own discipline of keeping a server's data on file rather than deleting it (rule A.6).

**Grafana MCP server. Excluded: full tool surface requires Grafana Cloud and paid Enterprise/Assistant plugins, unreachable on a free-tier self-hosted credential.**
- Maintainer: official, Grafana Labs. Repo: github.com/grafana/mcp-grafana.
- Adoption: 3,361 stars (checked 2026-08-17); listed in the official MCP registry as `io.github.grafana/mcp-grafana` v1.1.0, with both a Docker path and a hosted endpoint at mcp.grafana.com/mcp.
- Last commit: 2026-08-12 (commit 85acacb). Latest release v1.1.0, 2026-08-10.
- Auth and free-tier surface: confirmed hard partial-surface risk, source-verified. Pulled directly from the README table (github.com/grafana/mcp-grafana/blob/main/README.md, checked 2026-08-17): Agent Observability tools are "disabled by default and work only in Grafana Cloud"; Assistant tools require the paid `grafana-assistant-app` plugin; Snowflake datasource tools require the Grafana Enterprise plugin; several other toolsets (admin, ClickHouse, CloudWatch, Elasticsearch/OpenSearch, Graphite, Quickwit) are disabled by default and need explicit `--enabled-tools` flags; and every tool additionally carries a per-tool RBAC scope requirement. A self-hosted OSS Grafana instance with a free-tier service account structurally cannot reach the Cloud-only and paid-plugin tools.
- Transport: stdio (default), SSE, and streamable HTTP, with TLS support.
- Tool count: roughly 103 tool rows in the README table (README prose rounds this down to "70+").

**Atlassian MCP server (Jira/Confluence). Excluded: Jira Service Management and Bitbucket Cloud tools require org-admin-provisioned licensed access, and Teamwork Graph is a paid feature, unreachable on a free-tier credential.**
- Maintainer: official, Atlassian ("Atlassian Rovo MCP Server"). Repo github.com/atlassian/atlassian-mcp-server is documentation/config only; the server itself runs remotely at mcp.atlassian.com. A community alternative, sooperset/mcp-atlassian, has more stars (5,749 vs 967) but loses the tie-break to the official server per rule A.4's ordering (official-vendor status before star count).
- Adoption: 967 stars, 121 forks for the official repo (checked 2026-08-17).
- Last commit: 2026-07-27 (commit 94a3043).
- Auth and free-tier surface: confirmed hard partial-surface risk. Per support.atlassian.com/atlassian-rovo-mcp-server/docs/supported-tools/ (checked 2026-08-17): Jira Service Management tools require an org-admin-provisioned API token and a JSM license; Bitbucket Cloud tools require a linked Bitbucket workspace and org-admin tokens; "Teamwork Graph" tools are Beta and will become paid features billed in Rovo credits. A non-admin account without JSM or Bitbucket licenses structurally cannot see the full tool list, independent of any permission we could grant ourselves.
- Transport: remote streamable HTTP at `/mcp` (recommended), deprecated SSE at `/sse`.
- Tool count: 59+ (2 common, 15 Jira, 10 Confluence, 5 JSM, 14 Bitbucket, 4 Teamwork Graph, 9 Compass), itself gated by the licensing above, so an unprivileged account sees fewer.

Either may re-enter consideration via the self-submission path (rule A.5) if its vendor ships a free tier that reaches full tool-surface enumerability, per Gate 2's own re-test provision for servers marked blocked, auth-gated.

**Figma MCP server. Excluded 2026-08-18: the only credential this server accepts is an OAuth 2.1 browser sign-in, which no free-tier static credential can stand in for.**
- Maintainer, adoption, and last-commit evidence: unchanged from B.2 item 10's prior text (retained here per rule A.6's discipline of keeping a server's evidence on file rather than deleting it): dual-tracked, community GLips/Figma-Context-MCP (15,670 stars, 1,242 forks, last push 2026-08-07) and official figma/mcp-server-guide (1,896 stars, last push 2026-08-14), both checked 2026-08-17.
- Auth and free-tier surface: confirmed hard partial-surface risk, resolved live 2026-08-18 (`servers.yaml`'s figma entry carries the full citation trail): the primary candidate resolved to Figma's official remote-hosted server at `mcp.figma.com/mcp`. Per developers.figma.com/docs/figma-mcp-server/ and help.figma.com/hc/en-us/articles/32132100833559 (both checked 2026-08-18), the remote server requires an interactive OAuth 2.1 browser sign-in on first connect; there is no static personal-access-token or API-key convention this harness's remote dialer can pass as a bearer header. A community forum thread requests PAT-based auth support, but it is not shipped as of 2026-08-18. A keyless `tools/list` probe against `https://mcp.figma.com/mcp` returned HTTP 401, confirming the endpoint is live and auth is genuinely required, with the OAuth-browser-flow gap as the remaining, structural blocker.
- This is a different flavor of Gate 2 failure than Grafana and Atlassian/Jira above: those two have a free-tier *credential* that exists but returns a truncated surface (paid plugins, licensed toolsets); Figma has no acquirable free-tier credential at all for this harness, because the only auth path is a browser-interactive flow with no scriptable, reproducible token this harness (or a third party re-running it) can obtain non-interactively. Both failure modes fail the same Gate 2 test ("obtain the server's complete `tools/list` response using a credential we can obtain at no cost... checked, not assumed"), so both are excluded under the same rule.
- Decision made by the session on 2026-08-18, applying rule A.2 Gate 2 mechanically per the operator's standing instruction to run this rule without per-server escalation; published here for Roshan's review, on the same ratification footing as the 2026-08-17 grafana/atlassian exclusions and reserve promotions above.
- Re-eligible via the self-submission path (rule A.5) the day the harness gains OAuth 2.1 support, or the day Figma ships a static-token convention for the remote server's `tools/list`, whichever comes first; neither is true as of 2026-08-18.
- Transport: remote streamable HTTP at `/mcp`.
- Tool count: never measured; OAuth-only auth means this harness could not complete a keyless or single-token probe against the tool surface itself, only confirm the endpoint is live and gated.

### B.3 Auth-enumeration risk summary

Four of the fifteen carry a flag that must be resolved or explicitly accepted before onboarding, per Gate 2 and the PRD's partial-surface refusal rule. Three more, Grafana, Atlassian/Jira, and Figma, carried a Gate 2 problem severe enough to be excluded outright rather than carried into the fifteen with a flag; see B.2.1 for the ratified rationale. Grafana and Atlassian/Jira fail on a truncated-surface-behind-a-paid-tier basis (excluded 2026-08-17); Figma fails on a no-acquirable-free-credential basis, OAuth-2.1-browser-flow-only with no static-token convention (excluded 2026-08-18).

| Server | Risk type | Resolvable on a free/no-cost credential? |
| --- | --- | --- |
| GitHub | Classic-PAT scope filtering hides tools | Likely, via OAuth flow instead of a classic PAT; needs live verification |
| Linear | Read-only endpoint/scope truncates the tool list | Yes, by deliberately using the full `/mcp` endpoint with a write-scoped credential, not `/mcp/readonly` |
| Sentry | OAuth-scope gating, source-confirmed; possible paid-tier gate on one AI tool (unconfirmed) | Likely, via a broad-scope OAuth grant; needs live verification |
| Slack (community) | Not scope gating in the usual sense; "stealth mode" bypasses Slack's OAuth model via browser-session cookies, a compliance question, not an enumeration one | Roshan's decision: stealth mode risks a ToS question, scoped OAuth mode is a genuine partial surface |

Grafana and Atlassian/Jira were the two hard cases: their full tool surfaces are structurally behind paid product tiers, not just permission scopes, and a free-tier credential cannot reach them. Ratification on 2026-08-17 resolved this by applying the partial-surface refusal rule: both are excluded (option (c) of the three this document originally raised), rather than published with a "measured subset" caveat (option (a)) or backed by provisioned paid test access outside the $0 Tier 1 budget (option (b)). See B.2.1.

### B.4 SQLite: considered, evidence-excluded

The brief named "Postgres/SQLite" as a paired candidate. Research found the SQLite reference implementation (`modelcontextprotocol/servers-archived/src/sqlite`) archived alongside Postgres, frozen since 2025-05-28, with the same "no security guarantees" banner. Unlike Postgres, no actively maintained, comparably adopted successor was found in this research pass; the legacy PyPI package `mcp-server-sqlite` still shows 56,655 downloads/month, but that is residual traffic on a dead package, not evidence of a viable current candidate (`pypistats.org`, checked 2026-08-17). SQLite is therefore excluded from the initial 15 on Gate 4 grounds (abandoned, no commit in over 12 months) rather than folded in as a weak entry. It stays on the bench: if a maintained SQLite MCP server surfaces via the self-submission path (rule A.5), it re-enters consideration on its own evidence.

### B.5 Five reserve candidates

Selected for strong independent adoption evidence and readiness to backfill a category slot, particularly the two auth-blocked candidates in B.3. Reserves 1 and 2 were promoted into the 15 on 2026-08-17 (see B.2 item 10 and item 14 at the time, and B.2.1); they stay listed here in their original reserve-ordering position for the record. Reserve 1, Figma, was subsequently excluded 2026-08-18 on a separate Gate 2 failure discovered at onboarding (see B.2.1); reserve 3, Chrome DevTools MCP, was promoted the same day to backfill the vacated slot (see B.2 item 15) and is next in reserve order after reserves 1 and 2.

1. **Figma MCP.** Promoted into the 15, 2026-08-17 (originally B.2 item 10); excluded 2026-08-18 under the partial-surface rule once the primary candidate resolved to an OAuth-2.1-browser-flow-only remote server with no static-token convention (see B.2.1). (community: GLips/Figma-Context-MCP, 15,670 stars, 1,242 forks, last push 2026-08-07; official: figma/mcp-server-guide, 1,896 stars, last push 2026-08-14; both checked 2026-08-17). Fills a design/creative-tooling gap no current candidate covers. Higher raw stars than several servers already in the 15 (Grafana at 3,361, Cloudflare's domain-specific server at 4,082). Re-eligible per rule A.5 if the harness gains OAuth support or Figma ships a static-token convention.
2. **Context7.** Promoted into the 15, 2026-08-17 (see B.2 item 13). (Upstash, upstash/context7, 60,868 stars, pushed 2026-08-17, checked same day). Documentation-retrieval category; independently strong on both Smithery and PulseMCP's traffic list, the single strongest multi-source adoption signal of any server surfaced in this research pass.
3. **Chrome DevTools MCP.** Promoted into the 15, 2026-08-18, backfilling the slot vacated by Figma's exclusion (see B.2 item 15). (Google, ChromeDevTools/chrome-devtools-mcp, 49,287 stars, pushed 2026-08-17, checked at the time this reserve list was compiled; 49,349 stars checked live again 2026-08-18 at promotion). Browser-automation backup for Playwright, with a higher raw star count than Playwright itself. No credential of any kind gates its tool surface; onboarding found 52 tools live with every free category/experimental flag enabled (B.2 item 15).
4. **Brave Search MCP** (official, brave/brave-search-mcp-server, 1,384 stars, last push 2026-08-14, checked 2026-08-17). Fills a web-search category gap; notably, the archived `modelcontextprotocol/servers-archived` README names this repo as the official successor to its own archived Brave Search reference server, and it is the single highest-useCount server found on Smithery (104,186).
5. **AWS Labs MCP** (official, awslabs/mcp, 9,609 stars, 1,709 forks, pushed 2026-08-14, checked 2026-08-17). Cloud-infrastructure category gap; a component of this monorepo (AWS Documentation) also appears on PulseMCP's claimed top-15 list.

### B.6 Conflict of interest: HydraDNS

HydraDNS is Roshan's own project. Per the recusal policy that PRD.md 4 requires to be published before the first ranking, any server Roshan built or majority-controls is subject to recusal, and per rule A.8 above, HydraDNS's MCP server (if one exists or is built) is either excluded from ranking entirely or carries a permanent, explicit conflict-of-interest disclosure, whichever the recusal policy specifies. **HydraDNS is not proposed for inclusion in this 15-server list or the reserve bench.** No research was conducted on it for this document, and none should be cited toward its inclusion until the recusal policy itself is published and Roshan has made the exclude-or-disclose decision it requires. This is his call, not a default this document sets.
