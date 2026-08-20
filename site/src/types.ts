// Shapes emitted by the loadline Go harness (docs/methodology-v0.md, PRD.md
// section 1). The site is a pure renderer over this JSON; it does not compute
// measurements, only aggregates and formats them.

export type Kind = 'measured' | 'modeled'

export type ServerStatus = 'ok' | 'unreachable' | 'auth' | 'protocol_error' | 'timeout' | 'partial_surface' | 'schema_invalid'

export interface RunInfo {
  date: string
  methodology_version: string
  harness_version: string
}

export interface Provenance {
  server_version: string | null
  wire_sha256: string | null
  canonical_sha256: string | null
}

export interface OpenAICounts {
  // Absent in older data files, which predate this field; treated as
  // available: true for backward compat (see naiveTokensForModel).
  available?: boolean
  total_schema_tokens: number | null
  per_tool: Record<string, number>
}

export interface ClaudeCounts {
  model: string
  available: boolean
  total_schema_tokens: number | null
  native_tools_param_tokens: number | null
  measured_at: string | null
}

export interface GeminiCounts {
  model: string
  available: boolean
  total_schema_tokens: number | null
  measured_at: string | null
}

export interface ServerCounts {
  openai_o200k: OpenAICounts
  claude: ClaudeCounts
  gemini: GeminiCounts
}

export interface NaiveMode {
  tokens: number | null
  kind: 'measured'
}

export interface ToolSearchMode {
  stub_tokens: number | null
  per_tool_avg: number | null
  k_range: [number, number]
  kind: 'modeled'
}

export interface CodeMode {
  tokens_estimate: number | null
  kind: 'modeled'
}

export interface ServerModes {
  naive: NaiveMode
  tool_search: ToolSearchMode
  code_mode: CodeMode
}

export interface RetrievabilityScore {
  top3_fraction: number
  mrr: number
  queries_per_tool: number
  kind: Kind
}

export interface HygieneDimensions {
  description_adequacy: number
  when_to_use_signal: number
  parameter_descriptions: number
  enum_documentation: number
  naming_clarity: number
  disambiguation: number
}

export interface HygieneGrade {
  grade: string
  score: number
  dimensions: HygieneDimensions
  kind: Kind
}

export interface ServerEntry {
  id: string
  name: string
  maintainer: string
  status: ServerStatus
  tool_count: number | null
  protocol_revision: string | null
  provenance: Provenance
  counts: ServerCounts
  // Null/omitted when openai_o200k is unavailable: naive/tool_search/code_mode
  // estimates all derive from the o200k schema-token count.
  modes?: ServerModes | null
  // Schema 0.2+. Null when the surface couldn't be enumerated (unreachable,
  // etc). Absent entirely on pre-0.2 data, which never computed these:
  // treated the same as null, never as zero.
  retrievability?: RetrievabilityScore | null
  hygiene?: HygieneGrade | null
}

export interface LoadlineData {
  schema_version: string
  sample: boolean
  run: RunInfo
  servers: ServerEntry[]
}

export type ModelId = 'openai_o200k' | 'claude' | 'gemini'
export type ClientMode = 'naive' | 'tool_search' | 'code_mode'

export interface ModelPriceEntry {
  label: string
  input_price_per_mtok: number
  notes?: string
  // Methodology v0 section 4, assumption 3. Surfaces below this many tokens do
  // not cache at all, so the cache-read figure is unreachable for them.
  // Absent on older pricing files, which carried no minimum: treated as
  // "no minimum on file", never as zero.
  min_cacheable_prefix_tokens?: number
  min_cacheable_prefix_note?: string
  // Per-model override for PricingData.cache_write_multiplier. Most providers'
  // cache-write premium matches the global default, but some (e.g. Gemini
  // explicit caching) charge no premium on the populating write, only a
  // separate storage fee this site does not model. Absent means "use the
  // global multiplier," so older pricing files behave exactly as before.
  cache_write_multiplier?: number
}

export interface PricingData {
  as_of: string
  label: string
  cache_read_multiplier: number
  cache_write_multiplier: number
  models: Record<string, ModelPriceEntry>
}

// Tier 2 (docs/tier2-task-suites.md), published from
// data/tier2-published.json. A separate file from data.json because it is a
// separate tier with its own suite version, its own server pins and its own
// run date, and because the raw Tier 2 tree is gitignored: only this derived
// summary is publishable.
//
// Nothing in here is a mode footprint. The interposer measures MCP wire
// traffic, not what a client loaded into the model's context (spec 4.3), so
// these figures never feed a stack total and never move a mode's kind chip.

export interface Tier2Distribution {
  median: number
  min: number
  max: number
}

export interface Tier2ClientEntry {
  client: string
  client_version: string
  model: string
  trials: number
  successes: number
  call_tokens_per_trial: Tier2Distribution
  tool_calls_per_trial: Tier2Distribution
}

export interface Tier2ServerEntry {
  server_pin: string
  tasks: number
  clients: Tier2ClientEntry[]
}

export interface Tier2Data {
  schema: string
  kind: 'measured'
  run_date: string
  suite_version: string
  interposer_version: string
  tokenizer: string
  trials: number
  successes: number
  // Keyed by the same server id data.json uses. A server absent from this map
  // has no Tier 2 run, which is not a zero.
  servers: Record<string, Tier2ServerEntry>
}
