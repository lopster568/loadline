import { useEffect, useMemo, useState } from 'react'
import type { ClientMode, LoadlineData, ModelId, PricingData } from './types'
import Banner from './components/Banner'
import StackBuilder from './components/StackBuilder'
import ResultsPanel from './components/ResultsPanel'
import Leaderboard from './components/Leaderboard'
import Footer from './components/Footer'
import { computeStackResult } from './lib/aggregate'
import { permalinkUrl, searchFromState, stateFromSearch } from './lib/permalink'
import './App.css'

type LoadState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; data: LoadlineData; pricing: PricingData | null }

export default function App() {
  const [load, setLoad] = useState<LoadState>({ status: 'loading' })

  const initial = useMemo(() => stateFromSearch(window.location.search), [])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set(initial.serverIds))
  const [mode, setMode] = useState<ClientMode>(initial.mode)
  const [model, setModel] = useState<ModelId>(initial.model)
  const [copyStatus, setCopyStatus] = useState<'idle' | 'copied'>('idle')

  useEffect(() => {
    const base = import.meta.env.BASE_URL
    Promise.all([
      fetch(`${base}data.json`).then((r) => {
        if (!r.ok) throw new Error(`data.json: HTTP ${r.status}`)
        return r.json() as Promise<LoadlineData>
      }),
      fetch(`${base}pricing.json`)
        .then((r) => (r.ok ? (r.json() as Promise<PricingData>) : null))
        .catch(() => null),
    ])
      .then(([data, pricing]) => setLoad({ status: 'ready', data, pricing }))
      .catch((err: unknown) => setLoad({ status: 'error', message: err instanceof Error ? err.message : String(err) }))
  }, [])

  useEffect(() => {
    const search = searchFromState({ serverIds: Array.from(selectedIds), mode, model })
    const newUrl = `${window.location.pathname}${search}`
    window.history.replaceState(null, '', newUrl)
  }, [selectedIds, mode, model])

  function toggleServer(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function copyLink() {
    const url = permalinkUrl({ serverIds: Array.from(selectedIds), mode, model })
    try {
      await navigator.clipboard.writeText(url)
      setCopyStatus('copied')
      setTimeout(() => setCopyStatus('idle'), 1800)
    } catch {
      window.prompt('Copy this link', url)
    }
  }

  if (load.status === 'loading') {
    return (
      <div className="container app-loading">
        <p>Loading run data…</p>
      </div>
    )
  }

  if (load.status === 'error') {
    return (
      <div className="container app-loading">
        <p>Could not load /data.json: {load.message}</p>
      </div>
    )
  }

  const { data, pricing } = load
  const result = computeStackResult(data.servers, Array.from(selectedIds), mode, model, pricing, data.run.date)
  const unreachableCount = data.servers.filter((s) => s.status === 'unreachable').length

  return (
    <>
      <Banner visible={data.sample} />
      <div className="container app-header">
        <h1 className="app-title">loadline</h1>
        <p className="app-subtitle">
          What an MCP server stack costs an agent's context window, by client mode, before any work happens.
        </p>
      </div>

      <main>
        <section className="section container" id="calculator">
          <h2 className="section-title">Stack calculator</h2>
          <p className="section-note">
            Pick servers, a client mode, and a model. Results update immediately and encode into the URL.
          </p>

          <StackBuilder
            servers={data.servers}
            selectedIds={selectedIds}
            onToggleServer={toggleServer}
            mode={mode}
            onModeChange={setMode}
            model={model}
            onModelChange={setModel}
            runDate={data.run.date}
          />

          <div className="results-block">
            <div className="results-block__header">
              <h3 className="stack-builder__label">Result</h3>
              <button type="button" className="copy-link-btn" onClick={copyLink}>
                {copyStatus === 'copied' ? 'Copied' : 'Copy link'}
              </button>
            </div>
            <ResultsPanel result={result} selectedCount={selectedIds.size} />
          </div>
        </section>

        <section className="section container" id="leaderboard">
          <h2 className="section-title">Leaderboard</h2>
          <p className="section-note">
            Rankings pair cost with retrievability and hygiene grades in a later release; cost alone is not quality.
          </p>
          <Leaderboard servers={data.servers} />
        </section>
      </main>

      <Footer run={data.run} unreachableCount={unreachableCount} />
    </>
  )
}
