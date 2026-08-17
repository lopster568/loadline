# Branding Candidates

**Product:** vendor-neutral standing measurement of MCP server context-window cost, centered on a stack calculator.
**Positioning:** "Lighthouse for MCP" — measurement infrastructure, not a startup, not a joke.
**Owner:** Roshan Singh (brand deliberately not tied to his name).
**Date:** 2026-08-17
**Status:** research complete, one open gap (see Limitations)

---

## Recommendation

**`loadline`** — primary domain **`loadline.dev`** (also secure `loadline.tools`).

Everything below is the evidence.

---

## 1. Method and how to read the results

Domain availability was checked over RDAP against **validated registry endpoints**, not `rdap.org`. This matters: the first pass through `rdap.org` was rate-limited into garbage (HTTP 429 on most lookups), and a second pass through `https://www.registry.google/rdap/` returned a false `404 AVAILABLE` for **every** `.dev` name — the control `web.dev` also came back "available", which is obviously wrong. The endpoint was silently broken.

Endpoints actually used, each confirmed with a known-registered control and a known-nonexistent control:

| TLD | Endpoint | Control (taken) | Control (free) |
| --- | --- | --- | --- |
| `.com` | `rdap.verisign.com/com/v1/` | `google.com` → 200 | — |
| `.dev` | `pubapi.registry.google/rdap/` | `web.dev` → 200 | `zzqqxxnotreal12345.dev` → 404 |
| `.io` | `rdap.identitydigital.services/rdap/` | `google.io` → 200 | `zzqqxxnotreal12345.io` → 404 |
| `.tools` | `rdap.identitydigital.services/rdap/` | `caliper.tools` → 200 | — |

`404` = unregistered, `200` = registered. **`.dev` had to be queried serially with backoff** — Google's RDAP rate-limits aggressively and returns 429, which is easy to misread as a result.

Two caveats on "AVAILABLE":

- **Unregistered is not the same as cheap.** Short one-word `.io` names (`tare.io`, `heft.io`) are almost certainly registry-reserved premium inventory. Price them before planning around them.
- **npm and GitHub were checked separately** (`registry.npmjs.org/<name>`, authenticated `gh api users/<name>`). Unauthenticated GitHub checks return 403 and are worthless; the numbers below are authenticated.

---

## 2. Candidate pool (24 checked)

Domain columns: `FREE` = unregistered per RDAP, `—` = registered.

### Descriptive

| Name | .com | .dev | .io | .tools | npm | GH org | Verdict |
| --- | --- | --- | --- | --- | --- | --- | --- |
| contextcost | — | — | FREE | FREE | free | free | **BLOCKED** |
| contextmeter | — | FREE | FREE | FREE | free | free | **BLOCKED** |
| mcpgauge | FREE | FREE | FREE | FREE | free | free | **BLOCKED** |
| tokenledger | — | — | — | FREE | taken | taken | **BLOCKED** |
| toolcost | — | FREE | FREE | FREE | free | free | CLEAR |
| toolload | — | FREE | FREE | FREE | free | free | crowded |
| toolscale | — | FREE | — | FREE | free | free | crowded |
| stackweigh | FREE | FREE | FREE | FREE | free | free | CLEAR |
| stackweight | — | FREE | FREE | FREE | free | free | CLEAR |

### Metaphor — weight / load / measurement

| Name | .com | .dev | .io | .tools | npm | GH org | Verdict |
| --- | --- | --- | --- | --- | --- | --- | --- |
| loadline | — | FREE | — | FREE | free | dormant squat | **CLEAR** |
| loadmark | — | FREE | FREE | FREE | taken | taken | crowded |
| weighbridge | — | FREE | FREE | FREE | free | taken | CLEAR |
| dunnage | — | FREE | — | FREE | free | taken | CLEAR |
| tare | — | — | FREE | — | taken | taken | crowded |
| heft | — | — | FREE | FREE | taken | taken | **BLOCKED** |
| ballast | — | — | — | FREE | reclaimable | taken | crowded |
| plimsoll | — | — | — | FREE | taken | taken | crowded |
| deadweight | — | — | — | FREE | taken | taken | crowded |
| tonnage | — | FREE | — | FREE | free | taken | crowded |
| lading | — | FREE | — | FREE | free | taken | crowded |
| caliper | — | — | — | — | taken | taken | **BLOCKED** |
| fathom | — | — | — | — | taken | taken | **BLOCKED** |
| freeboard | — | — | — | FREE | taken | taken | crowded |
| yardstick / assay / plumbline / loadout / burden | — | — | — | — / — / — / — / FREE | — | — | dead on arrival |

### Why the BLOCKED names are blocked

| Name | Blocking collision |
| --- | --- |
| **fathom** | Worst in the pool. Two prominent products (Fathom Analytics, 8,012★ `usefathom/fathom`; Fathom AI notetaker at fathom.ai) **plus an existing multi-repo MCP ecosystem** — `Dot-Fun/fathom-mcp`, `agencyenterprise/fathom-mcp-server`, and five more. The name is already spoken in MCP contexts. |
| **caliper** | `hyperledger-caliper/caliper` (703★, active blockchain **benchmark framework** — same verb, same audience), `google/caliper` (817★ microbenchmark lib), `llnl/Caliper` (416★). Plus Caliper Corp, a ~60-year-old assessment brand, and IMS Caliper Analytics, an ed-tech standard. All four TLDs taken. |
| **heft** | `@rushstack/heft` — Microsoft's actively maintained JS/TS build orchestrator (`microsoft/rushstack`, 6,489★, published within days of this check). Naming a dev tool `heft` in 2026 is naming it after a Microsoft build tool. |
| **mcpgauge** | Two independent projects already converged on this exact name **in the MCP-grading niche**: `Michael-WhiteCapData/mcpgauge` ("deterministic, offline quality grader for MCP servers", live on PyPI as `mcpgauge`) and `RitikPatill/mcpgauge` (MCP eval framework). Perfect domain availability is a trap here — the name is claimed where it counts. |
| **contextmeter** | Five GitHub repos on the bare name, two doing nearly this product: `Bahgs/contextmeter` ("context observability and token economics for AI coding agents") and `rogerchappel/contextmeter` ("local-first context window analyzer"). PyPI package live: "a live meter for your LLM's context window — see exactly what's filling it." |
| **contextcost** | `CAOShurong/contextcost` and a live PyPI package, both pitched as "measure what a repository costs an AI coding agent to read." Same words, same concept. |
| **tokenledger** | npm `tokenledger` is a live versioned package (v0.1.6, gyde.ai) doing token tracking across AI coding agents; GitHub org taken; `zh667/TokenLedger` at 22★ on the identical concept. |

---

## 3. Competitive finding worth acting on independently of the name

The PRD names PolicyLayer as the one standing competitor. The GitHub sweep turned up more of the niche, all early-stage:

| Project | Stars | What it does |
| --- | --- | --- |
| `nishantmodak/tare-mcp` | 4 | **"Inspect your MCP tool setup — see context weight, token usage, server dominance, and tool overlaps before your agent runs."** |
| `Michael-WhiteCapData/mcpgauge` | 1 | Offline grader for MCP server tool definitions, on PyPI |
| `RitikPatill/mcpgauge` | 0 | MCP eval framework, LLM-as-judge, regression detection |
| `sneha4175/mcp-tool-router` | 0 | MCP proxy retrieving top-k relevant tools to cut context bloat |
| `jijoyo/opencode-lazy-load` | 0 | Claims 88–90% MCP tool token overhead reduction |
| `Cali0707/mcp-token-experiments` | 0 | Proxy experiments to reduce MCP token counts |
| `hegner123/tersemcp.com` | 0 | Catalog of "token-efficient" MCP servers |
| `zenthys/toolLoading` | 0 | Go SDK for "context explosion" from too many tool definitions |
| `GlobalUnderdog/mcp-token-cost` | 0 | Name-adjacent, no description |
| `maxi-maxima/mcp-context-window-cost-planner` | 0 | Name-adjacent, no description |

**Two reads.** The niche has no incumbent — nothing above 4 stars, so the PRD's positioning holds. But `tare-mcp` is a one-line restatement of this product's own pitch, which is why `tare` came off the shortlist and why the PRD's moat claim (governance: selection rule, recusal policy, raw artifacts, corrections log, right-of-reply) is the part that actually differentiates. The measurement alone is not defensible; several people have already built it.

---

## 4. Shortlist

### 1. `loadline`

- **Domains:** `loadline.dev` FREE, `loadline.tools` FREE. `.com`, `.io`, `.app` registered.
- **npm:** free. **GitHub:** bare `loadline` is a dormant squat — user created 2020-03-21, 0 public repos, never updated after creation day. `loadline-dev`, `loadlinehq`, `getloadline`, `loadlinetools` all free.
- **Collisions:** none material. Top GitHub hit is `andmarti1424/loadline_plotter` at 6★ (vacuum-tube plotting). No MCP-space hit, no AI/dev-tool product.
- **Rationale:** the Plimsoll load line is a standing, regulator-published, vendor-neutral mark stating how much load is safe — and it carries **several marks on the same hull for different conditions** (tropical, summer, winter, fresh water). That is exactly the PRD's central argument: three client modes, always three, and "a single number for MCP cost is a wrong number." The metaphor does the product's hardest explaining job before the reader finishes the title. It sits in the same maritime-instrument register as "Lighthouse" without imitating it, reads as infrastructure, and verbs cleanly ("what's its loadline?"). Scope-safe: a load line is about safe capacity under stated conditions, not about tokens — v2 progressive-disclosure refereeing and v3 cross-harness testing are just more marks on the same hull, no rename.
- **Weakness:** you never get the clean canonical handle. `.com` and `.io` are gone and the bare GitHub username is squatted (dormant, but not yours), so the brand permanently lives at `loadline.dev` + a suffixed org. "Load line" is also an electronics term, which means early SEO fights vacuum-tube tutorials.

### 2. `stackweight`

- **Domains:** `stackweight.dev` FREE, `stackweight.io` FREE, `stackweight.tools` FREE. `.com` registered.
- **npm:** free. **GitHub org:** free.
- **Collisions:** effectively none. Only `vjcitn/stackweight` at 0★ (R package load times).
- **Rationale:** the cleanest namespace in the pool paired with the most literal tie to the product's core noun — the PRD calls it a *stack calculator*, and this is what the stack weighs. Zero explanation needed in a README, and it survives an HN title without sounding clever.
- **Weakness:** plain to the point of forgettable, and "weight" is the narrow framing the brief warned about — it describes v1 token accounting well but sits awkwardly over v2 refereeing and v3 cross-harness capability testing. Also loses `.com`. *(Note: the variant spelling `stackweigh` is the one name in the pool with **all four TLDs free** plus free npm and free GitHub org. It is worth registering defensively, but not as the primary — see weakness of #5.)*

### 3. `weighbridge`

- **Domains:** `weighbridge.dev` FREE, `weighbridge.io` FREE, `weighbridge.tools` FREE. `.com` registered.
- **npm:** free. **GitHub:** bare org taken; `weighbridge-dev` / `getweighbridge` free.
- **Collisions:** none in software. Top hits are literal industrial truck-scale projects, max 19★ (`Amexatgit/WEIGHBRIDGE-AUTOMATION-SYSTEM`), all HX711/ESP8266 hardware. No AI or dev-tool brand.
- **Rationale:** semantically the closest match to the actual positioning. A weighbridge is a public, neutral, standing scale that every vehicle drives onto before entering — operated by nobody's vendor, trusted because it is boring and always there. That is the governance moat expressed as a noun, and it makes the recusal policy and corrections log feel native rather than bolted on.
- **Weakness:** eleven characters and three syllables, and it is British/industrial vocabulary — much of the US audience says "truck scale" and will not have the referent. Heavy in an HN title and slow to type as a URL.

### 4. `toolcost`

- **Domains:** `toolcost.dev` FREE, `toolcost.io` FREE, `toolcost.tools` FREE. `.com` registered.
- **npm:** free. **GitHub org:** free.
- **Collisions:** minor. `ArtemKx1/opencode-toolcost` at 1★ is a per-tool token/cost tracker for OpenCode — adjacent concept, but scoped and prefixed, and the bare name is unclaimed on every registry.
- **Rationale:** maximum clarity for zero cleverness. A reader who has never heard of the project understands the whole value proposition from the URL, which is the single most valuable property for a link that server authors will drop into READMEs without context.
- **Weakness:** the narrowest name on the shortlist and the one the brief explicitly cautioned against. "Cost" hard-codes v1 token-and-dollar framing into the brand; the moment v2 starts refereeing progressive disclosure and v3 tests across harnesses, the name is measurably describing less than the product does. Also the least memorable — nothing here becomes a habit or a verb.

### 5. `dunnage`

- **Domains:** `dunnage.dev` FREE, `dunnage.tools` FREE. `.com`, `.io` registered.
- **npm:** free. **GitHub:** bare org taken by a low-signal account; variants free.
- **Collisions:** none. Nothing above 0★; all hits are literal packing-material businesses.
- **Rationale:** the sharpest metaphor in the pool. Dunnage is the packing material that consumes cargo space while being no part of the cargo — a precise, slightly wry name for context consumed before the agent does any work. Distinctive enough that the URL is unforgettable once learned, and completely uncontested.
- **Weakness:** fails the stated hard requirement outright. Nobody spells "dunnage" correctly after hearing it once (dunnidge, dunage, donnage), and almost nobody knows the word, so the metaphor lands only after it is explained — which is backwards for a name whose job is to travel in citations and HN titles unaccompanied.

---

## 5. Ranking

| # | Name | Best domain | Why it sits here |
| --- | --- | --- | --- |
| 1 | **loadline** | `loadline.dev` | Only candidate where the metaphor argues the product's thesis (multiple marks, multiple conditions) while staying short, spellable, and scope-proof. |
| 2 | stackweight | `stackweight.dev` | Cleanest namespace and instantly legible, but narrow framing and forgettable. The safe pick if the metaphor is judged too indirect. |
| 3 | weighbridge | `weighbridge.dev` | Best expression of neutral standing infrastructure; loses on length and on a referent half the audience lacks. |
| 4 | toolcost | `toolcost.dev` | Clearest, most disposable. Correct only if v2/v3 never ship. |
| 5 | dunnage | `dunnage.dev` | Best insider metaphor, disqualifying spellability. |

### Recommendation: `loadline`, at `loadline.dev`

The decision turns on one requirement doing more work than the others: the name has to survive v2 and v3 without a rename, because a *standing* measurement that rebrands has broken the only promise it makes. That eliminates `toolcost` and pressures `stackweight` — both describe token accounting, and the roadmap is progressive-disclosure refereeing and cross-harness testing. A load line does not describe tokens; it describes how much load is safe under a stated condition, and it has always come in sets of marks for different conditions. The product's core editorial claim, that a single MCP cost number is a wrong number, is already encoded in the object the name points at.

It also passes the practical filters the others split on: eight characters, two familiar syllables, spelled correctly on first hearing, no MCP-space collision, no prominent product, free on npm, and it reads as an instrument rather than a company. `Loadline: what your MCP stack costs before the first message` is a working HN title today.

Accept the two costs knowingly. The canonical `.com` and `.io` are gone and the bare GitHub username is a dormant squat, so the permanent home is `loadline.dev` with a suffixed org — acceptable, since `.dev` reads as native to this audience and the URL is the citation surface, not the org handle. And the electronics sense of "load line" will contest early search results, which argues for consistently shipping the name attached to its qualifier ("Loadline for MCP") until the term is owned.

**Register together:** `loadline.dev`, `loadline.tools`, npm `loadline`, GitHub org `loadline-dev` (or `loadlinehq`).

---

## 6. Limitations and open actions

1. **Trademark clearance is incomplete.** The session's web-search budget was exhausted (200/200) before dedicated trademark queries could run, and the HTML-scraping fallbacks were blocked. Everything above rests on GitHub, npm, and PyPI registry evidence plus targeted page fetches. **Before committing, run a real USPTO/EUIPO search for `loadline` in software/SaaS classes (Nice 9 and 42).** The freight and logistics sector is the likely source of a same-word mark, and while that is a different class, it should be seen rather than assumed.
2. **Premium pricing unverified.** RDAP reports registration status, not price. `loadline.dev` is a two-word compound and should be standard-priced, but confirm at the registrar before planning the launch around it.
3. **`.app` coverage is partial.** Google's RDAP rate-limiting truncated the `.app` sweep; only `ballast`, `heft`, `tare`, `loadline`, and `plimsoll` were resolved (all registered). Re-run if `.app` matters.
4. **Squat risk on the bare GitHub username.** `github.com/loadline` is dormant with zero repos since 2020. GitHub's name-release policy will not reclaim it for a live account, so plan on the suffixed org rather than hoping.
