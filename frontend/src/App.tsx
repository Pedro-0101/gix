import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Window } from '@wailsio/runtime'
import { motion } from 'motion/react'
import { ChatService, ConfigService } from '../bindings/gix/internal/app'
import { onChatDelta, onChatDone, onChatError, onChatUsage, onWindowShown } from './lib/events'
import { MessageCard } from './components/MessageCard'
import { SettingsView } from './views/SettingsView'
import { HistoryView } from './views/HistoryView'
import { tr } from './i18n'

type View = 'chat' | 'settings' | 'history'
type Msg = { role: 'user' | 'assistant'; content: string; pending?: boolean }

// Must match the Go side (internal/app/shell.go).
const WIDTH = 680
const TOP_MAX_RATIO = 0.6

export default function App() {
  const [view, setView] = useState<View>('chat')
  const [lang, setLang] = useState('pt')
  const [theme, setTheme] = useState('light')
  const [opacity, setOpacity] = useState(85)
  const [msgs, setMsgs] = useState<Msg[]>([])
  const [input, setInput] = useState('')
  const [usage, setUsage] = useState<{ tokens: number; cost: number } | null>(null)
  const [streaming, setStreaming] = useState(false)
  const [nonce, setNonce] = useState(0) // bumped on every window show to replay the enter animation

  const rootRef = useRef<HTMLDivElement>(null)
  const barRef = useRef<HTMLDivElement>(null)
  const endRef = useRef<HTMLDivElement>(null)
  const taRef = useRef<HTMLTextAreaElement>(null)

  const expanded = view !== 'chat' || msgs.length > 0
  const maxH = Math.round((window.screen?.availHeight || 900) * TOP_MAX_RATIO)
  const panelMax = Math.max(180, maxH - (barRef.current?.offsetHeight ?? 64))

  const loadCfg = () => ConfigService.Get().then((c: any) => {
    setLang(c.language); setTheme(c.theme)
    if (typeof c.opacity === 'number') {
      setOpacity(c.opacity)
    }
  }).catch(() => {})
  useEffect(() => { loadCfg() }, [])
  useEffect(() => { document.documentElement.dataset.theme = theme }, [theme])

  // Resize the OS window to match the content, anchored at the top so it grows
  // downward. Width is fixed, so content height never depends on window height
  // (no resize feedback loop).
  const rafRef = useRef(0)
  const fit = useCallback(() => {
    cancelAnimationFrame(rafRef.current)
    rafRef.current = requestAnimationFrame(() => {
      const el = rootRef.current
      if (!el) return
      Window.SetSize(WIDTH, Math.ceil(el.getBoundingClientRect().height))
    })
  }, [])

  useLayoutEffect(() => { fit() }, [fit, expanded, view, msgs, usage, nonce])

  // Re-attach on `nonce` change: the root motion.div is keyed, so showing the
  // window again remounts it and `rootRef` points at a fresh node.
  useEffect(() => {
    const el = rootRef.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(() => fit())
    ro.observe(el)
    return () => ro.disconnect()
  }, [fit, nonce])

  // Streaming wiring.
  useEffect(() => {
    const offDelta = onChatDelta((delta) => {
      setMsgs((m) => {
        const copy = [...m]
        const i = copy.length - 1
        const last = copy[i]
        if (last && last.role === 'assistant') copy[i] = { ...last, content: last.content + delta, pending: false }
        return copy
      })
    })
    const offDone = onChatDone((d) => {
      setStreaming(false)
      setMsgs((m) => {
        const copy = [...m]
        const i = copy.length - 1
        const last = copy[i]
        if (last && last.role === 'assistant') copy[i] = { ...last, content: d.content, pending: false }
        return copy
      })
    })
    const offErr = onChatError((code) => {
      setStreaming(false)
      setMsgs((m) => {
        const copy = [...m]
        const i = copy.length - 1
        const last = copy[i]
        const text = code === 'no_api_key' ? tr(lang, 'no_api_key') : `${tr(lang, 'error_prefix')}${code}`
        if (last && last.role === 'assistant') copy[i] = { ...last, content: text, pending: false }
        return copy
      })
    })
    const offUsage = onChatUsage((u) => setUsage(u))
    return () => { offDelta(); offDone(); offErr(); offUsage() }
  }, [lang])

  // Each time the window is shown, reset to a clean bar and focus it.
  useEffect(() => {
    const off = onWindowShown(() => {
      setView('chat'); setMsgs([]); setUsage(null); setInput(''); setStreaming(false)
      setNonce((n) => n + 1)
      requestAnimationFrame(() => taRef.current?.focus())
    })
    requestAnimationFrame(() => taRef.current?.focus())
    return () => { off() }
  }, [])

  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [msgs])

  // Auto-grow the input up to ~5 lines.
  useEffect(() => {
    const ta = taRef.current
    if (!ta) return
    ta.style.height = 'auto'
    ta.style.height = Math.min(ta.scrollHeight, 132) + 'px'
  }, [input])

  // Esc once leaves settings/history; Esc twice (within 500ms) hides the window.
  useEffect(() => {
    let last = 0
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      const now = Date.now()
      const double = now - last < 500
      last = now
      if (double) { ChatService.Cancel(); Window.Hide(); return }
      if (view !== 'chat') setView('chat')
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [view])

  const send = () => {
    const text = input.trim()
    if (!text) return
    const cmd = text.toLowerCase()
    if (cmd === '/new' || cmd === '/limpar') {
      ChatService.NewConversation(); setMsgs([]); setUsage(null); setInput(''); setView('chat'); return
    }
    if (cmd === '/config' || cmd === '/configuracoes' || cmd === '/settings') { setView('settings'); setInput(''); return }
    if (cmd === '/historico' || cmd === '/history') { setView('history'); setInput(''); return }
    setView('chat')
    setMsgs((m) => [...m, { role: 'user', content: text }, { role: 'assistant', content: '', pending: true }])
    setStreaming(true)
    ChatService.Send(text)
    setInput('')
  }

  const item = {
    hidden: { opacity: 0, y: 8, filter: 'blur(4px)' },
    show: { opacity: 1, y: 0, filter: 'blur(0px)', transition: { type: 'spring' as const, duration: 0.3, bounce: 0 } },
  }

  return (
    <motion.div
      key={nonce}
      ref={rootRef}
      initial="hidden"
      animate="show"
      variants={{ show: { transition: { staggerChildren: 0.07 } } }}
      className="flex flex-col overflow-hidden rounded-xl text-fg"
      style={{
        maxHeight: maxH,
        background: `rgba(${theme === 'dark' ? '0,0,0' : '255,255,255'} / ${opacity / 100 * 0.3})`,
        boxShadow: 'var(--shell-shadow)'
      }}
    >
      {/* ----- The always-visible input bar (drag handle for the window). ----- */}
      <motion.div
        ref={barRef}
        variants={item}
        className="flex shrink-0 items-end gap-2 px-3 py-2.5 [--wails-draggable:drag]"
      >
        <span className="grid size-7 shrink-0 place-items-center self-center text-muted [--wails-draggable:no-drag]">
          {streaming ? <Spinner /> : <PromptIcon />}
        </span>
        <textarea
          ref={taRef}
          className="max-h-[132px] flex-1 resize-none self-center bg-transparent py-1 text-[15px] leading-relaxed text-fg outline-none placeholder:text-muted/70 [--wails-draggable:no-drag]"
          rows={1}
          value={input}
          placeholder={tr(lang, 'placeholder')}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() } }}
        />
        <button
          onClick={send}
          disabled={!input.trim()}
          aria-label={tr(lang, 'placeholder')}
          className="grid size-8 shrink-0 self-center place-items-center rounded-field bg-accent text-white outline-none transition-[scale,opacity] duration-150 ease-out [--wails-draggable:no-drag] hover:brightness-110 active:not-disabled:scale-[0.96] disabled:opacity-40 focus-visible:shadow-[0_0_0_2px_var(--shell-bg),0_0_0_4px_var(--ring-focus)]"
        >
          <ArrowIcon />
        </button>
      </motion.div>

      {/* ----- The panel that expands downward with the answer / view. ----- */}
      {expanded && (
        <motion.div
          initial={{ opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ type: 'spring', duration: 0.3, bounce: 0 }}
          className={`min-h-0 border-t border-[color:var(--shell-border)] selectable ${view === 'history' ? 'overflow-hidden' : 'overflow-y-auto'}`}
          style={view === 'history' ? { height: panelMax } : { maxHeight: panelMax }}
        >
          {view === 'chat' && (
            <div className="space-y-3 px-3 py-3">
              {usage && (
                <div className="flex items-center gap-2 font-mono text-xs text-muted tabular-nums">
                  <span>{usage.tokens} tokens</span>
                  <span className="opacity-40">·</span>
                  <span>${usage.cost.toFixed(6)}</span>
                </div>
              )}
              {msgs.map((m, i) => (
                <MessageCard key={i} role={m.role}
                  content={m.pending ? tr(lang, 'thinking') : m.content}
                  pending={m.pending}
                  label={m.role === 'user' ? tr(lang, 'you') : tr(lang, 'ai')} />
              ))}
              <div ref={endRef} />
            </div>
          )}
          {view === 'settings' && <SettingsView lang={lang} onClose={() => { loadCfg(); setView('chat') }} />}
          {view === 'history' && <HistoryView lang={lang} onClose={() => setView('chat')} />}
        </motion.div>
      )}
    </motion.div>
  )
}

function PromptIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2"
      strokeLinecap="round" strokeLinejoin="round" className="size-4">
      <path d="M9 7l5 5-5 5" />
    </svg>
  )
}

function ArrowIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" className="size-4 -ml-px">
      <path d="M12 19V5M6 11l6-6 6 6" />
    </svg>
  )
}

function Spinner() {
  return (
    <motion.svg viewBox="0 0 24 24" fill="none" className="size-4 text-accent"
      animate={{ rotate: 360 }} transition={{ duration: 0.8, repeat: Infinity, ease: 'linear' }}>
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="2.5" strokeOpacity="0.2" />
      <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
    </motion.svg>
  )
}
