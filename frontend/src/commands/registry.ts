import type { Command } from './types'
import { newCommand } from './builtins/new'
import { configCommand } from './builtins/config'
import { historyCommand } from './builtins/history'
import { helpCommand } from './builtins/help'

// The single source of truth. New commands — including future user-defined ones —
// are added here (or pushed at runtime); the dispatcher and /help read this list,
// so nothing else needs to change.
export const commands: Command[] = [
  newCommand,
  configCommand,
  historyCommand,
  helpCommand,
]

// Resolves raw bar input to a command. Only a single leading-slash token with no
// arguments counts as a command (e.g. "/help"); anything else is a chat message.
// Matching is case-insensitive over name + aliases.
export function resolveCommand(input: string, list: Command[] = commands): Command | null {
  const text = input.trim()
  if (!text.startsWith('/') || /\s/.test(text)) return null
  const key = text.slice(1).toLowerCase()
  if (!key) return null
  return list.find((c) => c.name === key || (c.aliases ?? []).includes(key)) ?? null
}

export type { Command, CommandContext, View } from './types'
