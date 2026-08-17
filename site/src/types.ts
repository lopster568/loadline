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

export interface ServerEntry {
  id: string
  name: string
  maintainer: string
  status: ServerStatus
  tool_count: number | null
  protocol_revision: string | null
  provenance: Provenance
  counts: ServerCounts
  modes: ServerModes
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
}

export interface PricingData {
  as_of: string
  label: string
  cache_read_multiplier: number
  cache_write_multiplier: number
  models: Record<string, ModelPriceEntry>
}
