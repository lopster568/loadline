import type { ClientMode, ModelId, ServerEntry } from '../types'
import { formatTokens } from '../lib/format'
import { NO_DATA_LABEL } from '../lib/aggregate'
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

const MODE_OPTIONS: { id: ClientMode; label: string; hint: string }[] = [
  { id: 'naive', label: 'Naive full load (measured)', hint: 'All tool definitions loaded upfront' },
  { id: 'tool_search', label: 'Tool Search (modeled)', hint: 'Search stub upfront, 3 to 5 tools loaded on demand' },
  { id: 'code_mode', label: 'Code Mode (modeled)', hint: 'Whole API expressed as a compact interface' },
]

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
            {MODE_OPTIONS.map((opt) => (
              <label key={opt.id} className={`mode-option${mode === opt.id ? ' mode-option--checked' : ''}`}>
                <input
                  type="radio"
                  name="mode"
                  checked={mode === opt.id}
                  onChange={() => onModeChange(opt.id)}
                />
                <span>
                  <span className="mode-option__label">{opt.label}</span>
                  <span className="mode-option__hint">{opt.hint}</span>
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
