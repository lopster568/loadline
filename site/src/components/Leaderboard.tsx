import { useMemo, useState } from 'react'
import type { ServerEntry } from '../types'
import { formatTokens } from '../lib/format'
import './Leaderboard.css'

interface LeaderboardProps {
  servers: ServerEntry[]
}

interface Row {
  server: ServerEntry
  naiveTokens: number | null
  perToolAvg: number | null
  dataOk: boolean
}

type SortKey = 'name' | 'tool_count' | 'naive' | 'per_tool_avg' | 'status' | 'protocol_revision'
type SortDir = 'asc' | 'desc'

const COLUMNS: { key: SortKey; label: string }[] = [
  { key: 'name', label: 'Name' },
  { key: 'tool_count', label: 'Tool count' },
  { key: 'naive', label: 'Naive tokens' },
  { key: 'per_tool_avg', label: 'Per-tool avg' },
  { key: 'status', label: 'Status' },
  { key: 'protocol_revision', label: 'Protocol revision' },
]

function nullsLast(v: number | null): number {
  return v === null ? Number.POSITIVE_INFINITY : v
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
        return { server, naiveTokens, perToolAvg, dataOk }
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

      let cmp = 0
      switch (sortKey) {
        case 'name':
          cmp = a.server.name.localeCompare(b.server.name)
          break
        case 'tool_count':
          cmp = nullsLast(a.server.tool_count) - nullsLast(b.server.tool_count)
          break
        case 'naive':
          cmp = nullsLast(a.naiveTokens) - nullsLast(b.naiveTokens)
          break
        case 'per_tool_avg':
          cmp = nullsLast(a.perToolAvg) - nullsLast(b.perToolAvg)
          break
        case 'status':
          cmp = a.server.status.localeCompare(b.server.status)
          break
        case 'protocol_revision':
          cmp = (a.server.protocol_revision ?? '').localeCompare(b.server.protocol_revision ?? '')
          break
      }
      return sortDir === 'asc' ? cmp : -cmp
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
          {sorted.map(({ server, naiveTokens, perToolAvg, dataOk }) => (
            <tr key={server.id} className={!dataOk ? 'leaderboard__row--unreachable' : ''}>
              <td>{server.name}</td>
              <td>{formatTokens(server.tool_count)}</td>
              <td>{formatTokens(naiveTokens)}</td>
              <td>{formatTokens(perToolAvg)}</td>
              <td>{server.status}</td>
              <td>{server.protocol_revision ?? 'n/a'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
