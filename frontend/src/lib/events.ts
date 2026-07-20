import { Events } from '@wailsio/runtime'
import type { AlertProposal, Usage } from '../api/types'

// --- Pub-sub local para eventos que antes vinham do Go via Wails Events ---
// (chat stream, propostas da IA, alertas disparados via push SSE). O Go só
// emite mais o window:shown (hotkey/tray). O resto é produzido pelo cliente
// HTTP/SSE em services.ts e dispersado aqui.

type Handler<T> = (payload: T) => void

const subs: Record<string, Set<Handler<any>>> = {}
function dispatch<T>(name: string, payload: T) {
  const set = subs[name]
  if (!set) return
  for (const fn of set) { try { fn(payload) } catch { /* handler isolado */ } }
}
function subscribe<T>(name: string, cb: Handler<T>): () => void {
  ;(subs[name] ??= new Set()).add(cb as Handler<any>)
  return () => { subs[name]?.delete(cb as Handler<any>) }
}

// --- Chat (produzido pelo consumer SSE de POST /v1/chat em services.ts) ---

export const emitUserMsg = (text: string) => dispatch('chat:user_msg', text)
export const emitChatDelta = (delta: string) => dispatch('chat:delta', delta)
export const emitChatDone = (content: string) => dispatch('chat:done', { content })
export const emitChatError = (msg: string) => dispatch('chat:error', msg)
export const emitChatUsage = (u: Usage) => dispatch('chat:usage', u)
export const emitChatEmptyResponse = (text: string) => dispatch('chat:empty_response', text)
export const emitNoteProposed = (n: { title: string; content: string; tags: string[] }) => dispatch('note:proposed', n)
export const emitAlertProposed = (a: AlertProposal) => dispatch('alert:proposed', a)

export const onUserMsg = (cb: (text: string) => void) => subscribe<string>('chat:user_msg', cb)
export const onChatDelta = (cb: (delta: string) => void) => subscribe<string>('chat:delta', cb)
export const onChatUsage = (cb: (u: Usage) => void) => subscribe<Usage>('chat:usage', cb)
export const onChatDone = (cb: (d: { content: string }) => void) => subscribe<{ content: string }>('chat:done', cb)
export const onChatError = (cb: (msg: string) => void) => subscribe<string>('chat:error', cb)
export const onChatEmptyResponse = (cb: (text: string) => void) => subscribe<string>('chat:empty_response', cb)
export const onNoteProposed = (cb: (p: { title: string; content: string; tags: string[] }) => void) => subscribe<{ title: string; content: string; tags: string[] }>('note:proposed', cb)
export const onAlertProposed = (cb: (p: AlertProposal) => void) => subscribe<AlertProposal>('alert:proposed', cb)

// --- Alertas disparados (produzido pelo consumer SSE de GET /v1/push) ---

export type AlertFiredPayload = { id: number; message: string; noteId: number | null }
export type AlertOpenPayload = { id: number; noteId: number | null }

export const emitAlertFired = (a: AlertFiredPayload) => dispatch('alert:fired', a)
export const emitAlertOpen = (p: AlertOpenPayload) => dispatch('alert:open', p)

export const onAlertFired = (cb: (a: AlertFiredPayload) => void) => subscribe<AlertFiredPayload>('alert:fired', cb)
export const onAlertOpen = (cb: (p: AlertOpenPayload) => void) => subscribe<AlertOpenPayload>('alert:open', cb)

// --- Auth (o refresh falhou em definitivo → a UI volta p/ a tela de login) ---

export const emitAuthError = () => dispatch('auth:error', undefined)
export const onAuthError = (cb: () => void) => subscribe<void>('auth:error', cb)

// --- Window:shown permanece vindo do Go (hotkey/tray) ---

export const onWindowShown = (cb: () => void) =>
  Events.On('window:shown', () => cb())
