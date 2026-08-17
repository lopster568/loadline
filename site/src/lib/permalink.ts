import type { ClientMode, ModelId } from '../types'

export interface StackState {
  serverIds: string[]
  mode: ClientMode
  model: ModelId
}

const MODES: ClientMode[] = ['naive', 'tool_search', 'code_mode']
const MODELS: ModelId[] = ['openai_o200k', 'claude', 'gemini']

export const DEFAULT_STATE: StackState = {
  serverIds: [],
  mode: 'naive',
  model: 'openai_o200k',
}

export function stateFromSearch(search: string): StackState {
  const params = new URLSearchParams(search)
  const serversParam = params.get('servers')
  const modeParam = params.get('mode')
  const modelParam = params.get('model')

  const serverIds = serversParam ? serversParam.split(',').filter(Boolean) : []
  const mode = MODES.includes(modeParam as ClientMode) ? (modeParam as ClientMode) : DEFAULT_STATE.mode
  const model = MODELS.includes(modelParam as ModelId) ? (modelParam as ModelId) : DEFAULT_STATE.model

  return { serverIds, mode, model }
}

export function searchFromState(state: StackState): string {
  const params = new URLSearchParams()
  if (state.serverIds.length > 0) params.set('servers', state.serverIds.join(','))
  params.set('mode', state.mode)
  params.set('model', state.model)
  const s = params.toString()
  return s ? `?${s}` : ''
}

export function permalinkUrl(state: StackState): string {
  const { origin, pathname } = window.location
  return `${origin}${pathname}${searchFromState(state)}`
}
