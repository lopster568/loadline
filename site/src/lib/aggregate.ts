import type { ClientMode, Kind, ModelId, PricingData, ServerEntry } from '../types'

const CONTEXT_WINDOW = 200_000

export interface AttributionRow {
  server: ServerEntry
  tokens: number
  fraction: number
}

export interface ExcludedRow {
  server: ServerEntry
  reason: string
}

export interface StackDollars {
  coldWrite: number
  cacheRead: number
  // Methodology v0 section 4, assumption 3: a surface below the provider's
  // minimum cacheable prefix does not cache at all, so it can never achieve
  // the cache-read figure. The cell is annotated rather than showing a number
  // the stack cannot reach.
  minCacheablePrefixTokens: number | null
  cacheReadAchievable: boolean
}

export interface StackResult {
  mode: ClientMode
  model: ModelId
  kind: Kind
  totalTokens: number
  totalTokensLow: number | null
  totalTokensHigh: number | null
  windowFraction: number
  attribution: AttributionRow[]
  excluded: ExcludedRow[]
  dollars: StackDollars | null
  pricingUnavailable: boolean
  // Dollar provenance: dollars come from pricing.json, not from the sweep, so
  // they never carry the token measured/modeled chip. These two fields drive
  // the dollar section's own labelling.
  pricingAsOf: string | null
  pricingUnverified: boolean
}

const TOOL_SEARCH_K_MID = 4

// Shared with StackBuilder's per-server disabled-note copy, so the "no data"
// wording only lives in one place.
export const NO_DATA_LABEL = 'no data'

// PRD section 1.1 / methodology section 3: three modes, always three, never
// collapsed into one. Order is fixed so the comparison rows, the mode selector
// and the permalink all agree.
export const CLIENT_MODES: ClientMode[] = ['naive', 'tool_search', 'code_mode']

export function modeLabel(mode: ClientMode): string {
  if (mode === 'naive') return 'Naive full load'
  if (mode === 'tool_search') return 'Tool Search'
  return 'Code Mode'
}

/**
 * Methodology 3.1 to 3.3. Naive full load is the canonical serialization
 * counted, so it is measured. Tool search carries measured per-tool costs
 * inside a modeled total, because k is an assumption. Code mode is a flat
 * published figure, so it is modeled outright.
 *
 * The Tier 2 run of 2026-08-18 does not move either modeled label, and this
 * function is where that would show up if it did. Tier 2 measures MCP wire
 * traffic and per-trial session cost; neither client's usage output isolates a
 * mode's tool-definition footprint, so there is nothing to relabel against
 * (docs/tier2-task-suites.md 4.3, methodology 3.3).
 */
export function modeKind(mode: ClientMode): Kind {
  return mode === 'naive' ? 'measured' : 'modeled'
}

function naiveTokensForModel(server: ServerEntry, model: ModelId): number | null {
  if (model === 'openai_o200k') {
    const available = server.counts.openai_o200k.available ?? true
    return available ? server.counts.openai_o200k.total_schema_tokens : null
  }
  if (model === 'claude') return server.counts.claude.available ? server.counts.claude.total_schema_tokens : null
  return server.counts.gemini.available ? server.counts.gemini.total_schema_tokens : null
}

function toolSearchTokens(server: ServerEntry, k: number): number | null {
  const toolSearch = server.modes?.tool_search
  if (!toolSearch) return null
  const { stub_tokens, per_tool_avg } = toolSearch
  if (stub_tokens === null || per_tool_avg === null) return null
  return stub_tokens + per_tool_avg * k
}

/**
 * Computes totals, per-server attribution, and exclusions for a selected
 * stack under one client mode and one model/tokenizer. Servers that are
 * unreachable, or that lack a measurement for the selected model, are
 * excluded from all totals and reported separately rather than silently
 * dropped or treated as zero.
 */
export function computeStackResult(
  allServers: ServerEntry[],
  selectedIds: string[],
  mode: ClientMode,
  model: ModelId,
  pricing: PricingData | null,
  runDate: string,
): StackResult {
  const selected = selectedIds
    .map((id) => allServers.find((s) => s.id === id))
    .filter((s): s is ServerEntry => Boolean(s))

  const attribution: AttributionRow[] = []
  const excluded: ExcludedRow[] = []

  let totalTokens = 0
  let totalLow = 0
  let totalHigh = 0
  const hasRange = mode === 'tool_search'

  for (const server of selected) {
    if (server.status !== 'ok') {
      excluded.push({ server, reason: `${NO_DATA_LABEL} (${server.status} ${runDate})` })
      continue
    }

    if (mode === 'naive') {
      const tokens = naiveTokensForModel(server, model)
      if (tokens === null) {
        excluded.push({ server, reason: `no ${modelLabel(model)} measurement (pending)` })
        continue
      }
      attribution.push({ server, tokens, fraction: 0 })
      totalTokens += tokens
    } else if (mode === 'tool_search') {
      const toolSearch = server.modes?.tool_search ?? null
      const midTokens = toolSearch ? toolSearchTokens(server, TOOL_SEARCH_K_MID) : null
      const lowTokens = toolSearch ? toolSearchTokens(server, toolSearch.k_range[0]) : null
      const highTokens = toolSearch ? toolSearchTokens(server, toolSearch.k_range[1]) : null
      if (midTokens === null || lowTokens === null || highTokens === null) {
        excluded.push({ server, reason: 'no tool-search model (pending)' })
        continue
      }
      attribution.push({ server, tokens: midTokens, fraction: 0 })
      totalTokens += midTokens
      totalLow += lowTokens
      totalHigh += highTokens
    } else {
      const tokens = server.modes?.code_mode.tokens_estimate ?? null
      if (tokens === null) {
        excluded.push({ server, reason: 'no code-mode model (pending)' })
        continue
      }
      attribution.push({ server, tokens, fraction: 0 })
      totalTokens += tokens
    }
  }

  for (const row of attribution) {
    row.fraction = totalTokens > 0 ? row.tokens / totalTokens : 0
  }
  attribution.sort((a, b) => b.tokens - a.tokens)

  const kind = modeKind(mode)

  let dollars: StackResult['dollars'] = null
  let pricingUnavailable = false
  if (pricing) {
    const priceEntry = pricing.models[model]
    if (priceEntry) {
      const pricePerToken = priceEntry.input_price_per_mtok / 1_000_000
      const minPrefix = priceEntry.min_cacheable_prefix_tokens ?? null
      const writeMultiplier = priceEntry.cache_write_multiplier ?? pricing.cache_write_multiplier
      dollars = {
        coldWrite: totalTokens * pricePerToken * writeMultiplier,
        cacheRead: totalTokens * pricePerToken * pricing.cache_read_multiplier,
        minCacheablePrefixTokens: minPrefix,
        cacheReadAchievable: minPrefix === null || totalTokens >= minPrefix,
      }
    } else {
      pricingUnavailable = true
    }
  } else {
    pricingUnavailable = true
  }

  return {
    mode,
    model,
    kind,
    totalTokens,
    totalTokensLow: hasRange ? totalLow : null,
    totalTokensHigh: hasRange ? totalHigh : null,
    windowFraction: (totalTokens / CONTEXT_WINDOW) * 100,
    attribution,
    excluded,
    dollars,
    pricingUnavailable,
    pricingAsOf: pricing?.as_of ?? null,
    // pricing.json ships with label "verify before publish" until real list
    // prices are fetched; surface that so a placeholder cannot look authoritative.
    pricingUnverified: pricing ? pricing.label.toLowerCase().includes('verify before publish') : false,
  }
}

export function modelLabel(model: ModelId): string {
  if (model === 'openai_o200k') return 'OpenAI o200k'
  if (model === 'claude') return 'Claude'
  return 'Gemini'
}
