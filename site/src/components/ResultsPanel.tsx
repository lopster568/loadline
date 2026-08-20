import type { ServerEntry, Tier2Data, Tier2ServerEntry } from '../types'
import type { StackResult } from '../lib/aggregate'
import { modeLabel, modelLabel } from '../lib/aggregate'
import { formatPercent, formatTokens, formatUsd } from '../lib/format'
import { IconX } from './Icons'
import KindChip from './KindChip'
import './ResultsPanel.css'

interface ResultsPanelProps {
  /** The selected mode's result: the detailed primary view. */
  result: StackResult
  /**
   * One result per client mode, in CLIENT_MODES order. PRD section 1.1 and
   * methodology section 3: three modes are always shown, never collapsed into
   * one, so a shared permalink can never be read as a single number.
   */
  modeResults: StackResult[]
  selectedCount: number
  /** Every server in the run, so a selected id can be named in the Tier 2 block. */
  servers: ServerEntry[]
  selectedIds: Set<string>
  /** Null when no Tier 2 file shipped with this build. */
  tier2: Tier2Data | null
}

export default function ResultsPanel({
  result,
  modeResults,
  selectedCount,
  servers,
  selectedIds,
  tier2,
}: ResultsPanelProps) {
  if (selectedCount === 0) {
    return (
      <div className="results-panel plate results-panel--empty">
        Select one or more servers to see what the stack draws from the context window.
      </div>
    )
  }

  const barMax = Math.max(...result.attribution.map((r) => r.tokens), 1)
  // No selected server produced a figure for the selected mode. The detail
  // view must not render the resulting 0 as a total or price it: methodology
  // section 3.4 forbids coercing a pending measurement into a measured zero.
  const primaryPending = result.attribution.length === 0

  /*
    Tier 2 call traffic, for whichever selected servers have a Tier 2 run.
    Three rules this block holds, and they are the reason it is a block of its
    own rather than a column in the totals:

    1. It is never summed into a stack total. Wire traffic and the tool-surface
       footprint are different costs, and the calculator's totals are the
       footprint.
    2. A selected server with no Tier 2 run is named as having none, not
       rendered as a zero.
    3. It carries the measured chip because the traffic was measured, and the
       note says plainly what it is measured over, because five scripted tasks
       are not a server's call cost in general.
  */
  const tier2Servers = tier2
    ? Array.from(selectedIds)
        .map((id) => {
          const server = servers.find((s) => s.id === id)
          const entry = tier2.servers[id]
          return server && entry ? { server, entry } : null
        })
        .filter((row): row is { server: ServerEntry; entry: Tier2ServerEntry } => row !== null)
    : []
  const tier2Missing = tier2
    ? Array.from(selectedIds)
        .map((id) => servers.find((s) => s.id === id))
        .filter((s): s is ServerEntry => s !== undefined && !tier2.servers[s.id])
    : []

  /*
    The heading, the note and the table are gated on there being a measured
    server to put in them. The missing line is not: a selection where nothing
    has a Tier 2 run is exactly the case that most needs to be told the figure
    is absent rather than zero, and gating it behind the table would have said
    nothing at all in the one case where silence reads as "no cost".
  */
  const tier2Block =
    tier2 && (tier2Servers.length > 0 || tier2Missing.length > 0) ? (
      <div className="results-panel__tier2">
        {tier2Servers.length > 0 && (
          <>
            <h4 className="stencil results-panel__subhead">
              Tier 2 call traffic <KindChip kind={tier2.kind} />
              <span className="price-provenance">
                suite {tier2.suite_version}, run {tier2.run_date}
              </span>
            </h4>
            <p className="results-panel__tier2-note">
              Tool-call arguments plus the results the server sent back, on the MCP wire. This is not the tool-surface
              footprint the totals above count, and it is not added to them. Five scripted tasks per server, three
              trials each, every trial counted including the ones that failed. A different task mix gives a different
              figure for the same server.
            </p>
            <table className="tier2-table">
              <thead>
                <tr>
                  <th>Server</th>
                  <th>Client</th>
                  <th>Call tokens per trial</th>
                  <th>Tool calls</th>
                  <th>Trials</th>
                </tr>
              </thead>
              <tbody>
                {tier2Servers.map(({ server, entry }) =>
                  entry.clients.map((c) => (
                    <tr key={`${server.id}-${c.client}`}>
                      <td>{server.name}</td>
                      <td>
                        {c.client} {c.client_version} / {c.model}
                      </td>
                      <td className="num">
                        {formatTokens(c.call_tokens_per_trial.median)} median (
                        {formatTokens(c.call_tokens_per_trial.min)} to {formatTokens(c.call_tokens_per_trial.max)})
                      </td>
                      <td className="num">
                        {c.tool_calls_per_trial.median} median ({c.tool_calls_per_trial.min} to{' '}
                        {c.tool_calls_per_trial.max})
                      </td>
                      <td className="num">
                        {c.successes} of {c.trials} passed
                      </td>
                    </tr>
                  )),
                )}
              </tbody>
            </table>
          </>
        )}
        {tier2Missing.length > 0 && (
          <p className="results-panel__tier2-note">
            No Tier 2 run for {tier2Missing.map((s) => s.name).join(', ')}. Not measured, which is not zero.
          </p>
        )}
      </div>
    ) : null

  return (
    <div className="results-panel plate">
      <div className="results-panel__modes">
        <h4 className="stencil results-panel__subhead">All three client modes</h4>
        <div className="mode-compare">
          {modeResults.map((m) => {
            // No selected server produced a figure for this mode. Methodology
            // section 3.4: a null modes block is "pending measurement", never
            // a measured zero, so no number is rendered here.
            const pending = m.attribution.length === 0
            const isPrimary = m.mode === result.mode
            return (
              <div
                className={`mode-compare__row${isPrimary ? ' mode-compare__row--primary' : ''}`}
                key={m.mode}
              >
                <div className="mode-compare__head">
                  <span className="mode-compare__name">{modeLabel(m.mode)}</span>
                  <KindChip kind={m.kind} />
                </div>
                {pending ? (
                  <span className="mode-compare__pending">pending measurement</span>
                ) : (
                  <>
                    <span className="mode-compare__tokens num">{formatTokens(m.totalTokens)}</span>
                    <span className="mode-compare__window">
                      <span className="num">{formatPercent(m.windowFraction)}</span> of a 200k window
                    </span>
                  </>
                )}
                {isPrimary && <span className="mode-compare__primary-tag">on the gauge</span>}
              </div>
            )
          })}
        </div>
        <p className="results-panel__modes-note">
          A stack has three costs, not one. Cost depends on what the client does with the tool surface, so all three
          modes are always reported.
        </p>
        {/*
          The chips are literal and this note says what they mean, so a reader
          never has to infer it. Methodology 3.2 and 3.3 hold the same rule.
        */}
        <p className="results-panel__modes-note">
          Naive full load is measured: it is this server's own tool surface, counted. Tool Search and Code Mode are
          modeled: k and the flat code-mode figure are assumptions laid over measured per-tool costs. The Tier 2 run
          of 2026-08-18 does not move either label. It measures traffic on the MCP wire, not what a client loaded
          into the model's context.
        </p>
      </div>

      <div className="results-panel__headline">
        <h4 className="stencil results-panel__subhead">{modeLabel(result.mode)}, in detail</h4>
        {primaryPending ? (
          <p className="results-panel__pending">
            Pending measurement. No selected server has a {modeLabel(result.mode).toLowerCase()} figure for this
            model, so there is no total to report. A missing measurement is not a zero.
          </p>
        ) : (
          <>
            <div className="results-panel__total">
              <span className="results-panel__total-number num">{formatTokens(result.totalTokens)}</span>
              <span className="results-panel__total-label">
                tokens <KindChip kind={result.kind} />
              </span>
            </div>
            {result.totalTokensLow !== null && result.totalTokensHigh !== null && (
              <p className="results-panel__range">
                Range across k = 3 to 5 retrieved tools:{' '}
                <span className="num">{formatTokens(result.totalTokensLow)}</span> to{' '}
                <span className="num">{formatTokens(result.totalTokensHigh)}</span> tokens.
              </p>
            )}
          </>
        )}
      </div>

      {!primaryPending && (
        <div className="results-panel__attribution">
          <h4 className="stencil results-panel__subhead">Per-server attribution</h4>
          <div className="attribution-list">
            {result.attribution.map((row) => (
              <div className="attribution-row" key={row.server.id}>
                <span className="attribution-row__name">{row.server.name}</span>
                <div className="attribution-row__bar-track">
                  <div
                    className="attribution-row__bar-fill"
                    style={{ width: `${(row.tokens / barMax) * 100}%` }}
                  />
                </div>
                <span className="attribution-row__tokens num">{formatTokens(row.tokens)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {!primaryPending && (
        <div className="results-panel__dollars">
          {/*
            Dollars come from pricing.json, not from the sweep, so they never
            carry the token measured/modeled chip. They get their own provenance
            label, plus an explicit unverified tag while pricing.json is a
            placeholder.
          */}
          <h4 className="stencil results-panel__subhead">
            Dollar cost, {modelLabel(result.model)}
            {result.pricingAsOf && (
              <span className="price-provenance">estimate, list prices as of {result.pricingAsOf}</span>
            )}
            {result.pricingUnverified && <span className="price-unverified">prices unverified</span>}
          </h4>
          {result.dollars ? (
            <table className="dollars-table">
              <thead>
                <tr>
                  <th>Cold write</th>
                  <th>Cache read</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td className="num">{formatUsd(result.dollars.coldWrite)}</td>
                  <td className="num">
                    {result.dollars.cacheReadAchievable ? (
                      formatUsd(result.dollars.cacheRead)
                    ) : (
                      <span className="dollars-table__unachievable">
                        not achievable (below {formatTokens(result.dollars.minCacheablePrefixTokens)}-token cache
                        minimum)
                      </span>
                    )}
                  </td>
                </tr>
              </tbody>
            </table>
          ) : (
            <p className="results-panel__no-price">No price on file for this model. See pricing.json.</p>
          )}
          <p className="results-panel__price-note">
            Cold write is paid once per cache lifetime. Cache read is what a steady-state turn actually costs. A single
            dollar figure would overstate steady-state cost, so none is shown.
          </p>
        </div>
      )}

      {tier2Block}

      {result.excluded.length > 0 && (
        <div className="results-panel__excluded">
          <h4 className="stencil results-panel__subhead">Excluded from totals</h4>
          <ul>
            {result.excluded.map((row) => (
              <li key={row.server.id}>
                <IconX size={13} />
                <span>
                  <span className="excluded-row__name">{row.server.name}</span>: {row.reason}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
