// Serviços do gix-server espelhando os antigos bindings Wails (NotesService,
// AlertsService, HistoryService, ChatService) + models/prefs/auth. O frontend
// chama estas funções em vez dos bindings Go.

import {
  ApiError,
  AuthError,
  clearTokens,
  getRefreshToken,
  isAuthed,
  loadTokens,
  request,
  setBaseURL,
  setTokens,
  streamSSE,
} from './client'
import {
  emitAlertFired,
  emitAlertProposed,
  emitChatDelta,
  emitChatDone,
  emitChatEmptyResponse,
  emitChatError,
  emitChatUsage,
  emitNoteProposed,
  emitUserMsg,
} from '../lib/events'
import { cancelOne, syncAlertSchedule, tap } from '../lib/alertSchedule'
import { NotificationService } from '../../bindings/github.com/wailsapp/wails/v3/pkg/services/notifications'
import type {
  Alert,
  AlertProposal,
  AskResult,
  CaptureResult,
  ChatEvent,
  Conversation,
  CreateAlertResult,
  Delivery,
  GraphData,
  Message,
  ModelInfo,
  Note,
  SearchResult,
  SummarizeResult,
  TidyResult,
  UserPrefs,
} from './types'

// --- Auth ---

export async function signup(email: string, password: string): Promise<void> {
  const data = await request<{ accessToken: string; refreshToken: string }>('POST', '/v1/auth/signup', {
    body: { email, password },
  })
  setTokens(data.accessToken, data.refreshToken)
}

export async function login(email: string, password: string): Promise<void> {
  const data = await request<{ accessToken: string; refreshToken: string }>('POST', '/v1/auth/login', {
    body: { email, password },
  })
  setTokens(data.accessToken, data.refreshToken)
}

export function logout(): void {
  clearTokens()
}

export { isAuthed, loadTokens, AuthError, ApiError, setBaseURL }

// --- Models & Prefs ---

export const Models = {
  list(): Promise<ModelInfo[]> {
    return request<ModelInfo[]>('GET', '/v1/models')
  },
}

export const Prefs = {
  get(): Promise<UserPrefs> {
    return request<UserPrefs>('GET', '/v1/prefs')
  },
  update(patch: Partial<UserPrefs>): Promise<UserPrefs> {
    // O servidor espera campos opcionais (ponteiros); mandamos só os alterados.
    const body: Record<string, unknown> = {}
    if (patch.model !== undefined) body.model = patch.model
    if (patch.language !== undefined) body.language = patch.language
    if (patch.systemPrompt !== undefined) body.systemPrompt = patch.systemPrompt
    if (patch.charLimit !== undefined) body.charLimit = patch.charLimit
    if (patch.timezone !== undefined) body.timezone = patch.timezone
    return request<UserPrefs>('PUT', '/v1/prefs', { body })
  },
}

// --- Notes ---

export const NotesService = {
  list(): Promise<Note[]> {
    return request<Note[]>('GET', '/v1/notes')
  },
  get(id: number): Promise<Note> {
    return request<Note>('GET', `/v1/notes/${id}`)
  },
  update(id: number, title: string, content: string, tags: string[]): Promise<Note> {
    return request<Note>('PUT', `/v1/notes/${id}`, { body: { title, content, tags } })
  },
  delete(id: number): Promise<void> {
    return request<void>('DELETE', `/v1/notes/${id}`)
  },
  setCharLimit(id: number, limit: number): Promise<void> {
    return request<void>('PUT', `/v1/notes/${id}/char-limit`, { body: { limit } })
  },
  capture(text: string): Promise<CaptureResult> {
    return request<CaptureResult>('POST', '/v1/notes/capture', { body: { text } })
  },
  createFromProposal(title: string, content: string, tags: string[]): Promise<CaptureResult> {
    return request<CaptureResult>('POST', '/v1/notes', { body: { title, content, tags } })
  },
  appendTo(targetId: number, content: string, tags: string[]): Promise<CaptureResult> {
    return request<CaptureResult>('POST', `/v1/notes/${targetId}/append`, { body: { content, tags } })
  },
  resolveOverflow(targetId: number, content: string, tags: string[], mode: string): Promise<CaptureResult> {
    return request<CaptureResult>('POST', `/v1/notes/${targetId}/overflow`, { body: { content, tags, mode } })
  },
  find(query: string): Promise<SearchResult[]> {
    return request<SearchResult[]>('GET', '/v1/notes/find', { query: { q: query } })
  },
  ask(query: string): Promise<AskResult> {
    return request<AskResult>('GET', '/v1/notes/ask', { query: { q: query } })
  },
  summarize(id: number): Promise<SummarizeResult> {
    return request<SummarizeResult>('POST', `/v1/notes/${id}/summarize`)
  },
  tidy(id: number): Promise<TidyResult> {
    return request<TidyResult>('POST', `/v1/notes/${id}/tidy`)
  },
  graph(): Promise<GraphData> {
    return request<GraphData>('GET', '/v1/notes/graph')
  },
}

// --- Alerts ---

export const AlertsService = {
  list(): Promise<Alert[]> {
    return request<Alert[]>('GET', '/v1/alerts')
  },
  create(text: string): Promise<CreateAlertResult> {
    return tap(request<CreateAlertResult>('POST', '/v1/alerts/parse', { body: { text } }), () => {
      console.log('[services] alert created via /parse, scheduling...')
      syncAlertSchedule(AlertsService.list).catch((e) => { console.error('[services] create alert schedule FAILED:', e) })
    })
  },
  createProposed(message: string, fireAt: string, recurrence: string, noteId: number | null): Promise<CreateAlertResult> {
    return tap(request<CreateAlertResult>('POST', '/v1/alerts', { body: { message, fireAt, recurrence, noteId } }), () => {
      console.log('[services] alert created via createProposed, scheduling...')
      syncAlertSchedule(AlertsService.list).catch((e) => { console.error('[services] createProposed alert schedule FAILED:', e) })
    })
  },
  createForNote(noteId: number, whenText: string): Promise<CreateAlertResult> {
    return tap(request<CreateAlertResult>('POST', `/v1/notes/${noteId}/alert`, { body: { text: whenText } }), () => {
      console.log('[services] alert created via createForNote, scheduling...')
      syncAlertSchedule(AlertsService.list).catch((e) => { console.error('[services] createForNote alert schedule FAILED:', e) })
    })
  },
  done(id: number): Promise<void> {
    return tap(request<void>('POST', `/v1/alerts/${id}/done`), () => {
      console.log(`[services] alert ${id} done, cancelling schedule...`)
      cancelOne(id).catch((e) => { console.error(`[services] done cancel ${id} FAILED:`, e) })
    })
  },
  cancel(id: number): Promise<void> {
    return tap(request<void>('POST', `/v1/alerts/${id}/cancel`), () => {
      console.log(`[services] alert ${id} cancelled, cancelling schedule...`)
      cancelOne(id).catch((e) => { console.error(`[services] cancel ${id} FAILED:`, e) })
    })
  },
  snooze(id: number, minutes: number): Promise<void> {
    return tap(request<void>('POST', `/v1/alerts/${id}/snooze`, { body: { minutes } }), () => {
      console.log(`[services] alert ${id} snoozed, rescheduling...`)
      syncAlertSchedule(AlertsService.list).catch((e) => { console.error('[services] snooze alert schedule FAILED:', e) })
    })
  },
}

// --- History ---

export const HistoryService = {
  list(): Promise<Conversation[]> {
    return request<Conversation[]>('GET', '/v1/conversations')
  },
  messages(id: number): Promise<Message[]> {
    return request<Message[]>('GET', `/v1/conversations/${id}/messages`)
  },
  delete(id: number): Promise<void> {
    return request<void>('DELETE', `/v1/conversations/${id}`)
  },
}

// --- Chat (stream SSE) ---

let chatController: AbortController | null = null

export const ChatService = {
  // Send posta no /v1/chat e consome o SSE, traduzindo cada evento para os
  // dispatchers de events.ts (chat:delta/done/error/usage, note/alert:proposed).
  async send(text: string, conversationId = 0): Promise<void> {
    chatController?.abort()
    chatController = new AbortController()
    emitUserMsg(text)
    try {
      await streamSSE(
        '/v1/chat',
        { conversationId, text },
        (eventName, data) => {
          const ev = parseChatEvent(eventName, data)
          if (!ev) return
          switch (ev.type) {
            case 'delta': emitChatDelta(ev.delta); break
            case 'done':
              emitChatDone(ev.content)
              if (!ev.content) {
                emitChatEmptyResponse(text)
              }
              break
            case 'error': emitChatError(ev.err); break
            case 'usage': emitChatUsage(ev.usage); break
            case 'note_proposed':
              emitNoteProposed({ title: ev.note.noteTitle, content: ev.note.content, tags: ev.note.tags ?? [] })
              break
            case 'alert_proposed':
              emitAlertProposed(ev.alert as AlertProposal)
              break
          }
        },
        chatController.signal,
      )
    } catch (err: any) {
      // Abort não é erro para o usuário (Cancel).
      if (err?.name === 'AbortError') return
      emitChatError(err?.message ?? 'erro')
    }
  },
  cancel(): void {
    chatController?.abort()
    chatController = null
  },
  // NewConversation era um reset local no Go; agora é só estado do frontend
  // (conversationId=0 inicia nova conversa no servidor). Mantido p/ compat.
  newConversation(): void {
    chatController?.abort()
  },
}

function parseChatEvent(eventName: string, data: string): ChatEvent | null {
  switch (eventName) {
    case 'delta': return { type: 'delta', delta: JSON.parse(data) as string }
    case 'done': return { type: 'done', content: JSON.parse(data) as string }
    case 'error': return { type: 'error', err: JSON.parse(data) as string }
    case 'usage': return { type: 'usage', usage: JSON.parse(data) as { tokens: number; cost: number } }
    case 'note_proposed': return { type: 'note_proposed', note: JSON.parse(data) as CaptureResult }
    case 'alert_proposed': return { type: 'alert_proposed', alert: JSON.parse(data) as AlertProposal }
    default: return null
  }
}

// --- Push (stream SSE de longa duração para disparos de alerta) ---

let pushController: AbortController | null = null

// startPush abre o GET /v1/push e, para cada entrega, ergue alert:fired (card
// no chat) e mostra um toast nativo (via serviço de notificações do Wails, se
// disponível). Reconecta em 5s se o stream cair.
export function startPush(): void {
  stopPush()
  const controller = new AbortController()
  pushController = controller
  const loop = async () => {
    while (!controller.signal.aborted) {
      try {
        await streamSSE(
          '/v1/push',
          {},
          (eventName, data) => {
            if (eventName !== 'alert') return
            const d = JSON.parse(data) as Delivery
            emitAlertFired({ id: d.alertId ?? 0, message: d.message, noteId: d.noteId })
            // Push assumiu esta ocorrência: desarma o toast do SO p/ não duplicar.
            if (d.alertId) void cancelOne(d.alertId)
            // Toast nativo: best-effort (o serviço pode não estar registrado).
            try { showDeliveryToast(d) } catch { /* sem toast, segue */ }
          },
          controller.signal,
        )
      } catch (err: any) {
        if (err?.name === 'AbortError') return
        // reconecta
        await new Promise((r) => setTimeout(r, 5000))
      }
    }
  }
  void loop()
}

export function stopPush(): void {
  pushController?.abort()
  pushController = null
}

// showDeliveryToast ergue uma notificação do SO via serviço de notificações do
// Wails (registrado em shell.go). Import dinâmico p/ não quebrar se o binding
// não existir (ex.: build sem o serviço).
async function showDeliveryToast(d: Delivery) {
  try {
    NotificationService.SendNotification({ id: `gix-alert-${d.deliveryId}`, title: 'gix', body: d.message })
  } catch (e) {
    console.error('services.showDeliveryToast:', e)
  }
}

export { getRefreshToken }
