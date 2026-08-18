import type { StackResult } from '../lib/aggregate'
import { modeLabel, modelLabel } from '../lib/aggregate'
import { formatPercent, formatTokens, formatUsd } from '../lib/format'
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
}

function KindChip({ kind }: { kind: 'measured' | 'modeled' }) {
  return <span className={`kind-chip kind-chip--${kind}`}>{kind}</span>
}

export default function ResultsPanel({ result, modeResults, selectedCount }: ResultsPanelProps) {
  if (selectedCount === 0) {
    return (
      <div className="results-panel results-panel--empty">
        Select one or more servers to see the stack's context cost.
      </div>
    )
  }

  const barMax = Math.max(...result.attribution.map((r) => r.tokens), 1)
  // No selected server produced a figure for the selected mode. The detail
  // view must not render the resulting 0 as a total or price it: methodology
  // section 3.4 forbids coercing a pending measurement into a measured zero.
  const primaryPending = result.attribution.length === 0

  return (
    <div className="results-panel">
      <div className="results-panel__modes">
        <h4 className="results-panel__subhead">All three client modes</h4>
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
                    <span className="mode-compare__tokens">{formatTokens(m.totalTokens)}</span>
                    <span className="mode-compare__window">
                      {formatPercent(m.windowFraction)} of a 200k window
                    </span>
                  </>
                )}
                {isPrimary && <span className="mode-compare__primary-tag">detailed below</span>}
              </div>
            )
          })}
        </div>
        <p className="results-panel__modes-note">
          A stack has three costs, not one. Cost depends on what the client does with the tool surface, so all three
          modes are always reported.
        </p>
      </div>

      <div className="results-panel__headline">
        <h4 className="results-panel__subhead">{modeLabel(result.mode)}, in detail</h4>
        {primaryPending ? (
          <p className="results-panel__pending">
            Pending measurement. No selected server has a {modeLabel(result.mode).toLowerCase()} figure for this
            model, so there is no total to report. A missing measurement is not a zero.
          </p>
        ) : (
          <>
            <div className="results-panel__total">
              <span className="results-panel__total-number">{formatTokens(result.totalTokens)}</span>
              <span className="results-panel__total-label">tokens <KindChip kind={result.kind} /></span>
            </div>
            {result.totalTokensLow !== null && result.totalTokensHigh !== null && (
              <p className="results-panel__range">
                Range across k = 3 to 5 retrieved tools: {formatTokens(result.totalTokensLow)} to{' '}
                {formatTokens(result.totalTokensHigh)} tokens.
              </p>
            )}
          </>
        )}
      </div>

      {!primaryPending && (
        <div className="results-panel__window">
          <div className="results-panel__window-label">
            <span>Share of a 200k context window</span>
            <span>{formatPercent(result.windowFraction)}</span>
          </div>
          <div className="progress-track">
            <div
              className="progress-fill"
              style={{ width: `${Math.min(result.windowFraction, 100)}%` }}
            />
          </div>
        </div>
      )}

      {!primaryPending && (
        <div className="results-panel__attribution">
          <h4 className="results-panel__subhead">Per-server attribution</h4>
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
                <span className="attribution-row__tokens">{formatTokens(row.tokens)}</span>
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
          <h4 className="results-panel__subhead">
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
                  <td>{formatUsd(result.dollars.coldWrite)}</td>
                  <td>
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

      {result.excluded.length > 0 && (
        <div className="results-panel__excluded">
          <h4 className="results-panel__subhead">Excluded from totals</h4>
          <ul>
            {result.excluded.map((row) => (
              <li key={row.server.id}>
                <span className="excluded-row__name">{row.server.name}</span>: {row.reason}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
