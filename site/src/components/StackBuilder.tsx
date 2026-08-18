import type { ClientMode, ModelId, ServerEntry } from '../types'
import { formatTokens } from '../lib/format'
import { CLIENT_MODES, NO_DATA_LABEL, modeKind, modeLabel } from '../lib/aggregate'
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

  return (
    <div className="stack-builder">
      <div className="stack-builder__servers">
        <h3 className="stack-builder__label">Servers</h3>
        <div className="server-grid">
          {servers.map((server) => {
            const disabledNote = statusLabel(server, runDate)
            const checked = selectedIds.has(server.id)
            return (
              <label
                key={server.id}
                className={`server-card${checked ? ' server-card--checked' : ''}${disabledNote ? ' server-card--unreachable' : ''}`}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => onToggleServer(server.id)}
                />
                <span className="server-card__body">
                  <span className="server-card__name">{server.name}</span>
                  <span className="server-card__meta">
                    {disabledNote ? (
                      <span className="server-card__unreachable-note">{disabledNote}</span>
                    ) : (
                      <>
                        <span>{formatTokens(server.tool_count)} tools</span>
                        <span className="server-card__dot">·</span>
                        <span>{server.maintainer}</span>
                      </>
                    )}
                  </span>
                </span>
              </label>
            )
          })}
        </div>
      </div>

      <div className="stack-builder__row">
        <div className="stack-builder__group">
          <h3 className="stack-builder__label">Client mode</h3>
          <div className="mode-options">
            {CLIENT_MODES.map((id) => (
              <label key={id} className={`mode-option${mode === id ? ' mode-option--checked' : ''}`}>
                <input
                  type="radio"
                  name="mode"
                  checked={mode === id}
                  onChange={() => onModeChange(id)}
                />
                <span>
                  <span className="mode-option__label">
                    {modeLabel(id)} ({modeKind(id)})
                  </span>
                  <span className="mode-option__hint">{MODE_HINTS[id]}</span>
                </span>
              </label>
            ))}
          </div>
        </div>

        <div className="stack-builder__group">
          <h3 className="stack-builder__label">Model / tokenizer</h3>
          <div className="mode-options">
            <label className={`mode-option${model === 'openai_o200k' ? ' mode-option--checked' : ''}`}>
              <input
                type="radio"
                name="model"
                checked={model === 'openai_o200k'}
                onChange={() => onModelChange('openai_o200k')}
              />
              <span>
                <span className="mode-option__label">OpenAI o200k</span>
                <span className="mode-option__hint">Local tiktoken, measured</span>
              </span>
            </label>
            <label
              className={`mode-option${model === 'claude' ? ' mode-option--checked' : ''}${!claudeAvailableAny ? ' mode-option--pending' : ''}`}
            >
              <input type="radio" name="model" checked={model === 'claude'} onChange={() => onModelChange('claude')} />
              <span>
                <span className="mode-option__label">Claude</span>
                <span className="mode-option__hint">{claudeAvailableAny ? 'count_tokens API, measured where available' : 'pending measurement'}</span>
              </span>
            </label>
            <label
              className={`mode-option${model === 'gemini' ? ' mode-option--checked' : ''}${!geminiAvailableAny ? ' mode-option--pending' : ''}`}
            >
              <input type="radio" name="model" checked={model === 'gemini'} onChange={() => onModelChange('gemini')} />
              <span>
                <span className="mode-option__label">Gemini</span>
                <span className="mode-option__hint">{geminiAvailableAny ? 'countTokens, measured where available' : 'pending measurement'}</span>
              </span>
            </label>
          </div>
        </div>
      </div>
    </div>
  )
}
