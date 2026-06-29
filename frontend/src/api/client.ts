// Cliente HTTP do gix-server: fetch com Bearer token (JWT), refresh automático
// em 401 e baseURL configurável (vinda do config local de desktop).
//
// O token de acesso e o refresh token são mantidos em localStorage. O refresh
// é serializado (uma chamada por vez) para evitar corrida quando vários
// requests pegam 401 ao mesmo tempo.

import { emitAuthError } from '../lib/events'

const ACCESS_KEY = 'gix.accessToken'
const REFRESH_KEY = 'gix.refreshToken'

let baseURL = 'http://localhost:3000'

// Os tokens vivem em memória (acesso síncrono nos requests) e são persistidos
// cifrados no cofre de sessão do desktop (DPAPI no Windows) via TokenService —
// não em localStorage. loadTokens() hidrata a memória na inicialização.
let accessToken: string | null = null
let refreshToken: string | null = null

export function setBaseURL(url: string) {
  baseURL = url.replace(/\/$/, '')
}

export function getBaseURL(): string {
  return baseURL
}

export function getAccessToken(): string | null {
  return accessToken
}

export function getRefreshToken(): string | null {
  return refreshToken
}

export function setTokens(access: string, refresh: string) {
  accessToken = access
  refreshToken = refresh
  void persist()
}

export function clearTokens() {
  accessToken = null
  refreshToken = null
  void persist()
}

export function isAuthed(): boolean {
  return !!accessToken
}

// persist grava (ou limpa, com par vazio) o blob no cofre nativo. Best-effort:
// sem o binding (build sem o serviço, ou ambiente de teste) a sessão segue só
// em memória até o app fechar.
async function persist(): Promise<void> {
  try {
    const mod: any = await import('../../bindings/gix/internal/app')
    const blob = accessToken && refreshToken ? JSON.stringify({ a: accessToken, r: refreshToken }) : ''
    await mod?.TokenService?.Save?.(blob)
  } catch { /* sem cofre nativo — segue em memória */ }
}

// loadTokens hidrata os tokens do cofre na inicialização. Faz uma migração única
// de sessões antigas guardadas em localStorage (antes do cofre) e o limpa.
export async function loadTokens(): Promise<void> {
  try {
    const mod: any = await import('../../bindings/gix/internal/app')
    const blob: string = (await mod?.TokenService?.Load?.()) ?? ''
    if (blob) {
      const t = JSON.parse(blob)
      accessToken = t.a ?? null
      refreshToken = t.r ?? null
      return
    }
  } catch { /* cofre indisponível — tenta migrar do localStorage abaixo */ }
  migrateFromLocalStorage()
}

function migrateFromLocalStorage(): void {
  try {
    const a = localStorage.getItem(ACCESS_KEY)
    const r = localStorage.getItem(REFRESH_KEY)
    if (a && r) {
      accessToken = a
      refreshToken = r
      localStorage.removeItem(ACCESS_KEY)
      localStorage.removeItem(REFRESH_KEY)
      void persist()
    }
  } catch { /* sem localStorage — ignora */ }
}

let refreshing: Promise<boolean> | null = null

// refresh troca o refresh token corrente por um novo par (rotação). Retorna
// true se conseguiu. É singleton: chamadas concorrentes compartilham o mesmo
// refresh em voo.
async function refresh(): Promise<boolean> {
  if (refreshing) return refreshing
  refreshing = (async () => {
    const rt = getRefreshToken()
    if (!rt) return false
    const res = await fetch(`${baseURL}/v1/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refreshToken: rt }),
    })
    if (!res.ok) {
      clearTokens()
      return false
    }
    const data = await res.json()
    setTokens(data.accessToken, data.refreshToken)
    return true
  })().finally(() => { refreshing = null })
  return refreshing
}

// AuthError sinaliza 401 definitivo (refresh falhou) para a UI poder mostrar
// a tela de login.
export class AuthError extends Error {}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// request faz uma chamada JSON autenticada. Em 401, tenta um refresh e re-tenta
// uma vez; se ainda falhar, lança AuthError.
export async function request<T>(
  method: string,
  path: string,
  opts: { body?: unknown; query?: Record<string, string | number | undefined> } = {},
): Promise<T> {
  const url = new URL(`${baseURL}${path}`)
  if (opts.query) {
    for (const [k, v] of Object.entries(opts.query)) {
      if (v !== undefined && v !== null) url.searchParams.set(k, String(v))
    }
  }

  const doFetch = async (token: string | null): Promise<Response> => {
    const headers: Record<string, string> = {}
    if (opts.body !== undefined) headers['Content-Type'] = 'application/json'
    if (token) headers['Authorization'] = `Bearer ${token}`
    return fetch(url.toString(), {
      method,
      headers,
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    })
  }

  const hadToken = !!getAccessToken()
  let res = await doFetch(getAccessToken())
  if (res.status === 401) {
    if (await refresh()) {
      res = await doFetch(getAccessToken())
    }
    if (!res.ok && res.status === 401) {
      // Só sinaliza expiração de sessão se já havia um token (não numa
      // tentativa de login, onde o 401 é credencial errada).
      if (hadToken) emitAuthError()
      throw new AuthError('não autorizado')
    }
  }

  if (!res.ok) {
    let msg = 'erro interno'
    try { msg = (await res.text()) || msg } catch { /* keep default */ }
    throw new ApiError(res.status, msg)
  }
  if (res.status === 204) return undefined as T
  const ct = res.headers.get('Content-Type') ?? ''
  if (!ct.includes('application/json')) return undefined as T
  return (await res.json()) as T
}

// streamSSE faz uma chamada POST autenticada cuja resposta é text/event-stream
// e dispersa cada evento nomeado via `onEvent`. Em 401, tenta refresh e
// re-tenta. O AbortSignal permita cancelar (ChatService.Cancel).
export async function streamSSE(
  path: string,
  body: unknown,
  onEvent: (eventName: string, data: string) => void,
  signal?: AbortSignal,
): Promise<void> {
  const doFetch = async (token: string | null): Promise<Response> => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    }
    if (token) headers['Authorization'] = `Bearer ${token}`
    return fetch(`${baseURL}${path}`, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal,
    })
  }

  const hadToken = !!getAccessToken()
  let res = await doFetch(getAccessToken())
  if (res.status === 401) {
    if (await refresh()) {
      res = await doFetch(getAccessToken())
    }
    if (!res.ok && res.status === 401) {
      if (hadToken) emitAuthError()
      throw new AuthError('não autorizado')
    }
  }
  if (!res.ok || !res.body) {
    throw new ApiError(res.status, 'streaming falhou')
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    // SSE: eventos separados por "\n\n"; cada um com linhas "event:"/"data:".
    let sep: number
    while ((sep = buffer.indexOf('\n\n')) !== -1) {
      const chunk = buffer.slice(0, sep)
      buffer = buffer.slice(sep + 2)
      let event = 'message'
      let data = ''
      for (const line of chunk.split('\n')) {
        if (line.startsWith('event:')) event = line.slice(6).trim()
        else if (line.startsWith('data:')) data += line.slice(5).trim()
      }
      if (event !== '') onEvent(event, data)
    }
  }
}
