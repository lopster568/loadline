# Measurement Methodology

| Field | Value |
| --- | --- |
| Methodology version | 0.1.1 (draft, not yet applied to a published release) |
| Date | 2026-08-17 |
| Status | Draft for review |
| Scope | Tier 1 static sweep. Tier 2 dynamic runs are specified separately. |

This document defines how numbers on this site are produced, so a third party with their own credentials can reproduce a published cell and a critic can locate the step they disagree with. Where a decision is a judgement call rather than a consequence of the protocol, the rejected alternative is stated.

Two labels appear on every published cell. **MEASURED**: the number came out of a harness run against a live server. **MODELED**: the number came out of a formula applied to measured inputs plus published third-party client behaviour.

---

## 1. Measurement procedure (Tier 1)

### 1.1 Server acquisition

Each server is pinned before the run: a released package version (npm, PyPI, container digest) for stdio servers, or the endpoint URL plus the self-reported `serverInfo.version` for remote servers. Pin, acquisition source, and timestamp go in the run record. Servers are installed from their published distribution channel, never from a working tree. `serverInfo` is self-reported and unverified by the protocol ([MCP 2026-07-28, Discovery](https://modelcontextprotocol.io/specification/2026-07-28/server/discover)), so remote servers with no package pin are marked unverified.

**OPEN:** whether container digests are required for all stdio servers in v1, or only for servers without a reproducible package version.

### 1.2 Transport handling

**stdio**: the harness launches a subprocess with a recorded environment and argument vector, with credentials in environment variables as the specification directs. **Remote (Streamable HTTP)**: requests go to the published endpoint using the server's documented authorization scheme, with auth scope and account tier published per row. The deprecated HTTP+SSE transport is unsupported; a server reachable only that way is a `protocol_error` failure.

### 1.3 Protocol revision handling

The harness speaks `2026-07-28` plus the handshake-based revisions `2025-06-18` and `2025-03-26`. The specification calls the first **modern** and the rest **legacy**, and they negotiate differently.

**Modern** has no handshake. Every request declares its version in the `_meta` key `io.modelcontextprotocol/protocolVersion`, and on Streamable HTTP in the `MCP-Protocol-Version` header. Servers must implement `server/discover`, which returns `supportedVersions`, `capabilities`, and `_meta['io.modelcontextprotocol/serverInfo']` in one request ([Versioning and Compatibility](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning)). **Legacy** sends `initialize` with a `protocolVersion`; the `InitializeResult` carries the selected version and `serverInfo`.

The harness always probes with `server/discover` first, then branches:

1. A `DiscoverResult` returns: modern server, so select the highest revision in both `supportedVersions` and the harness set.
2. An `UnsupportedProtocolVersionError` (`-32022`) returns: modern server, wrong version, so retry using `data.supported`.
3. Any other error or a timeout: treat as legacy and fall back to `initialize`, matching the specification's stdio backward-compatibility probe.

The revision used, the `supportedVersions` list where available, and the branch taken are all recorded. **Servers supporting several revisions are measured at the highest mutually supported one only**, because two revisions can yield two legitimate and different surfaces the calculator cannot present without confusing the reader.

**OPEN:** whether v1 publishes a second cell for servers whose surface differs materially across revisions, or leaves that to a season writeup.

### 1.4 Enumeration

The harness issues `tools/list` and follows `nextCursor` to exhaustion. A partial enumeration is a failure, not a partial number: per the partial-surface refusal commitment, a server whose full surface cannot be enumerated is refused a ranking rather than counted short. The specification states the tool set must not vary per connection but may vary by the authorization presented, which is the origin of the per-credential limitation in section 10.

### 1.5 Serialization of the tool surface

Token counts depend on how the surface becomes text, so serialization is fixed and versioned rather than left to each adapter. The **canonical serialization** is the JSON array of tool objects as received, normalized as follows: at tool level, the object is restricted to exactly eight fields, `name`, `title`, `description`, `inputSchema`, `outputSchema`, `annotations`, `icons`, `_meta`, and nothing is stripped from inside a retained field; key order as the server sent it; single-line, no insignificant whitespace. All three tokenizers count this same string, which is what makes the three columns comparable.

A second, Claude-only figure is recorded alongside: the count returned when the surface is mapped into Anthropic tool-definition shape and passed in the `tools` parameter of `count_tokens`, capturing provider-side serialization overhead the canonical string omits. It is a separate labelled column, never blended into the cross-provider number.

**OPEN:** whether the native `tools`-parameter count becomes the headline Claude figure in v1, at the cost of cross-provider comparability.

### 1.6 Token counting per provider

| Provider | Method | Pin |
| --- | --- | --- |
| Claude | `POST /v1/messages/count_tokens` | Model ID `claude-opus-5` |
| OpenAI | `tiktoken`, local | Encoding `o200k_base` |
| Gemini | `countTokens` | Model ID recorded per run |

All three are free at sweep volumes and no inference is performed. Counts are model-specific and move between releases: Anthropic's migration guidance states Claude Sonnet 5 produces approximately 30% more tokens than Claude Sonnet 4.6 for identical text. A count without its model stamp is meaningless.

**OPEN:** the exact Gemini model ID to pin, and confirmation that `o200k_base` is the correct encoding for the OpenAI model we intend to represent.

### 1.7 Cell stamping

Every published number carries: `model` · `tokenizer` · `measured_at` (UTC) · `protocol_revision` · `server_version` · `tools_list_sha256` · `methodology_version` · `harness_version`. A cell missing any stamp field is withheld from the release.

---

## 2. `$ref` resolution policy

SEP-2106, as shipped in the 2026-07-28 revision, permits tool schemas in JSON Schema 2020-12 (the default when no `$schema` is present) including `$ref` and `$defs`. Handling is therefore a methodology choice: two clients can legitimately put different amounts of text into context for one schema.

**Policy (v0):**

1. **Internal `$ref`s are resolved before counting.** Same-document pointers (`#/$defs/...`) are inlined. Recursive references are detected, left unresolved, and the tool is flagged.
2. **Network `$ref`s are never dereferenced.** The specification is explicit that implementations must not automatically dereference `$ref` values resolving to a network URI. The harness does not enable the opt-in fetching mode the specification permits.
3. **Unresolvable external `$ref`s are not counted permissively.** The specification says such schemas should be rejected rather than treated as permissive, so the tool is excluded and the server flagged; if the whole surface is affected, the server fails as `schema_invalid`.
4. **Resolution is bounded** in depth, subschema count, and time, per the specification's denial-of-service guidance. Exceeding a bound flags the tool.

**Why inline.** It approximates what a naive client serializes into context: a self-contained schema, where a `$defs` block referenced from three properties is paid for once on the wire but read three times by the reader.

**Rejected alternative: count the wire form as received.** Accurate for a client that passes raw schemas through, but it rewards a cosmetic transformation: an author can factor a repeated sub-object into `$defs` and cut their published number without changing what any model reads. That is the Goodhart failure this scoreboard exists to avoid.

The policy is versioned. Changing any of the four rules is a MINOR bump at minimum, with a before/after count for at least one affected server in the changelog.

**OPEN:** whether both the inlined and wire-form counts should be published, so readers can see the delta a server's `$defs` factoring produces.

---

## 3. Client-mode modeling

A single cost number is a wrong number, because cost depends on what the client does with the surface. Three modes are always reported, never collapsed into one.

### 3.1 Naive full-load (MEASURED)

All tool definitions serialized into context at session start. Cost equals the canonical-serialization count from 1.5. Reference client: Gemini CLI.

**Tokenizer basis.** The mode numbers throughout this section, naive, progressive, and code, are all computed on the `o200k_base` count of the canonical serialization from 1.6, the only tokenizer the harness can run offline and therefore the only one guaranteed present on every row. The Claude and Gemini columns are per-provider counts of that same canonical string, published alongside for comparison, but they do not feed the mode formulas below.

**Serialization assumption:** each client emits the full tool list in its provider's native tool-definition format, approximated by the canonical serialization. This is an assumption, not an audit: clients differ in whether they include `title`, `annotations`, and `outputSchema`, and that moves the number.

**OPEN:** a per-client serialization audit is not done for v0. Until it is, the column is labelled "naive client, canonical serialization" rather than named after any specific client.

### 3.2 Tool search / progressive disclosure (MEASURED inputs, MODELED total)

The client loads a small search facility upfront and pulls schemas on demand. Anthropic ships this as the tool-search tool with `defer_loading`, and has published a reduction from 72k to 8.7k tokens for a 50-plus-tool surface.

```
C_progressive = C_stub + sum(C_tool_i for i in retrieved_k)
```

- `C_stub`: upfront search facility plus per-tool routing entries. MEASURED where the stub format is published, otherwise MODELED at the published figure.
- `C_tool_i`: per-tool loaded cost. MEASURED, and this project's headline metric.
- `k`: 3 to 5 tools per session. Both bounds are reported, giving a range rather than a point.

**Retrieval simulation.** Which tools land in the k is simulated with BM25 over each tool's `name`, `title`, and `description` concatenated, indexed over the server's own surface, with BM25 parameters pinned and recorded. The composite total is MODELED because `k` is an assumption; the per-tool costs inside it are MEASURED.

**OPEN:** the BM25 retrieval simulation described above is specified but not yet implemented in the Tier 1 harness. `PerToolAvg` is reported as a mean over all measured tool costs rather than over a simulated retrieval set until it lands.

### 3.3 Code mode (MODELED)

The client expresses the whole API as a compact programmatic interface rather than as tool schemas, at roughly 1k tokens for a full surface. **Every code-mode figure carries a "modeled, not measured" label until Tier 2 validates it against real call traffic**, removed per server and only once a Tier 2 run for that server exists.

**Clamp.** The flat published-behaviour estimate (~1k tokens) is capped at the server's own naive count from 3.1: a surface whose full schema set already serializes to fewer tokens than the flat estimate is reported at its own naive count instead. This is the one harness addition to the published behaviour, and it only binds on surfaces already smaller than the flat estimate. Its purpose is narrow: code mode must never be reported as costing more than the schemas it replaces. The label stays "modeled" whether or not the clamp binds.

### 3.4 The modes-null contract

Every mode figure in this section is computed on the o200k_base count of the canonical serialization (1.6, 3.1). That count is itself a measurement that can fail: the total count over the canonical string can fail, or the per-tool count needed for the progressive-disclosure mean (3.2) can fail on any single tool in the surface. Either failure withholds the whole modes block rather than publishing a partial one. A per-tool mean taken over whatever subset of tools happened to count successfully is not a measurement of the surface; it is a measurement of which requests happened to succeed, and publishing it would misrepresent an arbitrary prefix as the whole. When the total or any per-tool count is unavailable, the row publishes `"modes": null`.

**Consumers must treat a null modes block as "pending measurement," never as a measured zero.** A cost ranking, chart, or aggregate that coerces a null into 0 rewards a server for failing to enumerate over succeeding at a high count, which is the same Goodhart failure section 2 rejects for `$ref` counting.

---

## 4. Dollar reporting

**Tokens are primary.** Dollars are secondary and never published as a single figure. Tool definitions are a near-perfect cache prefix: prompt caching is a prefix match and the render order is tools, then system, then messages, so the surface sits at the front of context and is byte-stable across turns. A single dollar figure is the cold-start figure presented as if it recurs, overstating steady-state cost by roughly an order of magnitude.

Every dollar figure is a pair:

- **Cold write**: `tokens x input_price x 1.25` (cache-write multiplier at 5-minute TTL).
- **Cache read**: `tokens x input_price x 0.1`.

Stated cache assumptions:

1. The tool surface is the cache prefix, written once per cache lifetime.
2. Read multiplier 0.1x base input; write multiplier 1.25x at 5-minute TTL. The 1-hour TTL writes at 2x and is not modelled in v0.
3. **Surfaces below the provider's minimum cacheable prefix do not cache at all.** For Claude that minimum is model-dependent (512 tokens on Claude Opus 5, 1024 on several others). A server falling below it cannot achieve the cache-read figure, and the cell is annotated.
4. Any change to the tool list invalidates the prefix, so a mid-session tool change pays the cold write again.

The per-provider price table is referenced by date, never inlined into cells. A price change is a changelog event, never a silent restatement of a prior release's dollars.

**OPEN:** dollar-pair computation as specified above is not yet implemented in the Tier 1 harness. The published site computes and displays the cold-write/cache-read dollar pairs from its own `pricing.json` in the interim, independent of this harness.

---

## 5. Retrievability metric v0

Purpose: detect servers that are cheap because their descriptions are gutted, the failure mode a cost-only ranking would pay for.

**Procedure.** For each tool, three task-phrase queries are authored (short, imperative, phrased as a user goal rather than as the tool name) and run against a BM25 index over `name` + `title` + `description` for the whole surface. A query scores 1 if its own tool ranks in the top k.

**Score.** `retrievability = correct_hits / total_queries`, reported at k=5 on a 0 to 100 scale, always alongside the absolute counts.

**Known limitations.** Queries authored from a tool's own description reward description-rich servers for partly circular reasons, inflating agreement with the hygiene grade. Real clients search a merged corpus across every attached server where cross-server name collisions dominate; v0 measures the easier single-server problem. BM25 is a proxy: no named client is guaranteed to use it, and regex variants behave differently. Query authorship is itself a bias surface.

**OPEN:** who authors the task-phrase queries and by what documented procedure. The current plan (harness author writes them) is not defensible for a public ranking.

**OPEN:** retrievability scoring as specified above is not yet implemented in the Tier 1 harness.

---

## 6. Schema hygiene grade v0

A weighted rubric over six mechanical dimensions, computed per server from the enumerated surface.

| # | Dimension | Check | Weight |
| --- | --- | --- | --- |
| 1 | Description presence | Fraction of tools with a non-empty `description` | 25 |
| 2 | Description adequacy | Fraction of descriptions within a length band and containing a when-to-use signal | 20 |
| 3 | Parameter descriptions | Fraction of `inputSchema` properties with a non-empty `description` | 20 |
| 4 | Enum documentation | Fraction of enum-typed parameters whose description accounts for the allowed values | 10 |
| 5 | Naming clarity | Conformance to the specification's tool-name guidance (1 to 128 characters, restricted charset, unique within server) plus internal naming consistency | 10 |
| 6 | Disambiguation | Absence of tool pairs above a similarity threshold on name plus description without a stated boundary | 15 |

The weighted score maps to a letter grade. Letter and all six sub-scores are published, because a single letter hides which dimension is failing.

**Prior art.** [Glama](https://glama.ai) publishes a schema-quality score composed of Tool Definition Quality (70%) and Server Coherence (30%), verified live 2026-08-16. Ours differs in one respect that matters: Glama's score stands alone, while this grade sits beside the cost figure for the same server and run. A standalone score does not constrain a cost ranking; a grade in the same row as the token count means an author cannot cut their published cost by deleting descriptions without the deletion showing up alongside it.

**OPEN:** dimension 2's when-to-use signal is keyword-detected, which is weak and gameable. A better mechanical proxy, or an explicit decision to accept and document the weakness, is required before v1.

**OPEN:** the dimension 6 similarity threshold is unset and needs calibration against the corpus before the grade is published.

**OPEN:** a stable citation anchor for the Glama 70/30 split. The figure comes from a live check on 2026-08-16.

**OPEN:** the schema hygiene grade as specified above is not yet implemented in the Tier 1 harness.

---

## 7. Failure policy

Failed servers publish as data points, and a failure never blocks a release.

| Class | Meaning |
| --- | --- |
| `unreachable` | Process did not start, or endpoint did not respond |
| `auth` | Reachable, but no credential sufficient to enumerate |
| `protocol_error` | Reachable and authenticated, but the exchange failed (unsupported transport, malformed responses, no mutually supported revision) |
| `timeout` | Exceeded the per-step budget recorded in the harness config |
| `partial_surface` | Enumeration started but could not complete; refused a ranking rather than counted short |
| `schema_invalid` | Surface enumerated, but schemas could not be resolved under section 2 |

Each failure row carries the same stamp fields as a successful row, minus token counts, plus the failure class and raw error. Server rot is part of the subject matter, so it is part of the dataset.

**Operator aborts.** An abort is a property of the run, not a failure class of any server: the classes above are all server-attributable, and a server the sweep never reached, or was cut short before finishing, earned no classification. An aborted run publishes rows only for the servers that completed before the interrupt; the servers left unswept carry no row at all, because publishing a failure row for a server the operator merely didn't reach would assert something the run never measured. The run document carries `"aborted": true`. If the interrupt lands before any server completes, there is nothing honest to publish: no artifacts are written, and the prior release is left in place.

---

## 8. Provenance and anti-gaming

Every run records three hashes:

- `tools_list_sha256`: SHA-256 over the raw `tools/list` response bytes as received, concatenated across pages. Detects any change including tool ordering, which matters because ordering changes break prompt caching.
- `canonical_sha256`: SHA-256 over the canonical serialization of 1.5, key order and tool order exactly as the server sent them. This is the token-counting basis; every published count in sections 1.6 and 3 traces back to this digest, so it must not tolerate reordering.
- `canonical_sorted_sha256`: SHA-256 over the same canonical serialization with object keys sorted and the tools array reordered by `name`. Detects semantic change while tolerating cosmetic reordering, the property section 8 originally asked of a single hash.

Section 1.5 needs an order-preserving digest because token counts depend on key order; this section needs an order-tolerant one to separate a real surface change from a server that merely re-emitted its fields in a different sequence. One hash cannot serve both, so both are published side by side rather than picking a winner. All three hashes and the pinned server version are published per row. A number whose hash does not match the published artifact is a defect.

**Benchmark-detecting builds.** The harness sends an honest `io.modelcontextprotocol/clientInfo`, which makes it identifiable. The specification says the tool set must not vary per connection, and that `clientInfo` is self-reported, unverified, and should not change server behaviour. A server varying its surface by client identity is therefore violating guidance rather than exploiting a loophole, and it is detectable: for a sampled subset, the harness runs a second pass under a generic identity and compares both hashes, publishing any divergence. It does not spoof identity by default, which would trade a governance commitment for a marginal detection gain.

**OPEN:** the size of the sampled subset, and whether a divergence downgrades the row or only annotates it.

---

## 9. Versioning

The methodology carries a semantic version, stamped on every cell.

- **MAJOR**: makes results incomparable to prior releases (changing the `$ref` policy, the canonical serialization, or which figure is headline). Results are comparable only within a major version; trend lines never cross a major boundary.
- **MINOR**: a new metric, failure class, or column; prior cells stay valid.
- **PATCH**: corrections with no effect on any number.

The changelog separates three delta types, because a reader who cannot tell them apart cannot trust a trend line:

1. **Server changed.** New `canonical_sha256` at the same methodology and harness version.
2. **Tokenizer or model changed.** Same hash, different count. Nothing about the server moved.
3. **Harness or methodology changed.** Same hash, same model, different count. We moved.

Every release classifies each delta into one of the three; a release with unclassified deltas does not ship. Results are never compared across harness versions.

---

## 10. Known limitations

1. **Per-credential tool surfaces.** The specification permits the tool set to vary by the authorization presented, so a server exposing more tools on a paid tier publishes a different number under our credential than under yours. Auth scope and tier are published per row, but this is the largest source of legitimate disagreement with our figures.
2. **Tokenizer drift.** Counts shift between model releases by amounts large enough to swamp real server changes. Stamping and delta separation contain that without removing it: two cells with different model stamps answer different questions.
3. **Modeled modes are not measured.** Code mode is entirely modeled in v0; progressive disclosure has measured inputs and a modeled total because `k` is an assumption. Neither should be cited as a measurement.
4. **Client serialization is unaudited** (see 3.1). The naive column approximates client behaviour rather than reproducing any specific client.
5. **Task-mix dependence.** Tier 2 measures what one chosen set of scripted tasks costs; a different mix produces different per-call flows for the same server. Suites are published, but none is neutral.
6. **Small Tier 2 n.** Tier 2 covers 3 servers: enough to keep Tier 1 honest on a capability axis, not enough to generalize. Findings are absolute counts against named servers, never rates.
7. **Retrievability circularity** (see section 5).
8. **Hygiene weights are unvalidated.** The six weights are a first draft, uncalibrated against any outcome measure. A server can score well on all six and still be hard for a model to use.
9. **Static enumeration is not usage.** Tier 1 measures what a server puts in front of a model before any work happens, not whether the tools work.
10. **Selection.** The corpus is about 15 curated servers chosen under a published selection rule. Coverage is not a claim this project makes.

---

## Changelog

### 0.1.1, 2026-08-17

The Tier 1 harness is now built. Bringing this document into agreement with what it actually implements surfaced the following, all PATCH-level: no published number changes as a result of these corrections, only the text describing it.

1. **Section 1.5 / section 8 key-order conflict resolved.** Section 1.5 fixes canonical key order as-sent because token counts depend on it; section 8 wanted a digest that tolerates cosmetic reordering. The harness ships two digests rather than choosing one: `canonical_sha256` (order-preserving, the token-counting basis) and `canonical_sorted_sha256` (key-sorted, tools array sorted by name, delivers section 8's reorder-tolerant detection). Section 8 and section 9's delta classification were amended to name both and state their separate roles. **This is a provisional ruling, pending operator ratification before v1.**
2. **Section 1.5 allowlist self-contradiction fixed.** The prior text restricted the tool object to six fields, then separately said `icons`, `_meta`, and a nonexistent `x-mcp-header` annotation were retained. The canonicalizer implements a single eight-field allow list (`name`, `title`, `description`, `inputSchema`, `outputSchema`, `annotations`, `icons`, `_meta`) with nothing stripped inside a retained field; the section now states that directly and drops the `x-mcp-header` reference, which does not exist in the implementation.
3. **`partial_surface` consistency confirmed.** Section 7 is the only status enumeration in this document; no other status list exists to fall out of step with it. No text changed.
4. **Section 3.1 tokenizer basis stated explicitly.** Added a paragraph naming `o200k_base` as the tokenizer the mode formulas in section 3 are computed on, since it is the only tokenizer the harness runs offline; the Claude and Gemini columns are per-provider counts of the same canonical string and do not feed the mode formulas.
5. **Section 3.3 clamp documented.** The flat ~1k-token code-mode estimate is capped at the server's own naive count, so code mode is never reported as costing more than the schemas it replaces. Label remains "modeled" whether or not the clamp binds.
6. **Implementation-gap OPEN items added.** BM25 retrieval simulation (3.2), retrievability scoring (5), schema hygiene grade (6), and dollar-pair computation (4) are specified in this document but not yet implemented in the Tier 1 harness. The published site computes and displays dollar pairs from its own `pricing.json` independently of the harness in the interim.
7. **Section 7 operator-abort behaviour documented.** The harness treats an abort as a property of the run rather than a failure class of any server: an aborted run publishes rows only for the servers that completed, the unswept servers carry no row, and the run document carries `"aborted": true`. If the interrupt lands before any server completes, no artifacts are written and the prior release is left in place. This was already the implemented behaviour; the section now states it.
8. **Section 3.4 modes-null contract added.** When the o200k_base count that the mode formulas run on is unavailable, whether the total count failed or any single per-tool count failed, the row publishes `"modes": null` rather than a mean over a partial, arbitrary subset of the surface. Consumers must read a null modes block as "pending measurement," never as a measured zero. This was already the implemented behaviour; the section now states it.
