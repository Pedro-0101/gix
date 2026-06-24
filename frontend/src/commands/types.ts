import type { Choice } from './interaction'

// The view the shell can show. Mirrors the `View` union in App.tsx.
export type View = 'chat' | 'settings' | 'history'

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
  // value, or null if the user cancels (Esc).
  choose(req: { title: string; choices: Choice[] }): Promise<string | null>
  // Puts the bar into input mode; resolves with the typed value once it passes
  // `validate` (which returns an i18n error key or null), or null on cancel.
  prompt(req: { title: string; placeholder?: string; validate?: (v: string) => string | null }): Promise<string | null>
  // Configuration access: read the current values, or persist one field and
  // reflect it live (theme, language, …).
  config: {
    get(): Promise<Record<string, any>>
    apply(key: string, value: string | number): Promise<void>
    // The valid model ids, for the `model` field's choices.
    models(): Promise<string[]>
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
  run(ctx: CommandContext): void | Promise<void>
}
