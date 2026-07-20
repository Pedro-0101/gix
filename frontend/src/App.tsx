import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'
import { Window } from '@wailsio/runtime'
import { motion } from 'motion/react'
import { ConfigService } from '../bindings/gix/internal/app'
import { ChatService, setBaseURL } from './api/services'
import { onWindowShown, onAlertFired, onAlertOpen } from './lib/events'
import { LoginView } from './views/LoginView'
import { useSession } from './lib/useSession'

import { frostColor } from './lib/frost'
import { InputBar } from './components/InputBar'
import { ShellPanel } from './components/ShellPanel'
import { resolveCommand } from './commands/registry'
import type { SearchState, View } from './commands/types'
import { analyzeBar } from './commands/highlight'
import { record as recordPrompt, prev as prevPrompt, next as nextPrompt, detach as detachPrompt } from './commands/promptHistory'
import { loadPromptHistory, savePromptHistory } from './lib/promptStore'
import { tr } from './i18n'
import { useReveal } from './lib/reveal'
import { useChat } from './lib/useChat'
import { useInteraction } from './lib/useInteraction'
import { useWindowFit } from './lib/useWindowFit'
import { useCommandContext } from './lib/useCommandContext'
import { useProposals } from './lib/useProposals'
import { useEmptyResponse } from './lib/useEmptyResponse'
import { useInteractionLog } from './lib/useInteractionLog'

// Must match the Go side (internal/app/shell.go).
const TOP_MAX_RATIO = 0.6

export default function App() {
  const [view, setView] = useState<View>('chat')
  const [searchState, setSearchState] = useState<SearchState | null>(null)
  const [alertFocusId, setAlertFocusId] = useState<number | null>(null)
  const [lang, setLang] = useState('pt')
  const [theme, setTheme] = useState('light')
  const [opacity, setOpacity] = useState(85) // background frost strength, 0–100
  const [input, setInput] = useState('')
  const [nonce, setNonce] = useState(0) // bumped on every window show to replay the enter animation
  const [revealKey, setRevealKey] = useState(0)
  const [pendingNoteId, setPendingNoteId] = useState<number | null>(null)
  const { authed, setAuthed, ready } = useSession()

  const rootRef = useRef<HTMLDivElement>(null)
  const barRef = useRef<HTMLDivElement>(null)
  const endRef = useRef<HTMLDivElement>(null)
  const taRef = useRef<HTMLTextAreaElement>(null)
  const overlayRef = useRef<HTMLDivElement>(null)
  // Recall history for the bar (oldest → newest submitted prompts), in a ref since
  // it drives the input through setInput and doesn't itself need a re-render.
  const historyRef = useRef(loadPromptHistory())
  // Always-current language, so a running command (e.g. /config) sees a language
  // change mid-flow instead of the value captured when it started.
  const langRef = useRef(lang)
  useEffect(() => { langRef.current = lang }, [lang])

  const { msgs, setMsgs, usage, setUsage, streaming, setStreaming } = useChat(lang)
  const {
    interaction, setInteraction, choose, prompt, slider,
    pickChoice, submitPrompt, submitSlider, reset,
  } = useInteraction({ input, setInput, setMsgs, setView, taRef })

  const { log: debugLog, clear: debugClear, paused: debugPaused, setPaused: setDebugPaused } = useInteractionLog()

  const expanded = view !== 'chat' || msgs.length > 0 || interaction != null
  const maxH = Math.round((window.screen?.availHeight || 900) * TOP_MAX_RATIO)
  const panelMax = Math.max(180, maxH - (barRef.current?.offsetHeight ?? 64))
  // Frost overlay over the native Acrylic backdrop: a translucent white veil whose
  // strength follows the Opacity setting. Both themes lift toward white (never a
  // black overlay), so raising Opacity always reads brighter/more legible — dark
  // mode no longer looks like a near-black slab. Users tune it via /config →
  // Opacidade or the Settings slider. See lib/frost.ts.
  const shellBg = frostColor(theme, opacity)

  const loadCfg = () => ConfigService.Get().then((c) => {
    if (!c) return // config ausente (primeiro boot): mantém os defaults
    langRef.current = c.language // sync now so an awaiting command sees it immediately
    setLang(c.language); setTheme(c.theme); setOpacity(c.opacity)
    if (c.server_url) setBaseURL(c.server_url)
  }).catch(() => {})
  useEffect(() => { loadCfg() }, [])
  useEffect(() => { document.documentElement.dataset.theme = theme }, [theme])

  const fit = useWindowFit(rootRef, nonce)
  useLayoutEffect(() => { fit() }, [fit, expanded, view, msgs, usage, nonce])

  // Live alert wiring: a fired alert posts a system card; clicking a toast body
  // (alert:open) jumps to the alerts view focused on that alert, or to the linked note.
  useEffect(() => {
    const offFired = onAlertFired((a) => {
      setView('chat')
      setMsgs((m) => [...m, { role: 'system', content: `${tr(lang, 'alert_created')} **${a.message}**` }])
    })
    const offOpen = onAlertOpen(({ id, noteId }) => {
      if (noteId) {
        setPendingNoteId(noteId)
        setView('notes')
      } else {
        setAlertFocusId(id)
        setView('alerts')
      }
    })
    return () => { offFired(); offOpen() }
  }, [lang, setMsgs])

  // The capability surface commands act through (see lib/useCommandContext.ts).
  const commandContext = useCommandContext({
    langRef, setView, setMsgs, setUsage, setSearchState, choose, prompt, slider, loadCfg,
  })
  // Ref sempre-atual do contexto, para efeitos (eventos) lerem a versão viva.
  const commandContextRef = useRef(commandContext)
  commandContextRef.current = commandContext

  // Proposed note/alert wiring: tool-call results from the AI propose creations;
  // the shell asks the user to confirm before saving them.
  useProposals({ setStreaming, setMsgs, commandContextRef })
  useEmptyResponse({ setStreaming, setMsgs, commandContextRef })

  // Each time the window is shown, reset to a clean bar and focus it.
  useEffect(() => {
    const off = onWindowShown(() => {
      reset()
      historyRef.current = detachPrompt(historyRef.current)
      setView('chat'); setMsgs([]); setUsage(null); setInput(''); setStreaming(false)
      setNonce((n) => n + 1)
      requestAnimationFrame(() => taRef.current?.focus())
    })
    requestAnimationFrame(() => taRef.current?.focus())
    return () => { off() }
  }, [reset, setMsgs, setUsage, setStreaming])

  // A última mensagem do assistente (não pendente, não-erro) é a que revela ao vivo.
  const lastIdx = msgs.length - 1
  const lastMsg = msgs[lastIdx]
  const activeTarget =
    lastMsg && lastMsg.role === 'assistant' && !lastMsg.pending && !lastMsg.instant ? lastMsg.content : ''
  const { shown, revealing } = useReveal(activeTarget, { done: !streaming, resetKey: revealKey })

  useEffect(() => { endRef.current?.scrollIntoView({ behavior: revealing ? 'auto' : 'smooth' }) }, [msgs, shown, revealing]) // 'auto' enquanto revela: suave reiniciaria a cada frame e nunca chegaria ao fim

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
      if (double) { ChatService.cancel(); Window.Hide(); return }
      if (view !== 'chat') { setView('chat'); setPendingNoteId(null) }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [view])

  const send = () => {
    if (streaming) return // block a second prompt while the previous answer streams
    const text = input.trim()
    if (!text) return
    historyRef.current = recordPrompt(historyRef.current, text)
    savePromptHistory(historyRef.current)
    const resolved = resolveCommand(text)
    if (resolved) { resolved.cmd.run(commandContext, resolved.arg); setInput(''); return }
    setView('chat')
    setMsgs((m) => [...m, { role: 'user', content: text }, { role: 'assistant', content: '', pending: true }])
    setRevealKey((k) => k + 1)
    setStreaming(true)
    ChatService.send(text)
    setInput('')
  }

  // Enquanto o cofre hidrata (assíncrono), não renderiza nada — a janela está
  // escondida no boot, então isso é invisível e evita piscar a tela de login.
  if (!ready) return null

  // Sem sessão, a paleta inteira dá lugar à tela de login (mesmo shell fosco e
  // mesmo rootRef, p/ o useWindowFit dimensionar a janela ao formulário).
  if (!authed) {
    return (
      <motion.div
        ref={rootRef}
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        className="flex flex-col overflow-hidden rounded-xl text-fg"
        style={{
          maxHeight: maxH,
          background: shellBg,
          ['--shell-bg' as any]: shellBg,
          boxShadow: 'var(--shell-shadow)',
        }}
      >
        <LoginView lang={lang} onAuthed={() => setAuthed(true)} />
      </motion.div>
    )
  }

  const bar = analyzeBar(input)

  // Drop a recalled prompt into the bar and park the caret at the end.
  const recallValue = (value: string) => {
    setInput(value)
    requestAnimationFrame(() => {
      const ta = taRef.current
      if (ta) ta.selectionStart = ta.selectionEnd = value.length
    })
  }

  const onBarKeyDown = (e: ReactKeyboardEvent<HTMLTextAreaElement>) => {
    // In prompt mode the bar collects a value; Enter submits it (Esc is handled
    // by the capture-phase interaction listener). No command completion here.
    if (interaction?.kind === 'prompt') {
      if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submitPrompt() }
      return
    }
    const ta = e.currentTarget
    const atEnd = ta.selectionStart === ta.value.length && ta.selectionEnd === ta.value.length
    // Tab anywhere, or → at the end of the text, accepts the name completion.
    if (bar.completion && (e.key === 'Tab' || (e.key === 'ArrowRight' && atEnd))) {
      e.preventDefault(); setInput(bar.accepted); return
    }
    // ↑/↓ recall earlier/later prompts, but only from the first/last line so
    // multi-line editing keeps normal caret movement.
    if (e.key === 'ArrowUp' && !ta.value.slice(0, ta.selectionStart).includes('\n')) {
      const r = prevPrompt(historyRef.current, ta.value)
      if (r.handled) { e.preventDefault(); historyRef.current = r.history; recallValue(r.value) }
      return
    }
    if (e.key === 'ArrowDown' && !ta.value.slice(ta.selectionEnd).includes('\n')) {
      const r = nextPrompt(historyRef.current)
      if (r.handled) { e.preventDefault(); historyRef.current = r.history; recallValue(r.value) }
      return
    }
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() }
  }

  return (
    <motion.div
      key={nonce}
      ref={rootRef}
      initial="hidden"
      animate="show"
      variants={{
        hidden: { opacity: 0 },
        show: { opacity: 1, transition: { staggerChildren: 0.07 } },
      }}
      className="flex flex-col overflow-hidden rounded-xl text-fg"
      style={{
        maxHeight: maxH,
        background: shellBg,
        ['--shell-bg' as any]: shellBg,
        boxShadow: 'var(--shell-shadow)',
      }}
    >
      <InputBar
        lang={lang}
        input={input}
        setInput={setInput}
        streaming={streaming}
        interaction={interaction}
        bar={bar}
        barRef={barRef}
        taRef={taRef}
        overlayRef={overlayRef}
        onDetachHistory={() => { historyRef.current = detachPrompt(historyRef.current) }}
        onKeyDown={onBarKeyDown}
        onSubmit={() => (interaction?.kind === 'prompt' ? submitPrompt() : interaction?.kind === 'slider' ? submitSlider() : send())}
      />

      {expanded && (
        <ShellPanel
          lang={lang}
          view={view}
          panelMax={panelMax}
          usage={usage}
          msgs={msgs}
          lastIdx={lastIdx}
          shown={shown}
          revealing={revealing}
          interaction={interaction}
          setInteraction={setInteraction}
          pickChoice={pickChoice}
          submitSlider={submitSlider}
          endRef={endRef}
          searchState={searchState}
          pendingNoteId={pendingNoteId}
          alertFocusId={alertFocusId}
          onCloseToChat={() => setView('chat')}
          onCloseClearNote={() => { setView('chat'); setPendingNoteId(null) }}
          onReloadCfgClose={() => { loadCfg(); setView('chat') }}
          onSelectNote={(id) => { setPendingNoteId(id); setView('notes') }}
          onCloseAlerts={() => { setAlertFocusId(null); setView('chat') }}
          debugLog={debugLog}
          debugPaused={debugPaused}
          onDebugTogglePause={() => setDebugPaused((p) => !p)}
          onDebugClear={debugClear}
          onDebugClose={() => setView('chat')}
        />
      )}
    </motion.div>
  )
}
