import { type Dispatch, type SetStateAction } from "react"
import { AlertsService, ChatService, ConfigService, NotesService } from "../../bindings/gix/internal/app"
import { commands } from "../commands/registry"
import type { CommandContext, SearchState, View } from "../commands/types"
import type { Msg, Usage } from "../types"

type Deps = {
  // Live language holder (read through, not snapshotted) so a long-running
  // command sees a mid-flow language change.
  langRef: { current: string }
  setView: (v: View) => void
  setMsgs: Dispatch<SetStateAction<Msg[]>>
  setUsage: (u: Usage | null) => void
  setSearchState: (s: SearchState) => void
  choose: CommandContext["choose"]
  prompt: CommandContext["prompt"]
  slider: CommandContext["slider"]
  loadCfg: () => void | Promise<void>
}

// useCommandContext assembles the capability surface commands act through (see
// commands/types.ts). It's the single place the shell exposes its abilities to
// commands, keeping them decoupled from React internals (Dependency Inversion).
export function useCommandContext(d: Deps): CommandContext {
  return {
    get lang() {
      return d.langRef.current
    },
    setView: d.setView,
    newConversation: () => {
      ChatService.NewConversation()
      d.setMsgs([])
      d.setUsage(null)
      d.setView("chat")
    },
    emitSystemMessage: (markdown) => {
      d.setView("chat")
      d.setMsgs((m) => [...m, { role: "system", content: markdown }])
    },
    getCommands: () => commands,
    choose: d.choose,
    prompt: d.prompt,
    slider: d.slider,
    config: {
      get: () => ConfigService.Get() as Promise<Record<string, any>>,
      apply: async (key, value) => {
        const cur: any = await ConfigService.Get()
        await ConfigService.Save({ ...cur, [key]: value })
        await d.loadCfg()
      },
      models: () => ConfigService.Models().then((m) => m ?? []),
    },
    notes: {
      capture: (text) => NotesService.Capture(text) as any,
      find: (query) => NotesService.Find(query).then((r) => (r ?? []) as any),
      ask: (query) => NotesService.Ask(query) as any,
      summarize: (id) => NotesService.Summarize(id) as any,
      tidy: (id) => NotesService.Tidy(id) as any,
      update: (id, title, content, tags) => NotesService.Update(id, title, content, tags).then(() => {}),
      createFromProposal: (title, content, tags) => NotesService.CreateFromProposal(title, content, tags) as any,
      appendTo: (targetId, content, tags) => NotesService.AppendTo(targetId, content, tags) as any,
    },
    openSearch: (state) => {
      d.setSearchState(state)
      d.setView("search")
    },
    alerts: {
      create: (text) => AlertsService.Create(text) as any,
      createProposed: (p) =>
        AlertsService.CreateProposed(p.message, p.fireAt, p.recurrence, p.noteId ?? null) as any,
    },
  }
}
