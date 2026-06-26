import { useCallback, useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'
import { Window } from '@wailsio/runtime'
import { motion } from 'motion/react'
import { AlertsService, ChatService, ConfigService, NotesService } from '../bindings/gix/internal/app'
import { onChatDelta, onChatDone, onChatError, onChatUsage, onWindowShown, onAlertFired, onAlertOpen } from './lib/events'
import { MessageCard } from './components/MessageCard'
import { ChoiceCard, ChoiceSummary } from './components/ChoiceCard'
import { Slider } from './components/Slider'
import { frostColor } from './lib/frost'
import { SettingsView } from './views/SettingsView'
import { HistoryView } from './views/HistoryView'
import { NotesView } from './views/NotesView'
import { SearchView } from './views/SearchView'
import { GraphView } from './views/GraphView'
import { AlertsView } from './views/AlertsView'
import { commands, resolveCommand, type CommandContext } from './commands/registry'
import type { SearchState } from './commands/types'
import { analyzeBar } from './commands/highlight'
import { moveSelection, type Interaction } from './commands/interaction'
import { emptyHistory, record as recordPrompt, prev as prevPrompt, next as nextPrompt, detach as detachPrompt, type PromptHistory } from './commands/promptHistory'
import { tr } from './i18n'
import { useReveal } from './lib/reveal'

type View = 'chat' | 'settings' | 'history' | 'notes' | 'search' | 'graph' | 'alerts'
type ChatMsg = { role: 'user' | 'assistant' | 'system'; content: string; pending?: boolean; instant?: boolean }
type ChoiceMsg = { role: 'choice'; title: string; chosenLabel: string }
type Msg = ChatMsg | ChoiceMsg

// Must match the Go side (internal/app/shell.go).
const WIDTH = 680
const TOP_MAX_RATIO = 0.6

// Submitted prompts persist across window shows and app restarts via localStorage,
// so ArrowUp recalls them like a shell history.
const PROMPT_HISTORY_KEY = 'gix.promptHistory'
const loadPromptHistory = (): PromptHistory => {
  try {
    const raw = localStorage.getItem(PROMPT_HISTORY_KEY)
    return emptyHistory(raw ? JSON.parse(raw) : [])
  } catch { return emptyHistory() }
}
const savePromptHistory = (h: PromptHistory) => {
  try { localStorage.setItem(PROMPT_HISTORY_KEY, JSON.stringify(h.entries)) } catch { /* storage unavailable */ }
}

export default function App() {
  const [view, setView] = useState<View>('chat')
  const [searchState, setSearchState] = useState<SearchState | null>(null)
  const [alertFocusId, setAlertFocusId] = useState<number | null>(null)
  const [lang, setLang] = useState('pt')
  const [theme, setTheme] = useState('light')
  const [opacity, setOpacity] = useState(85) // background frost strength, 0–100
  const [msgs, setMsgs] = useState<Msg[]>([])
  const [input, setInput] = useState('')
  const [usage, setUsage] = useState<{ tokens: number; cost: number } | null>(null)
  const [streaming, setStreaming] = useState(false)
  const [nonce, setNonce] = useState(0) // bumped on every window show to replay the enter animation
  const [revealKey, setRevealKey] = useState(0)
  const [pendingNoteId, setPendingNoteId] = useState<number | null>(null)
  // The active interaction (options card or input prompt), or null. Its promise
  // resolver and the prompt validator live in refs (they don't affect render).
  const [interaction, setInteraction] = useState<Interaction | null>(null)
  const resolverRef = useRef<((v: string | null) => void) | null>(null)
  const validateRef = useRef<((v: string) => string | null) | undefined>(undefined)
  // Recall history for the bar (oldest → newest submitted prompts), in a ref since
  // it drives the input through setInput and doesn't itself need a re-render.
  const historyRef = useRef<PromptHistory>(loadPromptHistory())

  const rootRef = useRef<HTMLDivElement>(null)
  const barRef = useRef<HTMLDivElement>(null)
  const endRef = useRef<HTMLDivElement>(null)
  const taRef = useRef<HTMLTextAreaElement>(null)
  const overlayRef = useRef<HTMLDivElement>(null)
  // Always-current language, so a running command (e.g. /config) sees a language
  // change mid-flow instead of the value captured when it started.
  const langRef = useRef(lang)
  useEffect(() => { langRef.current = lang }, [lang])

  const expanded = view !== 'chat' || msgs.length > 0 || interaction != null
  const maxH = Math.round((window.screen?.availHeight || 900) * TOP_MAX_RATIO)
  const panelMax = Math.max(180, maxH - (barRef.current?.offsetHeight ?? 64))
  // Frost overlay over the native Acrylic backdrop: a translucent white veil whose
  // strength follows the Opacity setting. Both themes lift toward white (never a
  // black overlay), so raising Opacity always reads brighter/more legible — dark
  // mode no longer looks like a near-black slab. Users tune it via /config →
  // Opacidade or the Settings slider. See lib/frost.ts.
  const shellBg = frostColor(theme, opacity)

  const loadCfg = () => ConfigService.Get().then((c: any) => {
    langRef.current = c.language // sync now so an awaiting command sees it immediately
    setLang(c.language); setTheme(c.theme)
    if (typeof c.opacity === 'number') setOpacity(c.opacity)
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
        if (last && last.role === 'assistant') copy[i] = { ...last, content: text, pending: false, instant: true }
        return copy
      })
    })
    const offUsage = onChatUsage((u) => setUsage(u))
    return () => { offDelta(); offDone(); offErr(); offUsage() }
  }, [lang])

  // Live alert wiring: a fired alert posts a system card; clicking a toast body
  // (alert:open) jumps to the alerts view focused on that alert.
  useEffect(() => {
    const offFired = onAlertFired((a) => {
      setView('chat')
      setMsgs((m) => [...m, { role: 'system', content: `${tr(lang, 'alert_created')} **${a.message}**` }])
    })
    const offOpen = onAlertOpen((id) => { setAlertFocusId(id); setView('alerts') })
    return () => { offFired(); offOpen() }
  }, [lang])

  // Each time the window is shown, reset to a clean bar and focus it.
  useEffect(() => {
    const off = onWindowShown(() => {
      resolverRef.current?.(null); resolverRef.current = null; validateRef.current = undefined
      historyRef.current = detachPrompt(historyRef.current)
      setView('chat'); setMsgs([]); setUsage(null); setInput(''); setStreaming(false); setInteraction(null)
      setNonce((n) => n + 1)
      requestAnimationFrame(() => taRef.current?.focus())
    })
    requestAnimationFrame(() => taRef.current?.focus())
    return () => { off() }
  }, [])

  // A última mensagem do assistente (não pendente, não-erro) é a que revela ao vivo.
  const lastIdx = msgs.length - 1
  const lastMsg = msgs[lastIdx]
  const activeTarget =
    lastMsg && lastMsg.role === 'assistant' && !lastMsg.pending && !lastMsg.instant ? lastMsg.content : ''
  const { shown, revealing } = useReveal(activeTarget, { done: !streaming, resetKey: revealKey })

  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [msgs, shown])

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
      if (view !== 'chat') { setView('chat'); setPendingNoteId(null) }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [view])

  // Keyboard while an interaction is active. Capture phase so Esc/Enter here win
  // over the global double-Esc-to-hide handler and the bar's send-on-Enter.
  useEffect(() => {
    if (!interaction) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); cancelInteraction(); return }
      if (interaction.kind !== 'choose') return
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setInteraction({ ...interaction, selected: moveSelection(interaction.choices.length, interaction.selected, 1) })
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setInteraction({ ...interaction, selected: moveSelection(interaction.choices.length, interaction.selected, -1) })
      } else if (e.key === 'Enter') {
        e.preventDefault(); e.stopPropagation()
        pickChoice(interaction.selected)
      }
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [interaction])

  // The capability surface commands act through (see commands/types.ts). Built
  // here so commands stay decoupled from React internals.
  const commandContext: CommandContext = {
    // Getter, not a snapshot: a long-running command reads the live language.
    get lang() { return langRef.current },
    setView,
    newConversation: () => {
      ChatService.NewConversation(); setMsgs([]); setUsage(null); setView('chat')
    },
    emitSystemMessage: (markdown) => {
      setView('chat'); setMsgs((m) => [...m, { role: 'system', content: markdown }])
    },
    getCommands: () => commands,
    choose: (req) => new Promise<string | null>((resolve) => {
      resolverRef.current = resolve
      setView('chat')
      setInteraction({ kind: 'choose', title: req.title, choices: req.choices, selected: 0, silent: req.silent })
    }),
    prompt: (req) => new Promise<string | null>((resolve) => {
      resolverRef.current = resolve
      validateRef.current = req.validate
      setView('chat'); setInput('')
      setInteraction({ kind: 'prompt', title: req.title, value: '', placeholder: req.placeholder })
      requestAnimationFrame(() => taRef.current?.focus())
    }),
    slider: (req) => new Promise<string | null>((resolve) => {
      resolverRef.current = resolve
      setView('chat')
      setInteraction({ kind: 'slider', title: req.title, value: req.value, min: req.min, max: req.max, step: req.step })
    }),
    config: {
      get: () => ConfigService.Get() as Promise<Record<string, any>>,
      apply: async (key, value) => {
        const cur: any = await ConfigService.Get()
        await ConfigService.Save({ ...cur, [key]: value })
        await loadCfg()
      },
      models: () => ConfigService.Models().then((m) => m ?? []),
    },
    notes: {
      capture: (text) => NotesService.Capture(text) as any,
      find: (query) => NotesService.Find(query).then((r) => (r ?? []) as any),
      ask: (query) => NotesService.Ask(query) as any,
    },
    openSearch: (state) => { setSearchState(state); setView('search') },
    alerts: {
      create: (text) => AlertsService.Create(text) as any,
    },
  }

  // Finalize the active `choose`: record the pick as an inert message and resolve.
  const pickChoice = (index: number) => {
    if (interaction?.kind !== 'choose') return
    const choice = interaction.choices[index]
    if (!choice) return
    const resolve = resolverRef.current
    resolverRef.current = null
    if (!interaction.silent) {
      setMsgs((m) => [...m, { role: 'choice', title: interaction.title, chosenLabel: choice.label }])
    }
    setInteraction(null)
    resolve?.(choice.value)
  }

  // Finalize the active `prompt`: validate the typed value; on success record it
  // and resolve, otherwise show the error and keep the bar open.
  const submitPrompt = () => {
    if (interaction?.kind !== 'prompt') return
    const value = input
    const err = validateRef.current?.(value) ?? null
    if (err) { setInteraction({ ...interaction, error: err }); return }
    const resolve = resolverRef.current
    resolverRef.current = null; validateRef.current = undefined
    setMsgs((m) => [...m, { role: 'choice', title: interaction.title, chosenLabel: value || '—' }])
    setInteraction(null); setInput('')
    resolve?.(value)
  }

  // Finalize the active `slider`: record the chosen number as an inert message and
  // resolve with its string form (config.apply coerces it back to a number).
  const submitSlider = () => {
    if (interaction?.kind !== 'slider') return
    const value = interaction.value
    const resolve = resolverRef.current
    resolverRef.current = null
    setMsgs((m) => [...m, { role: 'choice', title: interaction.title, chosenLabel: String(value) }])
    setInteraction(null)
    resolve?.(String(value))
    requestAnimationFrame(() => taRef.current?.focus())
  }

  // Abandon any active interaction (Esc), resolving its promise with null. Return
  // focus to the bar (it was disabled during a choose) so the user can keep typing.
  const cancelInteraction = () => {
    const resolve = resolverRef.current
    resolverRef.current = null; validateRef.current = undefined
    setInteraction(null); setInput('')
    resolve?.(null)
    requestAnimationFrame(() => taRef.current?.focus())
  }

  const send = () => {
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
    ChatService.Send(text)
    setInput('')
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
        background: shellBg,
        ['--shell-bg' as any]: shellBg,
        boxShadow: 'var(--shell-shadow)',
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
        {/* The textarea text is transparent; the overlay behind it paints the
            command token (accent) and the ghost name completion (muted). They
            share identical typography so the layers line up exactly. */}
        <div className="relative flex-1 self-center [--wails-draggable:no-drag]">
          <div
            ref={overlayRef}
            aria-hidden
            className="pointer-events-none absolute inset-0 max-h-[132px] overflow-hidden whitespace-pre-wrap break-words px-0 py-1 text-[15px] leading-relaxed"
          >
            {input.length > 0 && (bar.isCommand ? (
              <><span className="text-accent">{input}</span><span className="text-muted/50">{bar.completion}</span></>
            ) : (
              <span className="text-fg">{input}</span>
            ))}
          </div>
          <textarea
            ref={taRef}
            className="relative block max-h-[132px] w-full resize-none bg-transparent px-0 py-1 text-[15px] leading-relaxed text-transparent caret-[var(--color-fg)] outline-none placeholder:text-muted/70 disabled:cursor-not-allowed"
            rows={1}
            value={input}
            spellCheck={false}
            autoCorrect="off"
            autoCapitalize="off"
            disabled={interaction?.kind === 'choose' || interaction?.kind === 'slider'}
            placeholder={interaction?.kind === 'prompt'
              ? (interaction.placeholder ?? tr(lang, 'enter_value'))
              : tr(lang, 'placeholder')}
            onChange={(e) => { historyRef.current = detachPrompt(historyRef.current); setInput(e.target.value) }}
            onScroll={(e) => { if (overlayRef.current) overlayRef.current.scrollTop = e.currentTarget.scrollTop }}
            onKeyDown={onBarKeyDown}
          />
        </div>
        <button
          onClick={() => (interaction?.kind === 'prompt' ? submitPrompt() : interaction?.kind === 'slider' ? submitSlider() : send())}
          disabled={interaction?.kind === 'choose' || (interaction == null && !input.trim())}
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
          className={`min-h-0 border-t border-[color:var(--shell-border)] selectable ${view === 'history' || view === 'notes' || view === 'search' || view === 'graph' || view === 'alerts' ? 'overflow-hidden' : 'overflow-y-auto'}`}
          style={view === 'history' || view === 'notes' || view === 'search' || view === 'graph' || view === 'alerts' ? { height: panelMax } : { maxHeight: panelMax }}
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
              {msgs.map((m, i) => m.role === 'choice' ? (
                <ChoiceSummary key={i} title={m.title} chosenLabel={m.chosenLabel} />
              ) : (
                <MessageCard key={i} role={m.role}
                  content={
                    m.pending
                      ? tr(lang, 'thinking')
                      : i === lastIdx && m.role === 'assistant' && !m.instant
                        ? m.content.slice(0, shown)
                        : m.content
                  }
                  pending={m.pending}
                  revealing={i === lastIdx && m.role === 'assistant' && !m.pending && !m.instant && revealing}
                  label={m.role === 'user' ? tr(lang, 'you') : m.role === 'system' ? tr(lang, 'system') : tr(lang, 'ai')} />
              ))}
              {interaction?.kind === 'choose' && (
                <ChoiceCard
                  title={interaction.title}
                  choices={interaction.choices}
                  selected={interaction.selected}
                  onHover={(i) => setInteraction({ ...interaction, selected: i })}
                  onPick={(i) => pickChoice(i)}
                />
              )}
              {interaction?.kind === 'prompt' && (
                <div className="flex flex-col gap-1 px-1">
                  <span className="flex items-center gap-1 text-[11px] font-semibold tracking-wide text-accent">
                    {interaction.title}
                  </span>
                  {interaction.error && (
                    <span className="font-mono text-xs text-red-500">{interaction.error}</span>
                  )}
                </div>
              )}
              {interaction?.kind === 'slider' && (
                <motion.div
                  initial={{ opacity: 0, y: 8, filter: 'blur(4px)' }}
                  animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
                  transition={{ type: 'spring', duration: 0.35, bounce: 0 }}
                  className="flex flex-col gap-1.5 px-1"
                >
                  <span className="text-[11px] font-semibold tracking-wide text-accent">{interaction.title}</span>
                  <Slider
                    autoFocus
                    ariaLabel={interaction.title}
                    value={interaction.value}
                    min={interaction.min}
                    max={interaction.max}
                    step={interaction.step}
                    onChange={(v) => setInteraction({ ...interaction, value: v })}
                    onCommit={submitSlider}
                  />
                  <span className="text-[11px] text-muted">{tr(lang, 'slider_hint')}</span>
                </motion.div>
              )}
              <div ref={endRef} />
            </div>
          )}
          {view === 'settings' && <SettingsView lang={lang} onClose={() => { loadCfg(); setView('chat') }} />}
          {view === 'history' && <HistoryView lang={lang} onClose={() => { setView('chat'); setPendingNoteId(null) }} />}
          {view === 'notes' && <NotesView lang={lang} onClose={() => { setView('chat'); setPendingNoteId(null) }} initialActiveId={pendingNoteId} />}
          {view === 'search' && searchState && <SearchView lang={lang} state={searchState} onClose={() => setView('chat')} />}
          {view === 'graph' && <GraphView lang={lang} onClose={() => setView('chat')} onSelectNote={(id) => { setPendingNoteId(id); setView('notes') }} />}
          {view === 'alerts' && <AlertsView lang={lang} focusId={alertFocusId} onClose={() => { setAlertFocusId(null); setView('chat') }} />}
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
