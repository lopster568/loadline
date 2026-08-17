import { useMemo, useState } from 'react'
import type { ServerEntry } from '../types'
import { formatFraction, formatTokens } from '../lib/format'
import './Leaderboard.css'

interface LeaderboardProps {
  servers: ServerEntry[]
}

interface Row {
  server: ServerEntry
  naiveTokens: number | null
  perToolAvg: number | null
  dataOk: boolean
  hygieneGrade: string | null
  hygieneScore: number | null
  top3Fraction: number | null
}

type SortKey =
  | 'name'
  | 'tool_count'
  | 'naive'
  | 'per_tool_avg'
  | 'status'
  | 'protocol_revision'
  | 'hygiene'
  | 'retrievability'
type SortDir = 'asc' | 'desc'

const COLUMNS: { key: SortKey; label: string }[] = [
  { key: 'name', label: 'Name' },
  { key: 'tool_count', label: 'Tool count' },
  { key: 'naive', label: 'Naive tokens' },
  { key: 'per_tool_avg', label: 'Per-tool avg' },
  { key: 'status', label: 'Status' },
  { key: 'protocol_revision', label: 'Protocol revision' },
  { key: 'hygiene', label: 'Hygiene' },
  { key: 'retrievability', label: 'Retrievability' },
]

// Compares two nullable numbers so null always sorts last, in either sort
// direction. Direction only decides the order between two non-null values;
// null-ness is decided first and is never negated by desc.
function compareNullable(a: number | null, b: number | null, dir: SortDir): number {
  if (a === null && b === null) return 0
  if (a === null) return 1
  if (b === null) return -1
  const cmp = a - b
  return dir === 'asc' ? cmp : -cmp
}

export default function Leaderboard({ servers }: LeaderboardProps) {
  const [sortKey, setSortKey] = useState<SortKey>('per_tool_avg')
  const [sortDir, setSortDir] = useState<SortDir>('asc')

  const rows: Row[] = useMemo(
    () =>
      servers.map((server) => {
        const openaiAvailable = server.counts.openai_o200k.available ?? true
        const dataOk = server.status === 'ok' && openaiAvailable
        const naiveTokens = dataOk ? server.counts.openai_o200k.total_schema_tokens : null
        const perToolAvg =
          naiveTokens !== null && server.tool_count !== null && server.tool_count > 0
            ? Math.round(naiveTokens / server.tool_count)
            : null
        const hygieneGrade = server.hygiene?.grade ?? null
        const hygieneScore = server.hygiene?.score ?? null
        const top3Fraction = server.retrievability?.top3_fraction ?? null
        return { server, naiveTokens, perToolAvg, dataOk, hygieneGrade, hygieneScore, top3Fraction }
      }),
    [servers],
  )

  const sorted = useMemo(() => {
    const copy = [...rows]
    copy.sort((a, b) => {
      // Servers with a failed/unavailable count never rank on the merits of
      // a real measurement, pin them last no matter which column or
      // direction is active, instead of letting a null-as-zero or
      // null-as-Infinity artifact place them first.
      if (a.dataOk !== b.dataOk) return a.dataOk ? -1 : 1

      // Numeric columns pin null values last regardless of sortDir: null-ness
      // is compared before direction is applied, so desc never negates the
      // nulls-last sentinel and flips an ungraded/unmeasured row to the top.
      let cmp = 0
      switch (sortKey) {
        case 'name':
          cmp = a.server.name.localeCompare(b.server.name)
          return sortDir === 'asc' ? cmp : -cmp
        case 'tool_count':
          return compareNullable(a.server.tool_count, b.server.tool_count, sortDir)
        case 'naive':
          return compareNullable(a.naiveTokens, b.naiveTokens, sortDir)
        case 'per_tool_avg':
          return compareNullable(a.perToolAvg, b.perToolAvg, sortDir)
        case 'status':
          cmp = a.server.status.localeCompare(b.server.status)
          return sortDir === 'asc' ? cmp : -cmp
        case 'protocol_revision':
          cmp = (a.server.protocol_revision ?? '').localeCompare(b.server.protocol_revision ?? '')
          return sortDir === 'asc' ? cmp : -cmp
        case 'hygiene':
          return compareNullable(a.hygieneScore, b.hygieneScore, sortDir)
        case 'retrievability':
          return compareNullable(a.top3Fraction, b.top3Fraction, sortDir)
      }
      return cmp
    })
    return copy
  }, [rows, sortKey, sortDir])

  function handleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir('asc')
    }
  }

  return (
    <div className="leaderboard-wrap">
      <table className="leaderboard">
        <thead>
          <tr>
            {COLUMNS.map((col) => (
              <th key={col.key}>
                <button
                  type="button"
                  className={`leaderboard__sort-btn${sortKey === col.key ? ' leaderboard__sort-btn--active' : ''}`}
                  onClick={() => handleSort(col.key)}
                >
                  {col.label}
                  {sortKey === col.key && <span className="leaderboard__sort-arrow">{sortDir === 'asc' ? '↑' : '↓'}</span>}
                </button>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sorted.map(({ server, naiveTokens, perToolAvg, dataOk, hygieneGrade, hygieneScore, top3Fraction }) => (
            <tr key={server.id} className={!dataOk ? 'leaderboard__row--unreachable' : ''}>
              <td>{server.name}</td>
              <td>{formatTokens(server.tool_count)}</td>
              <td>{formatTokens(naiveTokens)}</td>
              <td>{formatTokens(perToolAvg)}</td>
              <td>{server.status}</td>
              <td>{server.protocol_revision ?? 'n/a'}</td>
              <td>
                {hygieneGrade !== null ? (
                  <span
                    className="hygiene-badge"
                    title={hygieneScore !== null ? `score ${hygieneScore.toFixed(2)}` : undefined}
                  >
                    {hygieneGrade}
                  </span>
                ) : (
                  'n/a'
                )}
              </td>
              <td>{formatFraction(top3Fraction)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
