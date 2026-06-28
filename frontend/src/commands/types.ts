import type { Choice } from './interaction'

// The view the shell can show. Mirrors the `View` union in App.tsx.
export type View = 'chat' | 'settings' | 'history' | 'notes' | 'search' | 'graph' | 'alerts'

// Outcomes of the note actions, mirrored structurally from the Go results
// (bindings). Kept as local interfaces so commands stay decoupled from the
// generated binding classes.
export interface CaptureResult {
  status: string
  noteId: number
  noteTitle: string
  // The AI-formatted body. Carried back on an attach proposal so the frontend can
  // either append it or create a new note without a second AI call.
  content: string
  tags: string[]
  message: string
  alert?: { message: string; fireAt: string; recurrence: string }
  // Set when status is 'attach_proposed': the existing note the AI judged this
  // capture belongs to. The frontend confirms before appending.
  attach?: { targetId: number; targetTitle: string }
}

// Outcome of creating an alert (Go app.CreateAlertResult).
// Result of creating a note from a proposal (Go app.CreateFromProposal).
export interface CreateFromProposalResult {
  status: string
  noteId: number
  noteTitle: string
  tags: string[]
  message: string
}

export interface CreateAlertResult {
  status: string
  alertId: number
  message: string
  fireAtLocal: string
  recurrence: string
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

// Result of summarizing one note (Go app.SummarizeResult). The summary is
// returned only; the caller applies it via notes.update so the change is undoable.
export interface SummarizeResult {
  status: string
  summary: string
  message: string
}

// Result of tidying one note (Go app.TidyResult). Like SummarizeResult the new
// body is returned only; the caller applies it via notes.update (undoable). Unlike
// a summary it preserves every fact — it reorganizes, it doesn't condense.
export interface TidyResult {
  status: string
  content: string
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
    // Summarize one note with the AI (no change applied); the caller persists the
    // result via update.
    summarize(id: number): Promise<SummarizeResult>
    // Reorganize one note with the AI without condensing (no change applied); the
    // caller persists the result via update so it's undoable.
    tidy(id: number): Promise<TidyResult>
    // Replace a note's title/content/tags exactly as given (no AI). Used to apply
    // a summary and to undo it.
    update(id: number, title: string, content: string, tags: string[]): Promise<void>
    // Store a note already formatted by the AI (tool call proposal), without
    // another AI call. Used when the user confirms a note:proposed.
    createFromProposal(title: string, content: string, tags: string[]): Promise<CreateFromProposalResult>
    // Append already-formatted content to an existing note (no AI). Used when the
    // capture router proposes attaching and the user confirms.
    appendTo(targetId: number, content: string, tags: string[]): Promise<CaptureResult>
  }
  // Opens the search view with the given state (loading, then results).
  openSearch(state: SearchState): void
  // Alerts access: create a reminder from natural-language text (the backend
  // calls the AI to parse the date/recurrence).
  alerts: {
    create(text: string): Promise<CreateAlertResult>
    createProposed(p: { message: string; fireAt: string; recurrence: string; noteId?: number }): Promise<CreateAlertResult>
  }
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
