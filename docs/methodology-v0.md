# Measurement Methodology

| Field | Value |
| --- | --- |
| Methodology version | 0.3.1 (current; applies to runs from 2026-08-20). PATCH: sections 3.2 and 3.3 state what would move a mode label; no figure moves. 0.3.0 was the MINOR before it: every acquisition records the dependency set the resolver produced (section 1.1). Rows published before 0.3.0 carry the stamp they were produced under and do not carry the new field. |
| Date | 2026-08-20 |
| Status | Draft for review |
| Scope | Tier 1 static sweep. Tier 2 dynamic runs are specified separately. |

This document defines how numbers on this site are produced, so a third party with their own credentials can reproduce a published cell and a critic can locate the step they disagree with. Where a decision is a judgement call rather than a consequence of the protocol, the rejected alternative is stated.

Two labels appear on every published cell. **MEASURED**: the number came out of a harness run against a live server. **MODELED**: the number came out of a formula applied to measured inputs plus published third-party client behaviour.

---

## 1. Measurement procedure (Tier 1)

### 1.1 Server acquisition

Each server is pinned before the run: a released package version (npm, PyPI, container digest) for stdio servers, or the endpoint URL plus the self-reported `serverInfo.version` for remote servers. Pin, acquisition source, resolved dependency set, and timestamp go in the run record. Servers are installed from their published distribution channel, never from a working tree. `serverInfo` is self-reported and unverified by the protocol ([MCP 2026-07-28, Discovery](https://modelcontextprotocol.io/specification/2026-07-28/server/discover)), so remote servers with no package pin are marked unverified.

**Resolved dependency versions.** The pin covers the server package and nothing beneath it. A server declaring an unbounded dependency, `mcp>=1.1.3` for instance, gets whichever version the resolver returns at acquisition time, so two acquisitions an hour apart can import different code while `acquisition.pinned` reads the same on both. Every acquisition therefore records what the resolver actually produced, in `acquisition.resolved_deps`, by kind:

| Kind | `method` | How the set is read |
| --- | --- | --- |
| PyPI, uvx | `uvx_importlib_metadata` | `uvx --from <spec> [--with <constraint> ...] python -c <importlib.metadata listing>` |
| npm, npx | `npx_node_modules_walk` | `npx -y -p <spec> node -e <node_modules walk>` |
| Container image | `container_image` | Nothing is read: the image ships its resolved dependencies in its own layers |
| Remote, binary | `not_applicable` | Nothing is resolved locally at acquisition time |

The exact argument vector is recorded in `resolved_deps.command`, so the listing is rerunnable rather than described.

**The listing reads the environment the server runs from, not a second resolve.** For both package runners the requirement set given to the listing is the launch's own set: the same package spec, the same `--with` constraints. uv keys its cached environment on that set and npx keys its cache directory on the package spec, so the listing resolves into the same place the launch is then started from. The listing runs first for that reason: it performs the resolve, and the launch reuses it. `resolved_deps.env` records the environment path, `sys.prefix` for uvx and the npx cache directory for npx, which is the same path a traceback out of the failing server cites, so a reader can confirm the list and the failure came from one environment.

**What is published per row.** The full resolved set as name and version pairs, plus a convenience `sdk` entry lifted out of it: `mcp` on PyPI, `@modelcontextprotocol/sdk` on npm. The SDK gets its own field because it is the version a reader of an `unreachable` row looks for first, and scanning a 130-entry list for it is not a reading experience an audit should require.

**A failed listing is data, never a failed acquisition.** The listing is instrumentation on the acquisition and observes it; it cannot fail it. A listing that will not run, or output that will not parse, publishes the error in `resolved_deps.error` and the server is measured exactly as it otherwise would be. Where there is nothing to list, the field records the reason in `resolved_deps.note` rather than a bare null, because a null says a listing is absent without saying why it was never possible.

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

**What would move this label, and why Tier 2 does not.** The total converts to MEASURED only when two things are read off a client that is actually running progressive disclosure: a per-session figure that isolates this mode's tool-definition footprint, and the `k` that client retrieved. Neither is available. The Tier 2 interposer measures traffic on the MCP wire and cannot see what a client injected into the model's context as tool definitions (`tier2-task-suites.md` 4.3), and neither client the harness drives exposes a usable substitute: Claude Code reports `usage.cache_creation_input_tokens` and `usage.cache_read_input_tokens` for the whole session, and Gemini CLI reports `stats.models.<id>.tokens.prompt` and `.cached`, and all four contain the system prompt and the conversation as well as the tool definitions. `C_stub` is likewise still the published flat figure and not a measurement: the stub format is not published, which is the branch of the rule above that reads MODELED. The 90-trial run of 2026-08-18 therefore leaves this label exactly where it was.

**OPEN:** the BM25 retrieval simulation described above is still not wired into this mode. The BM25 index itself now exists in the harness and is published for retrievability (section 5), but `PerToolAvg` here remains a mean over all measured tool costs rather than over a simulated retrieval set. Wiring the two together requires deciding which query set drives the simulation, and the derived queries of section 5 are same-source by construction, so they would bias the retrieval set toward the tools that describe themselves best rather than toward the tools a session actually needs.

### 3.3 Code mode (MODELED)

The client expresses the whole API as a compact programmatic interface rather than as tool schemas, at roughly 1k tokens for a full surface. **Every code-mode figure carries a "modeled, not measured" label, and the label comes off a server only when a run measures that server's code-mode context footprint directly.**

**The label rule, corrected in 0.3.1.** Until 0.3.1 this paragraph made the condition the existence of a Tier 2 run for the server. That is the wrong condition and it became live on 2026-08-18, when Tier 2 ran all three of its servers for the first full 90-trial suite. That run measures no code-mode figure at all, for two independent reasons: neither client it drives offers a code mode, so no trial was ever in one, and the interposer sees MCP wire traffic rather than model context (`tier2-task-suites.md` 4.3), so it could not have read the footprint even from a client that did. Read literally, the old rule would have licensed relabelling `filesystem`, `playwright` and `github` as MEASURED on the strength of data that says nothing about code mode. No published figure ever moved under it, because the rule was corrected before a run could trigger it.

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

## 5. Retrievability metric

Purpose: under a tool-search client, a tool that cannot be found by a task phrase is effectively broken however cheap its schema is. This metric detects servers that are cheap because their descriptions are gutted, the failure mode a cost-only ranking would otherwise pay for.

Both figures are MEASURED. They are computed from the surface the sweep already enumerated, so they cost no further contact with the server, and no assumed parameter enters them.

### 5.1 Query derivation

Queries are derived mechanically from each tool's own description. **Rejected alternative: the harness author writes task phrases per tool.** That was the v0.1 plan and it is not defensible for a public ranking, because the queries would be the ranking author's discretion applied privately to each server. A mechanical rule can be reproduced by a critic; a judged one cannot.

For each tool, from its `description` (or its `title` when the description is empty):

1. **Tokenize.** Lowercase, split at every non-alphanumeric rune and at snake_case and camelCase boundaries, drop single-character tokens. No stemming: a stemmer is another dictionary to pin and version, and the queries come from the same vocabulary as the documents they are scored against.
2. **Strip stopwords**, from a short generic published list. A longer list tuned against this corpus would be a parameter fitted to the servers being measured.
3. **Strip the tool's own name tokens**, including their snake and camel fragments. Without this step a description that repeats the tool name retrieves itself on the name field alone, and the metric would measure only whether the author wrote the name twice.
4. **Form N = 3 queries** from the surviving content words, in order:
   - `leading_phrase`: the first three content words. MCP descriptions conventionally open with an imperative verb, so this approximates the leading verb phrase without requiring a part-of-speech tagger.
   - `distinctive_bigram`: the adjacent content-word pair with the highest summed IDF over this server's own surface, ties going to the earliest pair. Adjacency is measured after stopword removal.
   - `content_words`: the deduplicated content-word set in first-occurrence order, capped at 12 terms so a long description does not turn its query into a copy of the document.

Forms that collapse to the same string on a short description are deduplicated, so a tool can yield fewer than three distinct queries. A tool with no derivable content word yields none, and is scored as not retrievable rather than dropped from the denominator: a gutted description is the failure this metric exists to catch, so excluding it would reward it.

The derivation is deterministic. **The full derived query set is published per server**, at `data/runs/<date>/<server>-queries.json`, with each query's form, terms, and the rank it achieved. Mechanical derivation is only an improvement on hand-written queries if the output is visible.

### 5.2 Scoring

Documents are each tool's `name` + `title` + `description`, indexed over the same server's surface and nothing wider. That join is the canonicalizer's `descriptor` from 1.5, consumed rather than rebuilt, so the document this metric indexes and the token base of dimension 6 in section 6 are the same string by construction and not by two implementations agreeing. Scoring is BM25 with `k1 = 1.2` and `b = 0.75`, pinned and stamped into the query artifact, with IDF in the non-negative form `ln(1 + (N - df + 0.5) / (df + 0.5))`.

A document scoring zero on a query is not ranked at all. Padding the tail with non-matches would let a tool on a small surface count as top-3 purely because there was nothing else in the list. Ties break on tool name, so a ranking is stable across runs.

### 5.3 Published figures

- **`top3_fraction`** is the published score: the fraction of tools that at least one of their own derived queries ranks in the top 3 of their own server's surface.
- **`mrr`** is secondary: the mean reciprocal rank over every derived query on the surface, counting an unmatched query as zero rather than dropping it. On a surface of three or fewer tools the top-3 fraction is close to vacuous, and MRR is the figure that still separates a tool ranking first from one ranking third.
- **`queries_per_tool`** records N = 3.

A row that was never enumerated publishes `"retrievability": null`. Consumers must read that as pending measurement, never as a measured zero, on the same terms as the modes-null contract of 3.4.

### 5.4 Known limitations

1. **Same-source derivation.** Deriving queries from a tool's own description biases toward tools that are retrievable by their own words. What this metric genuinely measures is **within-server disambiguation**, whether sibling tools shadow each other, and not whether real user phrasing finds the tool. It is not a model of a real user, and it should not be read as one.
2. **Circularity with the hygiene grade.** Because both are computed from the same descriptor text, a description-rich server scores well on both for partly shared reasons. The two numbers are not independent evidence.
3. **Single-server corpus.** Real clients search a merged corpus across every attached server, where cross-server name collisions dominate. This measures the easier single-server problem.
4. **BM25 is a proxy.** No named client is guaranteed to use it, and regex or embedding variants behave differently.
5. **Small surfaces.** With three or fewer tools, top-3 is satisfied by any match at all. Read MRR on those rows.

**OPEN (resolved, ratified).** Query authorship is now the mechanical derivation of 5.1 rather than the harness author. **This ruling is ratified 2026-08-18**, on the same footing as the digest ruling in the 0.1.1 changelog.

---

## 6. Schema hygiene grade

Six mechanical dimensions, each scored 0 to 100 and each published individually, computed per server from the enumerated surface. The grade is a letter on their **unweighted mean**.

| # | Dimension | Field | Check |
| --- | --- | --- | --- |
| 1 | Description presence and adequacy | `description_adequacy` | Per tool, the mean of three checks: a non-empty `description`; length in 20 to 1000 characters; sentence-like, defined as four or more whitespace-separated words opening on a letter |
| 2 | When-to-use signal | `when_to_use_signal` | Fraction of descriptions containing a phrase from the published keyword list that states when to reach for this tool rather than a sibling |
| 3 | Parameter descriptions | `parameter_descriptions` | Fraction of `inputSchema` properties, at every depth, carrying a non-empty `description` |
| 4 | Enum documentation | `enum_documentation` | Fraction of enum-typed properties whose own description **names** at least one allowed value, or whose values are all self-evident (three or more characters, opening on a letter, identifier charset) |
| 5 | Naming clarity | `naming_clarity` | Mean of three checks over the tool names: conformance to the specification's name rule (1 to 128 characters drawn from `[a-zA-Z0-9_-]`, unique within the server); the share of names in the surface's dominant casing style; the share of names free of single-letter or bare-numeric fragments |
| 6 | Disambiguation | `disambiguation` | Fraction of tools **not** involved in any sibling pair at or above the similarity threshold, by token Jaccard over name, title, and description with stopwords removed |

**Grade bands:** A at 90 and above, B at 75, C at 60, D at 40, F below 40.

**"Names a value" in dimension 4 is a whole-token match, not a substring one.** The description is tokenized with the same splitter as section 5.1, with short fragments kept, and a value counts as named only when every one of its own tokens appears as a token of the description. A substring test scored the shortest and most cryptic enums as documented by accident: `ro` sits inside "zero", `in` inside "within", `id` inside "valid", so exactly the values this dimension exists to catch were the easiest to pass. Stopwords are not stripped here, unlike in query derivation, because an enum value of `in` or `not` is still a value the description can legitimately name.

**Dimension 3 counts the tuple form of `items`.** The schema walk follows `items` whether it holds a single schema (draft 2020-12) or an array of positional schemas (draft-07). A tuple-form array is full of properties and enums that cost the same context as any other, so skipping it would let a schema hide its parameters behind a draft choice.

**Vacuous dimensions score 100, not 0.** A surface declaring no properties scores 100 on dimension 3, a surface with no enums scores 100 on dimension 4, and a one-tool surface scores 100 on dimension 6. There is nothing undocumented and nothing to confuse; penalising the absence would reward adding parameters, enums, and siblings that the server does not need.

**A surface with no tools at all is graded null, not vacuously.** The vacuous-100 rule above applies within a surface that exists. A surface with zero tools has no dimension to score at all: dimensions 3, 4 and 6 would return their vacuous 100 while dimensions 1, 2 and 5 return 0 over an empty denominator, and the unweighted mean of that mixture is 50, which prints as a measured grade of D. A server that put no tools in front of a model has not earned a D, or any other letter. The harness returns no grade for an empty surface and the row publishes `"hygiene": null`.

**Dimension 6 is scored on tools, not pairs.** Pair counts are quadratic in surface size, so one bad pair on a 24-tool server would cost a fraction of what the same defect costs a 4-tool server. The flagged pairs and the threshold in force are both published on the row as `similar_pairs` and `similarity_threshold`, so the dimension can be recomputed without reading the harness source.

**Similarity threshold: 0.60.** Calibrated against the three measured servers; see the 0.2.0 changelog for the pair distribution and the legitimate pair it was set to clear.

**Why unweighted.** The v0.1 draft carried weights of 25/20/20/10/10/15. They are dropped. Publishing weights asserts a calibration against a usability outcome that this project has not run, and known limitation 8 already says the weights were a first draft fitted to nothing. An unweighted mean makes the same claim honestly: six dimensions we can measure, none of which we have evidence to rank above another. Restoring weights requires an outcome measure to fit them to, and is a MINOR bump with a before/after grade for every server in the changelog.

Every dimension is mechanical and therefore gameable by an author who reads the harness source. That is the trade: a mechanical rule can be reproduced by a critic and a judged one cannot, and a server that games one dimension buys one sixth of the grade and leaves the other five unmoved.

A row that was never enumerated, or that enumerated an empty surface, publishes `"hygiene": null`, on the same terms as 3.4 and 5.3.

**Prior art.** [Glama](https://glama.ai) publishes a schema-quality score composed of Tool Definition Quality (70%) and Server Coherence (30%), verified live 2026-08-16. Ours differs in one respect that matters: Glama's score stands alone, while this grade sits beside the cost figure for the same server and run. A standalone score does not constrain a cost ranking; a grade in the same row as the token count means an author cannot cut their published cost by deleting descriptions without the deletion showing up alongside it.

**Accepted weakness: dimension 2.** The when-to-use signal is keyword-detected, and the v0.1 draft flagged that as weak and gameable. The weakness is accepted and documented rather than fixed, which is the second of the two options the OPEN item allowed. Keyword detection cannot tell a real selection boundary from the phrase "use this for" pasted into every description, and it produces false negatives on real surfaces: playwright scores 0 on this dimension even though `browser_take_screenshot` says "use browser_snapshot for actions" and `browser_network_requests` names its sibling directly, because neither phrasing is on the list. The dimension is kept because its failure direction is safe. A server stating no boundary anywhere reliably scores zero, which is a true finding, and the keyword list is generic and published rather than fitted to the measured corpus. Replacing it with a stronger mechanical proxy, most likely detecting cross-references to sibling tool names on the same surface, is deferred to v1.

**OPEN:** a stable citation anchor for the Glama 70/30 split. The figure comes from a live check on 2026-08-16.

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

**A new metric does not break comparability.** Adding a column is a MINOR bump precisely because prior cells stay valid: a 0.1.x row and a 0.2.x row still answer the same question about every figure they share, and the token counts, hashes, and mode figures are directly comparable across the boundary. What a 0.1.x row lacks is the `retrievability` and `hygiene` fields entirely. A consumer must treat a missing field on an older row as "this release did not compute it", which is the same reading as a null on a current row and is not the same as a zero.

---

## 10. Known limitations

1. **Per-credential tool surfaces.** The specification permits the tool set to vary by the authorization presented, so a server exposing more tools on a paid tier publishes a different number under our credential than under yours. Auth scope and tier are published per row, but this is the largest source of legitimate disagreement with our figures.
2. **Tokenizer drift.** Counts shift between model releases by amounts large enough to swamp real server changes. Stamping and delta separation contain that without removing it: two cells with different model stamps answer different questions.
3. **Modeled modes are not measured.** Code mode is entirely modeled in v0; progressive disclosure has measured inputs and a modeled total because `k` is an assumption. Neither should be cited as a measurement. The 90-trial Tier 2 run of 2026-08-18 does not change this. It measures MCP wire traffic, per-trial session cost and task outcomes, none of which is a per-mode context footprint; see 3.2 and 3.3 for what would.
4. **Client serialization is unaudited** (see 3.1). The naive column approximates client behaviour rather than reproducing any specific client.
5. **Task-mix dependence.** Tier 2 measures what one chosen set of scripted tasks costs; a different mix produces different per-call flows for the same server. Suites are published, but none is neutral.
6. **Small Tier 2 n.** Tier 2 covers 3 servers: enough to keep Tier 1 honest on a capability axis, not enough to generalize. Findings are absolute counts against named servers, never rates.
7. **Retrievability measures the easier problem** (see 5.4). Queries are derived from each tool's own description, so the metric reports within-server disambiguation, not real-user phrasing, and it is not independent of the hygiene grade.
8. **Hygiene dimensions are unranked.** The six dimensions are averaged unweighted because no outcome measure exists to weight them against (see section 6). A server can score well on all six and still be hard for a model to use, and the mean asserts only that six measurable things were measured.
9. **Static enumeration is not usage.** Tier 1 measures what a server puts in front of a model before any work happens, not whether the tools work.
10. **Selection.** The corpus is about 15 curated servers chosen under a published selection rule. Coverage is not a claim this project makes.
11. **The dependency listing is a second process, not a readout from the server.** Section 1.1's listing resolves the launch's requirement set into the same package-runner cache the launch reuses, and records the environment path so the two can be checked against each other, but it does not read the metadata out of the running server process. A resolver answering differently between the listing and the launch, on a cache eviction or an index change inside that window, would put the two out of step; the recorded environment path is what would show it.

---

## Changelog

### 0.3.1, 2026-08-20

**PATCH. Sections 3.2 and 3.3 state what would move a mode label. No figure moves and no cell is republished.** The published rows keep the stamp they were produced under, which is correct: a PATCH leaves them comparable by definition (section 9). This entry was drafted against 0.2.0 and lands as a PATCH on 0.3.0, because 0.3.0 shipped while it was in flight; nothing in it depends on which of the two it sits on.

The first full Tier 2 run landed on 2026-08-18: 90 trials, suite 1.0.1, three servers by two clients by fifteen tasks by three trials, 87 successes, no version drift and no `harness_suspect` trial. It was read against sections 3.2 and 3.3 to see what it converts from MODELED to MEASURED. The answer is nothing, and finding that out exposed a defect in how 3.3 stated its own rule.

1. **3.3's label rule was keyed on the wrong thing.** It removed the code-mode label "per server and only once a Tier 2 run for that server exists". A Tier 2 run for all three servers now exists and measures nothing about code mode, so the rule as written would have licensed three MEASURED relabels on the strength of unrelated data. The condition is now the measurement, not the run. Nothing moved under the old rule; it is corrected before anything could.
2. **3.2 now names the two observables the conversion needs** and records that neither client exposes either one. Claude Code's and Gemini CLI's usage blocks both report session-wide input and cache totals that contain the system prompt and the conversation, so neither isolates a tool-definition footprint, and the interposer cannot supply it because it watches the MCP wire rather than model context (`tier2-task-suites.md` 4.3).
3. **Known limitation 3 records the same finding**, so a reader who only reads the limitations does not come away thinking Tier 2 settled it.

**What Tier 2 does publish, and where.** The one Tier 2 quantity that is unambiguously measured and attributable to a named server is call traffic: the arguments of every `tools/call` request plus the results the server returned, counted with `o200k_base` so it shares this document's token basis. It is published in `data/tier2-published.json` and shown on the site as its own block, never summed into a stack total, because wire traffic and tool-surface footprint are different costs. That figure is specified by `tier2-task-suites.md` section 4, not by this document, so it is not a new metric under section 9 here and does not carry this version past a PATCH.

### 0.3.0, 2026-08-20

**MINOR. Every acquisition now records the dependency set the resolver produced, in `acquisition.resolved_deps` (section 1.1). No figure moves and no cell is republished.** Section 9 makes a new column a MINOR bump because prior cells stay valid, which is the case here: no count, hash, mode figure, retrievability score or hygiene grade is computed differently, and the only difference at row level is a field older rows do not have at all. PATCH was rejected: PATCH is for corrections with no effect on any number, and this corrects no prior figure, it adds an observation the harness was not making. The published run schema goes to `0.3` alongside. Rows published before this version lack the field, which reads as "this release did not record it", the same rule section 9 already states for `retrievability` and `hygiene`.

1. **The gap this closes.** The 2026-08-18 `fetch` row published `unreachable` after the server died at import at 15:38 UTC. Re-checked at approximately 16:45 UTC the same day on the same machine, a clean resolve started fine, because `mcp-server-fetch` declares `mcp>=1.1.3` and the two resolves returned different SDK versions. `acquisition.pinned` read `false` on both and could not tell them apart: it reports whether the server package carried a version, never what the resolver chose underneath it. The run artifact therefore could not explain its own row. Corrections log entry 2 records the row and commits the instrument.
2. **What is recorded, per acquisition kind**, is in the table in section 1.1: an `importlib.metadata` listing from inside the uvx environment, a `node_modules` walk of the tree npx installed, the image itself for a container, and a stated reason for the kinds that resolve nothing locally. The listing's argument vector travels on the row, so a reader reruns it rather than reconstructing it.
3. **The listing runs before the launch**, so the resolve it performs is the resolve the launch reuses, rather than a second answer minutes later. The environment path is recorded on both the PyPI and npm paths and is the path the server's own traceback cites, which is what ties a recorded version to an observed failure.
4. **Failure as data extends to the instrument.** A listing that will not run records its error on the row and changes nothing else about the measurement. An acquisition that resolves nothing records why in a note, not a null.
5. **Verified on 2026-08-20** against the two servers whose resolvers the harness uses: `fetch` reports `mcp` 1.29.0 in the uv environment the server process runs from, which is the version the corrections-log re-check found, and `filesystem` reports `@modelcontextprotocol/sdk` 1.30.0 out of the npx tree. The `filesystem` acquisition on that machine failed to launch for an unrelated environment reason and still carried its full resolved set, which is the property the field exists for: a row that cannot start can now say what it was starting.

### 0.2.0, 2026-08-17

**Correction, 2026-08-17.** A code review of the 0.2.0 implementation found six defects in the sections 5 and 6 harness. All six are fixed and this entry is corrected in place at the same version, because **no published figure moves**: the four scanned rows re-scan to identical `canonical_sha256`, identical token counts, identical retrievability (`filesystem` 1.0000 / `playwright` 0.9583 / `context7` 1.0000) and identical hygiene (`filesystem` C 73.41 / `playwright` B 82.18 / `context7` A 97.22), with `fetch` still unreachable and null on all three blocks. Every fix closed a latent defect that the current three-server corpus does not trigger, which is why the numbers hold; each would move a figure on a surface that exercises it, so they are stated here rather than left in the source.

1. **Dimension 4 required only a substring match.** A two-letter enum value passed on any description containing those letters: `ro` inside "zero", `in` inside "within", `id` inside "valid". Matching is now whole-token against the section 5.1 splitter, with short fragments kept and stopwords not stripped. Unmoved on this corpus because every measured enum is self-evident under the second clause of the rule and scores 100 by that path regardless.
2. **The schema walk dropped the draft-07 tuple form of `items`.** `prefixItems` was followed but an `items` array was not, so properties and enums nested inside a tuple were invisible to dimensions 3 and 4. Both forms are now walked. Unmoved because no measured server uses the tuple form.
3. **The name rule the code applied was not the rule this document cites.** The check accepted `.` and any Unicode letter; the cited rule is 1 to 128 characters of `[a-zA-Z0-9_-]`. The code is now the cited rule. Unmoved because no measured tool name carries a dot or a non-ASCII character; a dotted surface such as `jira.search` now loses a third of dimension 5, as the cited rule always said it should.
4. **An empty surface was graded rather than nulled.** Grading zero tools mixed three vacuous 100s with three vacuous 0s into a mean of 50 and published a measured D. Section 6 now states the rule and the harness returns no grade. Unmoved because the sweep already withheld the block for a zero-tool row; the defect was reachable only by a direct caller.
5. **Three descriptor and tokenizer rules existed in duplicate.** The camel and snake splitter, the four-decimal rounding, and the name-title-description join each had two implementations, and the canonicalizer's `descriptor` field, documented as the authoritative join, was consumed by neither metric. There is now one splitter (parameterized for the digit-run and short-fragment behaviours that genuinely differ between the two callers), one rounding, one join, and both metric packages consume the canonicalizer's descriptor rather than rebuilding it. Unmoved because the duplicates agreed; the risk was that a future edit to one would silently move only one metric.
6. **Threshold recalibration checked, not needed.** The 0.60 calibration in item 5 below was rechecked against the re-scanned corpus after the fixes: 368 sibling pairs, median 0.1111, mean 0.1186, exactly one pair above 0.5 at 0.8519, highest legitimate pair 0.4211, then 0.4167 and 0.4000. The distribution is unchanged to four decimals and the empty band between 0.4211 and 0.8519 has not shifted, so the threshold stands.

Two metrics that this document specified but the harness did not compute are now implemented and published: retrievability (section 5) and the schema hygiene grade (section 6). New published metrics are a MINOR bump under section 9. No figure that a 0.1.x row already carried changes: token counts, hashes, and mode figures are byte-identical, and the only difference at the row level is two fields that older rows do not have at all.

1. **Section 5 rewritten from specified to implemented, and its query-authorship OPEN resolved.** Queries are now derived mechanically from each tool's own description: tokenize with snake and camel splitting, strip stopwords, strip the tool's own name fragments, then form three queries (leading content-word phrase, highest-IDF adjacent bigram, deduplicated content-word set). The rejected alternative, the harness author hand-writing task phrases, was indefensible for a public ranking: a critic cannot reproduce a query set that came from the ranking author's judgement. The full derived set is published per server at `data/runs/<date>/<server>-queries.json`, including the rank each query achieved. **This ruling is ratified 2026-08-18.**
2. **Section 5 published figures fixed.** `top3_fraction` (the score) and `mrr` (secondary), at k = 3, with BM25 `k1 = 1.2` and `b = 0.75` stamped into the query artifact. The v0.1 draft specified k = 5 and a hit rate over queries; the change to k = 3 and a fraction over tools makes the headline figure answer "what share of this server's tools can be found" rather than "what share of our queries worked", which is the question a reader of a cost table is actually asking. Documents scoring zero are excluded from the ranking rather than padded onto the tail, so a two-tool server cannot claim top-3 by default.
3. **Section 5.4 states the honest limitation.** Same-source derivation measures within-server disambiguation, whether sibling tools shadow each other, and not real-user phrasing. It is not independent of the hygiene grade, since both read the same descriptor text.
4. **Section 6 rewritten from specified to implemented, and its weights dropped.** The six dimensions are now scored 0 to 100 each, published individually, and averaged **unweighted** for the letter grade (A at 90, B at 75, C at 60, D at 40, F below). The prior 25/20/20/10/10/15 weighting asserted a calibration against a usability outcome that has not been run, which known limitation 8 admitted in the same document. Restoring weights needs an outcome measure to fit them to and is itself a MINOR bump.
5. **Section 6 dimension 6 threshold calibrated to 0.60.** Measured empirically over all 368 sibling pairs on the three servers in `data/latest.json` (filesystem 91 pairs, playwright 276, context7 1), scored as token Jaccard over name, title, and description with stopwords removed. The distribution has a median of 0.111 and a mean of 0.119. Exactly one pair sits above 0.5: `list_directory` and `list_directory_with_sizes` on filesystem, at **0.8519**, whose descriptions are near-verbatim copies differing by one clause. The highest-scoring pair judged legitimate is `browser_network_requests` and `browser_network_request` on playwright at **0.4211**, which cross-reference each other explicitly and state their boundary; next are `browser_take_screenshot` and `browser_snapshot` at 0.4167, which also state their boundary, and `browser_close` and `browser_hover` at 0.4000, where the similarity is entirely an artifact of two four-word descriptions sharing the `browser` name prefix and the word "page". **0.60 sits in the empty band between 0.4211 and 0.8519**, clearing every legitimate pair by at least 0.179 and flagging the one near-duplicate by 0.25. The calibration is corpus-derived and will be rechecked when the corpus grows past three servers; the threshold and the flagged pairs are stamped on every row so the check does not require the harness source.
6. **Section 6 dimension 2 weakness accepted and documented**, which is the second option its OPEN item allowed. Keyword detection produces real false negatives: playwright scores 0 even though two of its descriptions state selection boundaries in phrasing the list does not carry. The list is kept generic and published rather than extended to fit the measured corpus, because a keyword list tuned until playwright passes is a parameter fitted to the servers being ranked. A stronger proxy, most likely sibling-name cross-reference detection, is deferred to v1.
7. **Section 6 vacuous-dimension rule stated.** No properties scores 100 on dimension 3, no enums scores 100 on dimension 4, one tool scores 100 on dimension 6. Scoring an absent thing as zero would reward servers for adding parameters, enums, and siblings they do not need.
8. **The null contract extends to both new metrics.** A row whose surface was never enumerated publishes `"retrievability": null` and `"hygiene": null`, never a zero. A zero on either is a damning measurement and an unreached server has not earned it. Both are computed from the already-captured surface, so they survive a token-count failure that withholds the modes block: the fetch row in the current run carries null for all three, while the three enumerated rows carry all three.
9. **Section 3.2's OPEN narrowed, not closed.** The BM25 index now exists in the harness, but it is not wired into the progressive-disclosure mode: `PerToolAvg` is still a mean over all measured tool costs. The obstacle is stated in 3.2, that the section 5 queries are same-source by construction and would bias the simulated retrieval set.
10. **Section 9 note on cross-version reading added.** A 0.1.x row lacks these two fields rather than differing from a 0.2.x row on any shared one, and a missing field reads as "not computed by that release", the same as a null.
11. **Gemini pinned model changed, 2026-08-18: `gemini-2.5-pro` to `gemini-3.1-pro-preview`.** Google's API now rejects `gemini-2.5-pro` for new API keys ("no longer available to new users") and its own error message names `gemini-3.1-pro-preview` as the replacement, which is what the harness pins. A preview-suffixed pin is expected to churn; every cell is stamped `(model, tokenizer, measured_at)` per section 1.6, so a future repin classifies as a "tokenizer changed" delta, never a server delta. No prior published cell carried a Gemini count (all earlier rows were `available:false` for lack of a key), so no comparability is lost. First Gemini-counted run: 2026-08-18, nine servers.

### 0.1.1, 2026-08-17

The Tier 1 harness is now built. Bringing this document into agreement with what it actually implements surfaced the following, all PATCH-level: no published number changes as a result of these corrections, only the text describing it.

1. **Section 1.5 / section 8 key-order conflict resolved.** Section 1.5 fixes canonical key order as-sent because token counts depend on it; section 8 wanted a digest that tolerates cosmetic reordering. The harness ships two digests rather than choosing one: `canonical_sha256` (order-preserving, the token-counting basis) and `canonical_sorted_sha256` (key-sorted, tools array sorted by name, delivers section 8's reorder-tolerant detection). Section 8 and section 9's delta classification were amended to name both and state their separate roles. **This ruling is ratified 2026-08-18.**
2. **Section 1.5 allowlist self-contradiction fixed.** The prior text restricted the tool object to six fields, then separately said `icons`, `_meta`, and a nonexistent `x-mcp-header` annotation were retained. The canonicalizer implements a single eight-field allow list (`name`, `title`, `description`, `inputSchema`, `outputSchema`, `annotations`, `icons`, `_meta`) with nothing stripped inside a retained field; the section now states that directly and drops the `x-mcp-header` reference, which does not exist in the implementation.
3. **`partial_surface` consistency confirmed.** Section 7 is the only status enumeration in this document; no other status list exists to fall out of step with it. No text changed.
4. **Section 3.1 tokenizer basis stated explicitly.** Added a paragraph naming `o200k_base` as the tokenizer the mode formulas in section 3 are computed on, since it is the only tokenizer the harness runs offline; the Claude and Gemini columns are per-provider counts of the same canonical string and do not feed the mode formulas.
5. **Section 3.3 clamp documented.** The flat ~1k-token code-mode estimate is capped at the server's own naive count, so code mode is never reported as costing more than the schemas it replaces. Label remains "modeled" whether or not the clamp binds.
6. **Implementation-gap OPEN items added.** BM25 retrieval simulation (3.2), retrievability scoring (5), schema hygiene grade (6), and dollar-pair computation (4) are specified in this document but not yet implemented in the Tier 1 harness. The published site computes and displays dollar pairs from its own `pricing.json` independently of the harness in the interim.
7. **Section 7 operator-abort behaviour documented.** The harness treats an abort as a property of the run rather than a failure class of any server: an aborted run publishes rows only for the servers that completed, the unswept servers carry no row, and the run document carries `"aborted": true`. If the interrupt lands before any server completes, no artifacts are written and the prior release is left in place. This was already the implemented behaviour; the section now states it.
8. **Section 3.4 modes-null contract added.** When the o200k_base count that the mode formulas run on is unavailable, whether the total count failed or any single per-tool count failed, the row publishes `"modes": null` rather than a mean over a partial, arbitrary subset of the surface. Consumers must read a null modes block as "pending measurement," never as a measured zero. This was already the implemented behaviour; the section now states it.
