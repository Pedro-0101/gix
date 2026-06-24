// Shell-style recall of previously submitted prompts in the input bar.
// ArrowUp walks toward older entries, ArrowDown back toward newer ones; stepping
// past the newest restores the draft the user was typing when they started
// browsing. Kept free of React so the navigation rule can be unit-tested and the
// shell only wires keys and persistence around it.

export type PromptHistory = {
  entries: string[] // submitted prompts, oldest → newest
  index: number | null // browsing position, or null while editing the live draft
  draft: string // the live input captured when browsing began
}

// Cap so the list can't grow without bound across a long-running session.
export const HISTORY_CAP = 100

export function emptyHistory(entries: string[] = []): PromptHistory {
  return { entries: entries.slice(-HISTORY_CAP), index: null, draft: '' }
}

// Append a submitted prompt. Blank lines and immediate repeats are ignored (a
// duplicate just refreshes the existing tail). Browsing state is reset either
// way, so the next ArrowUp starts from the newest entry.
export function record(h: PromptHistory, text: string): PromptHistory {
  const trimmed = text.trim()
  if (!trimmed) return { ...h, index: null, draft: '' }
  const last = h.entries[h.entries.length - 1]
  const entries = trimmed === last ? h.entries : [...h.entries, trimmed].slice(-HISTORY_CAP)
  return { entries, index: null, draft: '' }
}

// Step toward older entries (ArrowUp). `current` is what's in the bar now; it is
// stashed as the draft on the first step so ArrowDown can return to it.
// `handled` is false when there is nothing to recall, so the caller can let the
// key fall through to normal caret movement.
export function prev(h: PromptHistory, current: string): { history: PromptHistory; value: string; handled: boolean } {
  if (h.entries.length === 0) return { history: h, value: current, handled: false }
  if (h.index === null) {
    const index = h.entries.length - 1
    return { history: { ...h, index, draft: current }, value: h.entries[index], handled: true }
  }
  const index = Math.max(0, h.index - 1)
  return { history: { ...h, index }, value: h.entries[index], handled: true }
}

// Step toward newer entries (ArrowDown). Past the newest entry, browsing ends and
// the stashed draft comes back. `handled` is false when not browsing.
export function next(h: PromptHistory): { history: PromptHistory; value: string; handled: boolean } {
  if (h.index === null) return { history: h, value: '', handled: false }
  if (h.index >= h.entries.length - 1) {
    return { history: { ...h, index: null }, value: h.draft, handled: true }
  }
  const index = h.index + 1
  return { history: { ...h, index }, value: h.entries[index], handled: true }
}

// Leave browsing without changing the input — used when the user edits a recalled
// line, so the next ArrowUp restarts from the newest entry.
export function detach(h: PromptHistory): PromptHistory {
  return h.index === null ? h : { ...h, index: null, draft: '' }
}
