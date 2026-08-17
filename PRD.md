# loadline

**PRD + Execution Plan**

| Field | Value |
| --- | --- |
| Status | Draft v0.2 |
| Date | 2026-08-17 |
| Owner | Roshan Singh |
| Working title | loadline (chosen 2026-08-17; was MCP Stack Cost Lab) |
| Decision basis | Planning session with live research, 2026-08-16 |

---

## Executive Summary

**One-liner:** Lighthouse for MCP. A standing, versioned, vendor-neutral measurement of what MCP servers actually cost an agent's context window, centered on a stack calculator.

Every team wiring MCP servers into an agent faces the same unanswered question: how much of my context window does this stack eat before the agent does any work? Six or more vendors published one-shot token studies in 2026 and none of them maintained the data. One standing competitor exists, PolicyLayer, and it measures the wrong number with one tokenizer.

The product is a calculator, not a table. A user picks servers, a client, and a model, and gets total context footprint at session start, percentage of a 200k window consumed, per-server attribution, and a cold-write / cache-read dollar pair. Results are always reported per client mode, because a stack that costs 72k tokens under naive full-load costs roughly 8.7k under Claude Code's Tool Search and roughly 1k under Code Mode. A single number for MCP cost is a wrong number.

Two measurement tiers back the calculator. Tier 1 is a fully automated monthly sweep of about 15 curated servers at roughly zero marginal cost, because the token-counting endpoints we depend on are free. Tier 2 is scripted real-task runs against 3 servers through a call-logging interposer proxy, which supplies the capability axis that stops cost-only rankings from being actively harmful.

The moat is governance, and it ships in v1: a published server selection rule, a recusal policy published before the first ranking, raw run artifacts, a public corrections log, a 14-day right-of-reply window, and a harness anyone can re-run with their own credentials.

Target: first public release 2026.10, roughly six weeks from Phase 0.

---

## 1. Product

### 1.1 What it is

The core product is the **stack calculator**. The user selects:

- one or more MCP servers (for example GitHub MCP + Grafana MCP + Playwright)
- a client
- a model

The calculator returns:

| Output | Detail |
| --- | --- |
| Total context footprint | Tokens consumed at session start, before any work |
| Window share | Percentage of a 200k window consumed |
| Per-server attribution | Which server is costing what |
| Dollar cost | Shown **only** as a cold-write / cache-read pair, never a single figure |
| Client-mode breakdown | Three modes, always three, never collapsed into one |

Every stack gets a shareable permalink. That permalink is the distribution surface and the SEO surface at once.

A leaderboard and a monthly changelog exist as secondary views over the same underlying data. They are not the product.

### 1.2 The three client modes

Reporting a single cost number is the mistake that made every 2026 one-shot study obsolete within a quarter. Costs are reported per mode:

| Mode | Behavior | Reference point |
| --- | --- | --- |
| Naive full-load | All tool definitions loaded upfront | Gemini CLI today |
| Tool Search / progressive disclosure | ~500-token search stub upfront, 3 to 5 tools (~3k) loaded on demand | Claude Code default since v2.1.7, 2026-01-14. Anthropic's published figures: 72k to 8.7k for 50+ tools |
| Code Mode | Entire API expressed in ~1k tokens | Cloudflare-style |

### 1.3 Why dollars are a pair, not a number

Prompt caching makes single dollar figures roughly 10x overstated. Tool definitions are a near-perfect cache prefix: stable across turns, at the front of the context, read at 0.1x cost. Publishing one dollar figure for a stack is publishing the cold-start figure and implying it recurs. The calculator therefore shows cold-write and cache-read side by side, and tokens remain the primary unit throughout.

### 1.4 What this is explicitly not

| Not this | Why |
| --- | --- |
| "Bundlephobia for MCP" | Bundle size was stable and client-independent. MCP cost is neither. The analogy misleads on both axes. |
| A popularity directory | Smithery, PulseMCP, mcp.so, LobeHub, and the official registry already do that. |
| An accuracy benchmark | MCP-Bench and Scale AI's MCP-Atlas own task-completion accuracy. Different dimension, not our claim. |

---

## 2. Metrics and Methodology

### 2.1 Headline metrics

The obvious metric, upfront schema footprint, measures a client default Anthropic turned off in January 2026. It stays in the dataset but loses headline position.

| Metric | What it captures |
| --- | --- |
| Per-tool loaded cost | Schema tokens per tool actually retrieved, the number that matters under progressive disclosure |
| Description retrievability | Whether a tool surfaces under BM25 / regex-style search over names and descriptions |
| Schema hygiene grade | Rewards well-described, discoverable tools |
| Naive upfront footprint | Retained as one column, explicitly labelled "naive client" |

Hygiene and retrievability exist to defuse a specific Goodhart failure. If the ranking rewards small schemas alone, authors gut their descriptions to win, and gutted descriptions measurably break agent accuracy. The scoreboard must punish that move, not pay for it.

### 2.2 Multi-tokenizer measurement

| Provider | Method | Notes |
| --- | --- | --- |
| Claude | `count_tokens` API | No offline tokenizer exists. Free. |
| OpenAI | tiktoken, local | Free, offline |
| Gemini | Local tokenizer / free `countTokens` | Free |

Counts shift between model releases. Sonnet 5 counts roughly 30% higher than Sonnet 4.6 for identical text. Every published cell is therefore stamped with `(model, tokenizer, measured_at)`, and the changelog separates two delta types that would otherwise be indistinguishable: **server changed** and **tokenizer or harness changed**. A reader who cannot tell those apart cannot trust any trend line.

### 2.3 Tier 1: static sweep, monthly

- Scope: about 15 curated famous servers at v1. The selection rule is published. We do not chase PolicyLayer's 6,686-server coverage, and coverage is not a claim we make.
- Procedure, fully automated: launch server, MCP handshake, enumerate tools, count tokens.
- Cost: roughly zero. No LLM inference is involved, only free counting endpoints and local tiktoken.
- Provenance per run: server version plus a hash of the exact `tools/list` response. This is the anti-gaming record.

**Protocol coverage.** The harness must speak at least three MCP protocol revisions. The 2026-07-28 revision removed the `initialize` handshake and sessions and added `server/discover`. SEP-2106 permits arbitrary JSON Schema 2020-12 including `$ref`, which means our `$ref` resolution behavior is a methodology choice and is documented as one rather than presented as the only reading.

**Failure handling.** Failed servers auto-publish as data points ("unreachable, 2026-09-01"). A failure never blocks a release. Expect 2 to 4 failures per monthly run: 2026 audits put roughly 52% of MCP projects abandoned and 17.2% of remote servers unreachable. Server rot is part of the subject matter, so it is part of the dataset.

### 2.4 Tier 2: dynamic runs, 3 servers, mandatory

Tier 2 runs scripted real tasks through our **interposer**, an MCP call-logging proxy of roughly 150 to 250 lines. It measures real call-and-response token flows, which is the capability axis that keeps Tier 1 honest.

The interposer is already planned in Roshan's estate, and building it here also closes an instrument debt in jaeger-mcp-bench, where the Gemini runner never captured tool arguments.

Two hard rules:

1. **If Tier 2 stops running, the whole leaderboard pauses.** Cost-only rankings without a capability axis are actively harmful, and shipping them would be worse than shipping nothing.
2. **Never compare results across harness versions.** The interposer is versioned like a product. Harness variance can exceed model variance by a wide margin; the specific multiplier sits in the evidence-debt register and is internal guidance only.

Tier 2 refreshes less often than Tier 1.

---

## 3. Competitive Landscape

Verified live on 2026-08-16.

| Player | What they do | Gap we fill |
| --- | --- | --- |
| **PolicyLayer** (policylayer.com/token-cost) | The only standing competitor. Live, auto-refreshing, 6,686+ servers, per-server pages, percentile stats | tiktoken only; raw upfront token counts, the deprecated metric; zero community footprint (no HN, Reddit, or Twitter mentions found); lead-gen side asset of an AI-governance product |
| **tare-mcp** (nishantmodak/tare-mcp) | Inspect your MCP tool setup before the agent runs: context weight, token usage, server dominance, tool overlaps. Single-shot CLI inspection, under 5 GitHub stars (checked 2026-08-17) | No standing cadence, no multi-client modeling, no governance. Confirms the measurement itself is replicable; the moat is the standing infrastructure |
| **Glama** | Schema-quality score (Tool Definition Quality 70% / Server Coherence 30%) | No token or cost metric at all |
| **Smithery, PulseMCP, mcp.so, LobeHub, official registry** | Popularity metadata | No cost measurement |
| **Anthropic (SEP-2127 server cards)** | Server card spec | Explicitly scopes tool-surface data **out**. No first-party threat imminent |
| **MCPSpend, MCPcat** | Per-customer observability | No public cross-server rankings. MCPSpend has one categorical cost-share table from telemetry |
| **MCP-Bench, MCP-Atlas (Scale AI)** | Task-completion accuracy | Different dimension, complementary rather than competing |
| **Six+ vendor one-shots** (Cyclr, Scalekit, getunblocked, OnlyCLI, MindStudio, StackOne) + dead repo zhang-liz/mcp-token-benchmark (0 stars) | One-time token studies | Proof of demand. None maintained. All vendor-interested |

**Our differentiation against PolicyLayer, in order:** per-client modes, multi-tokenizer, stack composition, a dynamic tier, a neutral dedicated brand, and published governance.

**Demand caveat, baked into the design.** Those measurement posts got near-zero HN engagement. OnlyCLI's scored 2 points. The 400-point threads were opinion pieces, not data. The lesson: utility plus argument spreads, bare numbers do not. That is precisely why the calculator is the product and the leaderboard is a view, and why each season ships a written argument alongside the data.

---

## 4. Governance

Governance is not a compliance chore here. It is the moat, and it ships in v1.

| Commitment | Shape |
| --- | --- |
| Server selection rule | Published, stable, dated. Changes to it are changelog events |
| Recusal policy | Published **before** the first ranking, covering funding and employment |
| Corrections log | Public, append-only |
| Raw run artifacts | Published alongside every release |
| Right of reply | 14-day window before a server's numbers go public |
| Third-party reproducibility | Harness re-runnable by anyone with their own credentials |
| Auth transparency | Auth scope and tier published per row |
| Partial-surface refusal | A server whose full tool surface we cannot enumerate is refused a ranking, never partially counted |

Publishing the harness as re-runnable is the specific insurance against vendor pressure: revoking our API key cannot erase a finding that anyone can reproduce.

Every public number flows through Roshan's `~/PR/CLAIMS.md` ledger discipline.

---

## 5. Risks and Evidence Debt

### 5.1 Ranked risk register

From an adversarial pre-mortem. Reviewed each season.

| # | Risk | Mitigation |
| --- | --- | --- |
| 1 | Metric obsolescence as clients evolve (Tool Search, Code Mode) | Per-client-mode reporting **is** the product. Spec churn becomes content: "what Tool Search saves on your stack" |
| 2 | Prompt caching invalidates dollar claims | Tokens primary. Dollars only as a cold/cached pair |
| 3 | Spec churn breaks the harness | Budget one harness revision per MCP spec release |
| 4 | Claude tokenizer drift creates phantom deltas | Stamped cells, separated delta types in the changelog |
| 5 | Auth or tier-gated tool lists | Publish scope per row, refuse partial rankings |
| 6 | Goodhart: authors strip descriptions to win | Hygiene and retrievability metrics, Tier 2 capability axis, never a single rank |
| 7 | "You benchmarked a harness, not the protocol" | Published stable selection rule plus dated methodology. This objection is exactly why a standing artifact beats a one-shot |
| 8 | Operator burnout from breakage triage | Automated failure path, failures-as-data, monthly cadence for the free tier only |
| 9 | Vendor pressure via API-key revocation (**unverified precedent**) | Right of reply, raw artifacts, third-party reproducibility |
| 10 | Neutrality collapse when sponsors or job offers arrive (**unverified case studies**) | Recusal policy published day one |
| 11 | Servers detecting benchmark runs and serving different builds | Version plus `tools/list` hash provenance per run |

### 5.2 Evidence-debt register

Three research strands remain unverified. **None of the following may appear in public-bound text until re-researched against primary sources.** They are internal design guidance only.

| Strand | Status |
| --- | --- |
| (a) Criticism threads on the vendor one-shot studies | Unverified. Do not cite |
| (b) Abandonment history of zhang-liz/mcp-token-benchmark and peers | Unverified. Do not cite as a pattern |
| (c) Bundlephobia / LMArena / SWE-bench neutrality-and-burnout war stories, and the 7.8x harness-variance figure | Unverified. The 7.8x number informs harness-versioning discipline internally and must not be published |

---

## 6. Costs

| Line | Amount | Notes |
| --- | --- | --- |
| Tier 1 API | ~$0 | Free token-counting endpoints, local tiktoken |
| Tier 1 labor | Service accounts and keys for ~15 servers | Free tiers suffice for enumeration |
| Tier 1 compute | $0 | Roshan's PC |
| Tier 2 runs | ~$0 cash | Ceiling of ~150 agent runs per season, riding Claude Max plan quota and Gemini CLI free tier |
| Tier 2 fallback | $50 to $150 per season | Metered-API ceiling, only if independence from personal plans is ever needed |
| Domain | $10 to $30 / yr | |
| Hosting | $0 | GitHub Pages or Cloudflare Pages, static |

Wire-level token counts come from the interposer, so the measurements are billing-method-independent. Riding personal plan quota changes the cost of running the benchmark, not the validity of its numbers.

---

## 7. Execution Plan

### 7.1 Operating model

| Actor | Scope |
| --- | --- |
| Claude agent sessions | The building. Sonnet for mechanical work, Opus for judgment-heavy harness and methodology design |
| Roshan's PC | Heavy multi-server runs, prepared by agents and handed off |
| Roshan's personal hours | Methodology sign-off, server-list ratification, branding, DEPTH-style ownership of results before publishing, and all public text |

All public text passes Roshan's full public-text gate: fact-checked against sources, no AI smell, his own voice. No exemptions by size.

### 7.2 Phases

**Phase 0, week of Aug 18. Gate: Roshan ratifies the ledger edits and the server list.**

- Branding decision: generate name and domain candidates, run availability checks
- Home directory plus repo scaffold
- Draft selection rule and methodology doc v0
- Ratify the initial 15-server list
- Supersede the two existing `NEXT.md` rows (`mcp-interposer`, `mcp-tier-a-sweep`) with this project's rows

**Phase 1, weeks 1 to 3. Gate: harness reproduces a known server's counts across all three tokenizers.**

- Tier 1 harness: multi-revision MCP client, tokenizer adapters, provenance capture, failure-as-data path
- Interposer proxy v1, which also closes the jaeger-mcp-bench tool-argument capture debt
- 15-server corpus onboarding
- Calculator site MVP with stack permalinks

**Phase 2, weeks 3 to 5. Gate: governance docs final, right-of-reply dry run completed.**

- Tier 2 task suites for 3 servers
- Harness versioning discipline in place
- Governance docs: selection rule final, recusal policy, corrections log, right-of-reply process
- Dry-run the right-of-reply process against one friendly server

**Phase 3, week 6, late Sept or early Oct. Gate: verify-still-holds re-scan passes, every number cleared through CLAIMS.md.**

- First public release, 2026.10
- Launch writeup on blog and dev.to, riding the jaeger-mcp-bench audience
- Listings on awesome-mcp lists and registries
- Direct notes to the six vendors who ran one-shot studies. They have a standing incentive to cite a neutral reference rather than defend their own stale numbers
- HN submission is Roshan's decision. The 2-per-quarter cap likely argues for the fresh Q4 slot

### 7.3 Steady state and seasons

| Cadence | Work |
| --- | --- |
| Monthly | Tier 1 release with changelog |
| Per season, or on material server change | Tier 2 refresh |
| v2 season, Oct to Nov | Progressive-disclosure referee: Anthropic Tool Search vs Cloudflare Code Mode vs Bifrost, all under one harness. Vendor claims are 85%, 99.9%, and 92.8% |
| v3 season | Cross-harness consistency: the same server across Claude Code, Gemini CLI, and Codex. No such artifact exists anywhere today |
| 2027 at the earliest | CI-kit spinoff, and only after methodology authority is established |

### 7.4 Standing gates

- Verify-still-holds re-scan of PolicyLayer and the registry landscape before launch, and before each season
- Every public number through `CLAIMS.md`
- Pre-mortem risk register reviewed each season

---

## 8. Strategic Context

This fills the primary-focus slot vacated when HydraDNS was parked as a business in July 2026.

Win condition, chosen by Roshan: **real adoption and usage**, with the maintenance tail accepted as part of the deal. This is not a portfolio piece that can be abandoned after launch, and the Tier 2 pause rule encodes that.

Scheduling constraint: this must not collide with the Sep 30 OSS campaign goal of 2 talkable features. If the stalled cel-go feature PR wakes up, hours flex toward it and the Phase 3 date moves. The monthly-only cadence for Tier 1 exists partly to keep that flex available.

---

## 9. Success Metrics

Measured at 90 days post-launch. Honest-metrics rule applies: no small-n percentage framing, absolute counts only.

| Metric | Why it counts |
| --- | --- |
| Server authors citing or linking their scores | The strongest signal that the numbers are treated as authoritative rather than as marketing |
| Calculator sessions | Utility, not readership |
| Stack permalinks shared | Distribution working as designed |
| Servers self-submitting for inclusion | The point at which the selection rule becomes a queue instead of a chore |
| Corrections raised and handled publicly | A corrections log with entries is healthier than an empty one. Zero corrections at 90 days means nobody is checking |

Deliberately excluded: GitHub stars, HN points, and total servers covered. Coverage is PolicyLayer's claim, not ours.

---

## 10. Open Decisions

| # | Decision | Owner | Blocking |
| --- | --- | --- | --- |
| 1 | Name and domain: resolved. loadline / loadline.dev, decided 2026-08-17; domain registration pending | Roshan | Phase 0 |
| 2 | Home directory and `MAP.md` registration | Roshan ratifies | Phase 0 |
| 3 | Initial 15-server list ratification | Roshan | Phase 1 corpus onboarding |
| 4 | HN slot timing | Roshan | Phase 3 |
| 5 | Tier 2 task-suite design sign-off | Roshan | Phase 2 |
