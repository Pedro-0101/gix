import { useEffect, useState } from 'react'
import { ConfigService } from '../../bindings/gix/internal/app'
import { isAuthed, loadTokens, setBaseURL, startPush, stopPush } from '../api/services'
import { onAuthError } from './events'

// useSession governa o estado de sessão do desktop: hidrata os tokens do cofre
// nativo (DPAPI) na inicialização, escuta expiração (onAuthError → volta p/
// login) e mantém o stream de push SSE ligado só enquanto há sessão (garantindo
// o server_url do config local antes de abrir o stream). `ready` cobre a janela
// assíncrona da hidratação para o App não piscar a tela de login.
export function useSession() {
  const [authed, setAuthed] = useState(false)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    let alive = true
    loadTokens().finally(() => {
      if (!alive) return
      setAuthed(isAuthed())
      setReady(true)
    })
    const off = onAuthError(() => setAuthed(false))
    return () => { alive = false; off() }
  }, [])

  useEffect(() => {
    if (!authed) return
    let stopped = false
    ConfigService.Get().then((c: any) => {
      if (stopped) return
      if (c.server_url) setBaseURL(c.server_url)
      startPush()
    }).catch(() => {})
    return () => { stopped = true; stopPush() }
  }, [authed])

  return { authed, setAuthed, ready }
}
