import { motion } from "motion/react"
import { type Dispatch, type RefObject, type SetStateAction } from "react"
import type { Interaction } from "../commands/interaction"
import type { SearchState, View } from "../commands/types"
import type { Msg, Usage } from "../types"
import { MessageCard } from "./MessageCard"
import { ChoiceCard, ChoiceSummary } from "./ChoiceCard"
import { Slider } from "./Slider"
import { SettingsView } from "../views/SettingsView"
import { HistoryView } from "../views/HistoryView"
import { NotesView } from "../views/NotesView"
import { SearchView } from "../views/SearchView"
import { GraphView } from "../views/GraphView"
import { AlertsView } from "../views/AlertsView"
import { tr } from "../i18n"

// Views that own their scroll area and take the full panel height.
const fullViews = ["history", "notes", "search", "graph", "alerts"] as const
const isFullView = (v: View) => (fullViews as readonly string[]).includes(v)

type Props = {
  lang: string
  view: View
  panelMax: number
  // chat view
  usage: Usage | null
  msgs: Msg[]
  lastIdx: number
  shown: number
  revealing: boolean
  interaction: Interaction | null
  setInteraction: Dispatch<SetStateAction<Interaction | null>>
  pickChoice: (i: number) => void
  submitSlider: () => void
  endRef: RefObject<HTMLDivElement>
  // view routing
  searchState: SearchState | null
  pendingNoteId: number | null
  alertFocusId: number | null
  onCloseToChat: () => void
  onCloseClearNote: () => void
  onReloadCfgClose: () => void
  onSelectNote: (id: number) => void
  onCloseAlerts: () => void
}

// ShellPanel is the panel that expands downward below the bar: the chat
// transcript (messages, usage, and the active interaction card) plus the routing
// to the full-screen views. Presentational — all state and handlers come in.
export function ShellPanel(p: Props) {
  const full = isFullView(p.view)
  return (
    <motion.div
      initial={{ opacity: 0, y: -4 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ type: "spring", duration: 0.3, bounce: 0 }}
      className={`min-h-0 border-t border-[color:var(--shell-border)] selectable ${full ? "overflow-hidden" : "overflow-y-auto"}`}
      style={full ? { height: p.panelMax } : { maxHeight: p.panelMax }}
    >
      {p.view === "chat" && (
        <div className="space-y-3 px-3 py-3">
          {p.usage && (
            <div className="flex items-center gap-2 font-mono text-xs text-muted tabular-nums">
              <span>{p.usage.tokens} tokens</span>
              <span className="opacity-40">·</span>
              <span>${p.usage.cost.toFixed(6)}</span>
            </div>
          )}
          {p.msgs.map((m, i) => m.role === "choice" ? (
            <ChoiceSummary key={i} title={m.title} chosenLabel={m.chosenLabel} />
          ) : (
            <MessageCard key={i} role={m.role}
              content={
                m.pending
                  ? tr(p.lang, "thinking")
                  : i === p.lastIdx && m.role === "assistant" && !m.instant
                    ? m.content.slice(0, p.shown)
                    : m.content
              }
              pending={m.pending}
              revealing={i === p.lastIdx && m.role === "assistant" && !m.pending && !m.instant && p.revealing}
              label={m.role === "user" ? tr(p.lang, "you") : m.role === "system" ? tr(p.lang, "system") : tr(p.lang, "ai")} />
          ))}
          {p.interaction?.kind === "choose" && (
            <ChoiceCard
              title={p.interaction.title}
              choices={p.interaction.choices}
              selected={p.interaction.selected}
              onHover={(i) => p.setInteraction((prev) => prev && prev.kind === "choose" ? { ...prev, selected: i } : prev)}
              onPick={(i) => p.pickChoice(i)}
            />
          )}
          {p.interaction?.kind === "prompt" && (
            <div className="flex flex-col gap-1 px-1">
              <span className="flex items-center gap-1 text-[11px] font-semibold tracking-wide text-accent">
                {p.interaction.title}
              </span>
              {p.interaction.error && (
                <span className="font-mono text-xs text-red-500">{p.interaction.error}</span>
              )}
            </div>
          )}
          {p.interaction?.kind === "slider" && (
            <motion.div
              initial={{ opacity: 0, y: 8, filter: "blur(4px)" }}
              animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
              transition={{ type: "spring", duration: 0.35, bounce: 0 }}
              className="flex flex-col gap-1.5 px-1"
            >
              <span className="text-[11px] font-semibold tracking-wide text-accent">{p.interaction.title}</span>
              <Slider
                autoFocus
                ariaLabel={p.interaction.title}
                value={p.interaction.value}
                min={p.interaction.min}
                max={p.interaction.max}
                step={p.interaction.step}
                onChange={(v) => p.setInteraction((prev) => prev && prev.kind === "slider" ? { ...prev, value: v } : prev)}
                onCommit={p.submitSlider}
              />
              <span className="text-[11px] text-muted">{tr(p.lang, "slider_hint")}</span>
            </motion.div>
          )}
          <div ref={p.endRef} />
        </div>
      )}
      {p.view === "settings" && <SettingsView lang={p.lang} onClose={p.onReloadCfgClose} />}
      {p.view === "history" && <HistoryView lang={p.lang} onClose={p.onCloseClearNote} />}
      {p.view === "notes" && <NotesView lang={p.lang} onClose={p.onCloseClearNote} initialActiveId={p.pendingNoteId} />}
      {p.view === "search" && p.searchState && <SearchView lang={p.lang} state={p.searchState} onClose={p.onCloseToChat} />}
      {p.view === "graph" && <GraphView lang={p.lang} onClose={p.onCloseToChat} onSelectNote={p.onSelectNote} />}
      {p.view === "alerts" && <AlertsView lang={p.lang} focusId={p.alertFocusId} onClose={p.onCloseAlerts} />}
    </motion.div>
  )
}
