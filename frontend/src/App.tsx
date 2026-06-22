import { useEffect, useState } from 'react'
import { Window } from '@wailsio/runtime'
import { ChatView } from './views/ChatView'
import { SettingsView } from './views/SettingsView'
import { HistoryView } from './views/HistoryView'
import { ChatService, ConfigService } from '../bindings/gix/internal/app'

type View = 'chat' | 'settings' | 'history'

export default function App() {
  const [view, setView] = useState<View>('chat')
  const [lang, setLang] = useState('pt')
  const [theme, setTheme] = useState('light')

  const loadCfg = () => ConfigService.Get().then((c: any) => { setLang(c.language); setTheme(c.theme) })
  useEffect(() => { loadCfg() }, [])
  useEffect(() => { document.documentElement.dataset.theme = theme }, [theme])

  // Duplo-Esc esconde a janela e cancela streaming.
  useEffect(() => {
    let last = 0
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        const now = Date.now()
        if (now - last < 500) { ChatService.Cancel(); Window.Hide() }
        last = now
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="h-screen w-screen bg-bg text-fg">
      <div className="flex gap-2 px-2 py-1 bg-surface text-muted text-sm">
        <button onClick={() => setView('chat')}>Chat</button>
        <button onClick={() => setView('history')}>Histórico</button>
        <div className="flex-1" />
        <button onClick={() => setView('settings')}>⚙</button>
      </div>
      <div className="h-[calc(100%-2rem)]">
        {view === 'chat' && <ChatView lang={lang} />}
        {view === 'settings' && <SettingsView onClose={() => { loadCfg(); setView('chat') }} />}
        {view === 'history' && <HistoryView onClose={() => setView('chat')} />}
      </div>
    </div>
  )
}
