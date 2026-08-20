import { useEffect, useMemo, useState } from 'react'
import type { ClientMode, LoadlineData, ModelId, PricingData, ServerStatus, Tier2Data } from './types'
import Banner from './components/Banner'
import StackBuilder from './components/StackBuilder'
import DraftGauge from './components/DraftGauge'
import ResultsPanel from './components/ResultsPanel'
import Leaderboard from './components/Leaderboard'
import Footer from './components/Footer'
import { IconCheck, IconLink, PlimsollMark } from './components/Icons'
import { CLIENT_MODES, computeStackResult } from './lib/aggregate'
import { permalinkUrl, searchFromState, stateFromSearch } from './lib/permalink'
import './App.css'

type LoadState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; data: LoadlineData; pricing: PricingData | null; tier2: Tier2Data | null }

// Both files are served from the Vite base, which is not necessarily the site
// root, so the error copy quotes the URL actually fetched.
const DATA_URL = `${import.meta.env.BASE_URL}data.json`
const PRICING_URL = `${import.meta.env.BASE_URL}pricing.json`
// Tier 2 is a separate tier with its own suite version and run date, so it
// ships as its own file. Absent or unreadable means "no Tier 2 run to show",
// which is not a zero and is not an error: the calculator is Tier 1 and stands
// on its own without it.
const TIER2_URL = `${import.meta.env.BASE_URL}tier2.json`

export default function App() {
  const [load, setLoad] = useState<LoadState>({ status: 'loading' })

  const initial = useMemo(() => stateFromSearch(window.location.search), [])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set(initial.serverIds))
  const [mode, setMode] = useState<ClientMode>(initial.mode)
  const [model, setModel] = useState<ModelId>(initial.model)
  const [copyStatus, setCopyStatus] = useState<'idle' | 'copied'>('idle')

  useEffect(() => {
    Promise.all([
      fetch(DATA_URL).then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`)
        return r.json() as Promise<LoadlineData>
      }),
      fetch(PRICING_URL)
        .then((r) => (r.ok ? (r.json() as Promise<PricingData>) : null))
        .catch(() => null),
      fetch(TIER2_URL)
        .then((r) => (r.ok ? (r.json() as Promise<Tier2Data>) : null))
        .catch(() => null),
    ])
      .then(([data, pricing, tier2]) => setLoad({ status: 'ready', data, pricing, tier2 }))
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

  // Computed here (before the loading/error early returns) so the hook order
  // stays stable across renders; null until data has actually loaded.
  // All three modes are computed on every render, not just the selected one:
  // PRD section 1.1 requires the panel to show three totals at once, so a
  // permalink can never open as a single number.
  const modeResults = useMemo(() => {
    if (load.status !== 'ready') return null
    return CLIENT_MODES.map((m) =>
      computeStackResult(load.data.servers, Array.from(selectedIds), m, model, load.pricing, load.data.run.date),
    )
  }, [load, selectedIds, model])

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
        <p>Could not load {DATA_URL}: {load.message}</p>
      </div>
    )
  }

  const { data } = load
  // Non-null here: load.status === 'ready' at this point, and the useMemo
  // above computes the results whenever load is 'ready'.
  const allModeResults = modeResults!
  const stackResult = allModeResults.find((r) => r.mode === mode) ?? allModeResults[0]

  // Methodology section 7 defines six failure classes, all of which are
  // failures; counting only `unreachable` undercounts the run.
  const failuresByClass: Partial<Record<ServerStatus, number>> = {}
  for (const server of data.servers) {
    if (server.status === 'ok') continue
    failuresByClass[server.status] = (failuresByClass[server.status] ?? 0) + 1
  }

  return (
    <>
      <Banner visible={data.sample} />

      <header className="masthead">
        <div className="container masthead__inner">
          <div className="brand">
            <PlimsollMark size={26} className="brand__mark" />
            <h1 className="brand__word">loadline</h1>
            <span className="stencil brand__tag">MCP stack context cost</span>
          </div>
          <p className="masthead__lede">
            What an MCP server stack costs an agent's context window, by client mode, before any work happens.
          </p>
        </div>
      </header>

      <div className="rule-strip" aria-hidden="true" />

      <main>
        <section className="section container" id="calculator">
          <h2 className="stencil section__title">Stack calculator</h2>
          <p className="section__note">
            Pick servers, a client mode, and a model. Results update immediately and encode into the URL.
          </p>

          <div className="workbench">
            <div className="workbench__builder">
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
            </div>

            {/*
              The gauge rail spans both rows of the left column, so it is the
              only thing in its own column and can actually travel as the page
              scrolls instead of sitting on top of the result panel.
            */}
            <aside className="gauge-rail">
              <DraftGauge result={stackResult} selectedCount={selectedIds.size} />
            </aside>

            <div className="results-block">
              <div className="results-block__header">
                <h3 className="stencil">Result</h3>
                <button type="button" className="copy-link-btn" onClick={copyLink}>
                  {copyStatus === 'copied' ? <IconCheck size={13} /> : <IconLink size={13} />}
                  {copyStatus === 'copied' ? 'Copied' : 'Copy link'}
                </button>
              </div>
              <ResultsPanel
                result={stackResult}
                modeResults={allModeResults}
                selectedCount={selectedIds.size}
                servers={data.servers}
                selectedIds={selectedIds}
                tier2={load.tier2}
              />
            </div>
          </div>
        </section>

        <div className="rule-strip" aria-hidden="true" />

        <section className="section container" id="leaderboard">
          <h2 className="stencil section__title">The manifest</h2>
          <p className="section__note">
            Cost columns sit beside hygiene and retrievability grades; a low-cost server with a low hygiene grade is
            not a recommendation.
          </p>
          <Leaderboard servers={data.servers} />
        </section>
      </main>

      <Footer run={data.run} failuresByClass={failuresByClass} serverCount={data.servers.length} />
    </>
  )
}
