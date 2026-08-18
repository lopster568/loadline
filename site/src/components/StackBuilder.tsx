import type { ClientMode, ModelId, ServerEntry } from '../types'
import { formatTokens } from '../lib/format'
import { CLIENT_MODES, NO_DATA_LABEL, modeKind, modeLabel, modelLabel } from '../lib/aggregate'
import { IconCheck, IconWarning } from './Icons'
import BrandIcon from './BrandIcon'
import KindChip from './KindChip'
import './StackBuilder.css'

interface StackBuilderProps {
  servers: ServerEntry[]
  selectedIds: Set<string>
  onToggleServer: (id: string) => void
  mode: ClientMode
  onModeChange: (mode: ClientMode) => void
  model: ModelId
  onModelChange: (model: ModelId) => void
  runDate: string
}

// Labels and kinds come from lib/aggregate so the selector and the three-mode
// comparison in ResultsPanel can never drift apart.
const MODE_HINTS: Record<ClientMode, string> = {
  naive: 'All tool definitions loaded upfront',
  tool_search: 'Search stub upfront, 3 to 5 tools loaded on demand',
  code_mode: 'Whole API expressed as a compact interface',
}

const MODEL_IDS: ModelId[] = ['openai_o200k', 'claude', 'gemini']

function statusLabel(server: ServerEntry, runDate: string): string | null {
  if (server.status === 'ok') return null
  return `${NO_DATA_LABEL} (${server.status} ${runDate})`
}

export default function StackBuilder({
  servers,
  selectedIds,
  onToggleServer,
  mode,
  onModeChange,
  model,
  onModelChange,
  runDate,
}: StackBuilderProps) {
  const claudeAvailableAny = servers.some((s) => s.counts.claude.available)
  const geminiAvailableAny = servers.some((s) => s.counts.gemini.available)

  // A model with no measurement anywhere in the run is still selectable, and
  // still says so: "pending measurement" is the honest label, on the switch
  // and in the caption, never a silent zero downstream.
  const modelHints: Record<ModelId, string> = {
    openai_o200k: 'Local tiktoken, measured',
    claude: claudeAvailableAny ? 'count_tokens API, measured where available' : 'pending measurement',
    gemini: geminiAvailableAny ? 'countTokens, measured where available' : 'pending measurement',
  }
  const modelPending: Record<ModelId, boolean> = {
    openai_o200k: false,
    claude: !claudeAvailableAny,
    gemini: !geminiAvailableAny,
  }

  return (
    <div className="stack-builder">
      <section className="builder-block">
        <div className="builder-block__head">
          <h3 className="stencil">Servers</h3>
          <span className="builder-block__count num">
            {selectedIds.size} of {servers.length} loaded
          </span>
        </div>

        <div className="cargo-grid">
          {servers.map((server) => {
            const disabledNote = statusLabel(server, runDate)
            const checked = selectedIds.has(server.id)
            const hygiene = server.hygiene ?? null
            const grade = hygiene?.grade ?? null
            return (
              <label
                key={server.id}
                className={`cargo-plate plate${checked ? ' cargo-plate--loaded' : ''}${disabledNote ? ' cargo-plate--nodata' : ''}`}
              >
                <input
                  type="checkbox"
                  className="cargo-plate__input"
                  checked={checked}
                  onChange={() => onToggleServer(server.id)}
                />
                <span className="cargo-plate__tick" aria-hidden="true">
                  {checked && <IconCheck size={12} />}
                </span>
                <span className="cargo-plate__body">
                  <span className="cargo-plate__name">
                    <BrandIcon id={server.id} size={16} className="cargo-plate__icon" />
                    {server.name}
                  </span>
                  <span className="cargo-plate__meta">
                    {disabledNote ? (
                      <span className="cargo-plate__nodata-note">
                        <IconWarning size={12} />
                        {disabledNote}
                      </span>
                    ) : (
                      <>
                        <span className="num">{formatTokens(server.tool_count)}</span>
                        <span>tools</span>
                        <span className="cargo-plate__sep">/</span>
                        <span className="cargo-plate__maintainer">{server.maintainer}</span>
                      </>
                    )}
                  </span>
                </span>
                {hygiene !== null && grade !== null && (
                  <span
                    className={`grade-square grade-square--${grade.toLowerCase()}`}
                    title={`Hygiene grade ${grade}, score ${hygiene.score.toFixed(2)} (${hygiene.kind})`}
                  >
                    {grade}
                  </span>
                )}
              </label>
            )
          })}
        </div>
      </section>

      <div className="builder-switches">
        <section className="builder-block">
          <h3 className="stencil">Client mode</h3>
          <div className="switch">
            {CLIENT_MODES.map((id) => (
              <label key={id} className={`switch__seg${mode === id ? ' switch__seg--on' : ''}`} title={MODE_HINTS[id]}>
                <input type="radio" name="mode" checked={mode === id} onChange={() => onModeChange(id)} />
                <span className="switch__label">{modeLabel(id)}</span>
              </label>
            ))}
          </div>
          <p className="switch__caption">
            <KindChip kind={modeKind(mode)} />
            {MODE_HINTS[mode]}
          </p>
        </section>

        <section className="builder-block">
          <h3 className="stencil">Model / tokenizer</h3>
          <div className="switch">
            {MODEL_IDS.map((id) => (
              <label
                key={id}
                className={`switch__seg${model === id ? ' switch__seg--on' : ''}${modelPending[id] ? ' switch__seg--pending' : ''}`}
                title={modelHints[id]}
              >
                <input type="radio" name="model" checked={model === id} onChange={() => onModelChange(id)} />
                <span className="switch__brand-icon" aria-hidden="true">
                  <BrandIcon id={id} size={14} />
                </span>
                <span className="switch__label">{modelLabel(id)}</span>
                {modelPending[id] && (
                  <span className="switch__icon switch__icon--warn" aria-hidden="true">
                    <IconWarning size={13} />
                  </span>
                )}
              </label>
            ))}
          </div>
          <p className={`switch__caption${modelPending[model] ? ' switch__caption--pending' : ''}`}>
            {modelHints[model]}
          </p>
        </section>
      </div>
    </div>
  )
}
