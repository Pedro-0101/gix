import { commands } from './registry'
import type { Command } from './types'

// The result of inspecting the bar text for command highlighting + name completion.
export type BarHighlight = {
  isCommand: boolean   // typed text resolves (fully or by prefix) to a command
  completion: string   // ghost suffix to show after the typed text ('' if none)
  accepted: string     // value to set when the completion is accepted ('/help')
}

// Canonical names first so completion prefers them over aliases.
function completableNames(list: Command[]): string[] {
  return [...list.map((c) => c.name), ...list.flatMap((c) => c.aliases ?? [])]
}

// Pure: decides how the bar should be highlighted and what a Tab/→ would complete
// to. Command-like means a single leading-slash token with no whitespace whose
// key prefixes a known command name/alias.
export function analyzeBar(input: string, list: Command[] = commands): BarHighlight {
  const none: BarHighlight = { isCommand: false, completion: '', accepted: input }
  if (!input.startsWith('/') || /\s/.test(input)) return none
  const key = input.slice(1).toLowerCase()
  if (!key) return none
  const match = completableNames(list).find((n) => n.startsWith(key))
  if (!match) return none
  return { isCommand: true, completion: match.slice(key.length), accepted: '/' + match }
}
