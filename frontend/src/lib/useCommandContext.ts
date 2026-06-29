import { type Dispatch, type SetStateAction } from "react"
import { ConfigService } from "../../bindings/gix/internal/app"
import { AlertsService, ChatService, Models, NotesService } from "../api/services"
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
      ChatService.newConversation()
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
        // Coerce to the persisted field's type: numeric fields exposed as an
        // enum (e.g. open_press_count) resolve to a string '2'/'3', but the Go
        // config stores them as int — saving the raw string makes Save reject
        // and the /config loop bail to the chat. Match the canonical type.
        const coerced = typeof cur[key] === 'number' && typeof value === 'string' ? Number(value) : value
        await ConfigService.Save({ ...cur, [key]: coerced })
        await d.loadCfg()
      },
      models: () => Models.list().then((m) => m.map((x) => x.id)),
    },
    notes: {
      capture: (text) => NotesService.capture(text) as any,
      find: (query) => NotesService.find(query) as any,
      ask: (query) => NotesService.ask(query) as any,
      summarize: (id) => NotesService.summarize(id) as any,
      tidy: (id) => NotesService.tidy(id) as any,
      update: (id, title, content, tags) => NotesService.update(id, title, content, tags).then(() => {}),
      createFromProposal: (title, content, tags) => NotesService.createFromProposal(title, content, tags) as any,
      appendTo: (targetId, content, tags) => NotesService.appendTo(targetId, content, tags) as any,
      resolveOverflow: (targetId, content, tags, mode) =>
        NotesService.resolveOverflow(targetId, content, tags, mode) as any,
      setCharLimit: (id, limit) => NotesService.setCharLimit(id, limit),
    },
    openSearch: (state) => {
      d.setSearchState(state)
      d.setView("search")
    },
    alerts: {
      create: (text) => AlertsService.create(text) as any,
      createProposed: (p) =>
        AlertsService.createProposed(p.message, p.fireAt, p.recurrence, p.noteId ?? null) as any,
    },
  }
}
