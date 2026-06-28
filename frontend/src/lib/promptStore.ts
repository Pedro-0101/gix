import { emptyHistory, type PromptHistory } from "../commands/promptHistory"

// Submitted prompts persist across window shows and app restarts via localStorage,
// so ArrowUp recalls them like a shell history.
const PROMPT_HISTORY_KEY = "gix.promptHistory"

export const loadPromptHistory = (): PromptHistory => {
  try {
    const raw = localStorage.getItem(PROMPT_HISTORY_KEY)
    return emptyHistory(raw ? JSON.parse(raw) : [])
  } catch {
    return emptyHistory()
  }
}

export const savePromptHistory = (h: PromptHistory) => {
  try {
    localStorage.setItem(PROMPT_HISTORY_KEY, JSON.stringify(h.entries))
  } catch {
    /* storage unavailable */
  }
}
