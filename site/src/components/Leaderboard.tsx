import { useMemo, useState } from 'react'
import type { ServerEntry } from '../types'
import { formatFraction, formatTokens } from '../lib/format'
import { IconWarning } from './Icons'
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

const COLUMNS: { key: SortKey; label: string; numeric: boolean }[] = [
  { key: 'name', label: 'Name', numeric: false },
  { key: 'tool_count', label: 'Tool count', numeric: true },
  { key: 'naive', label: 'Naive tokens', numeric: true },
  { key: 'per_tool_avg', label: 'Per-tool avg', numeric: true },
  { key: 'status', label: 'Status', numeric: false },
  { key: 'protocol_revision', label: 'Protocol revision', numeric: false },
  { key: 'hygiene', label: 'Hygiene', numeric: false },
  { key: 'retrievability', label: 'Retrievability', numeric: true },
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
    <div className="manifest-wrap plate">
      <table className="manifest">
        <thead>
          <tr>
            {COLUMNS.map((col) => (
              <th
                key={col.key}
                className={col.numeric ? 'manifest__th manifest__th--num' : 'manifest__th'}
                aria-sort={sortKey === col.key ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
              >
                <button
                  type="button"
                  className={`manifest__sort${sortKey === col.key ? ' manifest__sort--active' : ''}`}
                  onClick={() => handleSort(col.key)}
                >
                  {col.label}
                  <span className="manifest__sort-arrow" aria-hidden="true">
                    {sortKey === col.key ? (sortDir === 'asc' ? '↑' : '↓') : ''}
                  </span>
                </button>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sorted.map(({ server, naiveTokens, perToolAvg, dataOk, hygieneGrade, hygieneScore, top3Fraction }) => (
            <tr key={server.id} className={!dataOk ? 'manifest__row--nodata' : ''}>
              <td className="manifest__name">{server.name}</td>
              <td className="num manifest__num">{formatTokens(server.tool_count)}</td>
              <td className="num manifest__num">{formatTokens(naiveTokens)}</td>
              <td className="num manifest__num">{formatTokens(perToolAvg)}</td>
              <td>
                <span className="manifest__status">
                  {server.status !== 'ok' && <IconWarning size={12} />}
                  {server.status}
                </span>
              </td>
              <td className="num">{server.protocol_revision ?? 'n/a'}</td>
              <td>
                {hygieneGrade !== null ? (
                  <span
                    className={`grade-square grade-square--${hygieneGrade.toLowerCase()}`}
                    title={hygieneScore !== null ? `score ${hygieneScore.toFixed(2)}` : undefined}
                  >
                    {hygieneGrade}
                  </span>
                ) : (
                  <span className="manifest__na">n/a</span>
                )}
              </td>
              <td className="num manifest__num">{formatFraction(top3Fraction)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
