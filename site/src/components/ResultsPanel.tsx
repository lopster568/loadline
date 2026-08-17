import type { StackResult } from '../lib/aggregate'
import { modelLabel } from '../lib/aggregate'
import { formatPercent, formatTokens, formatUsd } from '../lib/format'
import './ResultsPanel.css'

interface ResultsPanelProps {
  result: StackResult
  selectedCount: number
}

function KindChip({ kind }: { kind: 'measured' | 'modeled' }) {
  return <span className={`kind-chip kind-chip--${kind}`}>{kind}</span>
}

export default function ResultsPanel({ result, selectedCount }: ResultsPanelProps) {
  if (selectedCount === 0) {
    return (
      <div className="results-panel results-panel--empty">
        Select one or more servers to see the stack's context cost.
      </div>
    )
  }

  const barMax = Math.max(...result.attribution.map((r) => r.tokens), 1)

  return (
    <div className="results-panel">
      <div className="results-panel__headline">
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
      </div>

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

      <div className="results-panel__dollars">
        <h4 className="results-panel__subhead">
          Dollar cost, {modelLabel(result.model)} <KindChip kind={result.kind} />
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
                <td>{formatUsd(result.dollars.cacheRead)}</td>
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
