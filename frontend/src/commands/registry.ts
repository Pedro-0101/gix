import type { Command } from './types'
import { newCommand } from './builtins/new'
import { configCommand } from './builtins/config'
import { historyCommand } from './builtins/history'
import { helpCommand } from './builtins/help'
import { noteCommand } from './builtins/note'
import { notesCommand } from './builtins/notes'

// The single source of truth. New commands — including future user-defined ones —
// are added here (or pushed at runtime); the dispatcher and /help read this list,
// so nothing else needs to change.
export const commands: Command[] = [
  newCommand,
  noteCommand,
  notesCommand,
  configCommand,
  historyCommand,
  helpCommand,
]

// Resolves raw bar input to a command plus its argument. A bare slash token
// (e.g. "/help") matches any command. Text after the name only counts when the
// command opts in with `acceptsArgs` (e.g. "/note buy milk"); otherwise the
// input is a chat message and we return null. Matching is case-insensitive over
// name + aliases.
export function resolveCommand(
  input: string,
  list: Command[] = commands,
): { cmd: Command; arg: string } | null {
  const text = input.trim()
  if (!text.startsWith('/')) return null
  const body = text.slice(1)
  if (!body) return null

  const sep = body.search(/\s/)
  const key = (sep === -1 ? body : body.slice(0, sep)).toLowerCase()
  const arg = sep === -1 ? '' : body.slice(sep + 1).trim()

  const cmd = list.find((c) => c.name === key || (c.aliases ?? []).includes(key))
  if (!cmd) return null
  if (arg && !cmd.acceptsArgs) return null // args for a no-arg command → it's a message
  return { cmd, arg }
}

export type { Command, CommandContext, View } from './types'
