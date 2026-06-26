import type { Choice } from './interaction'

// The view the shell can show. Mirrors the `View` union in App.tsx.
export type View = 'chat' | 'settings' | 'history' | 'notes' | 'search' | 'graph'

// Outcomes of the note actions, mirrored structurally from the Go results
// (bindings). Kept as local interfaces so commands stay decoupled from the
// generated binding classes.
export interface CaptureResult {
  status: string
  noteId: number
  noteTitle: string
  tags: string[]
  message: string
}

// One ranked search hit (Go app.SearchResult).
export interface SearchHit {
  noteId: number
  title: string
  snippet: string
  content: string
  tags: string[]
  score: number
}

// Result of an /ask: an AI summary plus the source notes (Go app.AskResult).
export interface AskResult {
  status: string
  summary: string
  sources: SearchHit[]
  message: string
}

// The state driving the search view, set via CommandContext.openSearch. While
// `loading` is true the view shows a spinner; for mode 'ask' the summary panel
// renders once the answer arrives.
export type SearchMode = 'find' | 'ask'
export interface SearchState {
  query: string
  mode: SearchMode
  loading: boolean
  hits: SearchHit[]
  summary?: string
  status?: string
}

// CommandContext is the abstraction a command depends on (Dependency Inversion):
// the only capabilities the shell exposes to commands. Commands never reach into
// App's React state directly — they act through this surface, which keeps them
// decoupled and trivially relocatable (e.g. a future user-defined command).
export interface CommandContext {
  lang: string
  setView(v: View): void
  // Starts a fresh conversation and returns to the chat view.
  newConversation(): void
  // Appends a system message (not an AI reply) to the chat and shows it.
  emitSystemMessage(markdown: string): void
  // The live registry, so commands like /help can describe their peers.
  getCommands(): Command[]

  // Interaction primitive: emits an options card and resolves with the chosen
  // value, or null if the user cancels (Esc). `silent` cards leave no inert
  // record in the conversation (use for navigation menus).
  choose(req: { title: string; choices: Choice[]; silent?: boolean }): Promise<string | null>
  // Puts the bar into input mode; resolves with the typed value once it passes
  // `validate` (which returns an i18n error key or null), or null on cancel.
  prompt(req: { title: string; placeholder?: string; validate?: (v: string) => string | null }): Promise<string | null>
  // Shows a bounded slider (←/→ adjust, Enter commits); resolves with the chosen
  // value as a string, or null on cancel. Used for numeric settings.
  slider(req: { title: string; value: number; min: number; max: number; step: number }): Promise<string | null>
  // Configuration access: read the current values, or persist one field and
  // reflect it live (theme, language, …).
  config: {
    get(): Promise<Record<string, any>>
    apply(key: string, value: string | number): Promise<void>
    // The valid model ids, for the `model` field's choices.
    models(): Promise<string[]>
  }
  // Notes access: capture a quick note, search (no AI), or ask (search + AI
  // summary). The search commands push results to the view via openSearch.
  notes: {
    capture(text: string): Promise<CaptureResult>
    find(query: string): Promise<SearchHit[]>
    ask(query: string): Promise<AskResult>
  }
  // Opens the search view with the given state (loading, then results).
  openSearch(state: SearchState): void
}

// A single command. Adding one is the whole story: drop an object implementing
// this into the registry — the dispatcher and /help pick it up automatically
// (Open/Closed). Each command owns its own behavior (Single Responsibility).
export interface Command {
  name: string             // canonical, slash-less: 'help'
  aliases?: string[]       // alternative slash-less names: ['ajuda']
  descriptionKey: string   // i18n key shown in the /help listing
  hidden?: boolean         // when true, omitted from /help
  acceptsArgs?: boolean    // when true, text after the name is passed as `arg`
  run(ctx: CommandContext, arg?: string): void | Promise<void>
}
