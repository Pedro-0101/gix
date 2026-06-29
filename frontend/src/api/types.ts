// Tipos do contrato wire do gix-server (camelCase). Espelham
// internal/core/types.go do servidor — manter estável.

export type Usage = { tokens: number; cost: number }

export interface Note {
  id: number
  title: string
  content: string
  tags: string[]
  charLimit: number
  createdAt: string
  updatedAt: string
}

export interface Alert {
  id: number
  message: string
  noteId: number | null
  fireAt: string
  recurrence: string
  status: string
  createdAt: string
}

export interface Conversation {
  id: number
  title: string
  model: string
  createdAt: string
}

export interface Message {
  id: number
  role: string
  content: string
}

export interface SearchResult {
  noteId: number
  title: string
  snippet: string
  content: string
  tags: string[]
  score: number
}

export interface AlertProposal {
  message: string
  fireAt: string
  recurrence: string
}

export interface AttachProposal {
  targetId: number
  targetTitle: string
}

export interface OverflowProposal {
  targetId: number
  targetTitle: string
  length: number
  limit: number
}

export interface CaptureResult {
  status: string
  noteId: number
  noteTitle: string
  content: string
  tags: string[]
  message: string
  usage: Usage
  count: number
  alert: AlertProposal | null
  attach: AttachProposal | null
  overflow: OverflowProposal | null
}

export interface AskResult {
  status: string
  summary: string
  sources: SearchResult[]
  message: string
  usage: Usage
}

export interface SummarizeResult {
  status: string
  summary: string
  message: string
  usage: Usage
}

export interface TidyResult {
  status: string
  content: string
  message: string
  usage: Usage
}

export interface CreateAlertResult {
  status: string
  alertId: number
  message: string
  fireAtLocal: string
  recurrence: string
  usage: Usage
}

export interface GraphData {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface GraphNode {
  id: number
  title: string
  tags: string[]
}

export interface GraphEdge {
  source: number
  target: number
}

export interface ModelInfo {
  id: string
  inputPrice: number
  outputPrice: number
}

export interface UserPrefs {
  model: string
  language: string
  systemPrompt: string
  charLimit: number
  timezone: string
}

// Config local do desktop (prefs de shell). Vem do ConfigService Wails, não do
// servidor. Mantida aqui só para que o frontend tenha um tipo canônico.
export interface DesktopConfig {
  theme: string
  language: string
  open_key: string
  open_interval_ms: number
  open_press_count: number
  close_key: string
  close_interval_ms: number
  close_press_count: number
  opacity: number
  server_url: string
}

// Eventos do stream de chat (SSE do POST /v1/chat).
export type ChatEvent =
  | { type: 'delta'; delta: string }
  | { type: 'done'; content: string }
  | { type: 'error'; err: string }
  | { type: 'usage'; usage: Usage }
  | { type: 'note_proposed'; note: CaptureResult }
  | { type: 'alert_proposed'; alert: AlertProposal }

// Entrega de alerta vinda do stream de push (SSE do GET /v1/push).
export interface Delivery {
  deliveryId: number
  alertId: number | null
  message: string
  noteId: number | null
  fireAt: string
}
