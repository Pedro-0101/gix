// Tag-input helpers, mirroring the backend's normalization (lowercase, strip a
// leading '#', trim). Kept pure so the editor's chip input is testable without
// the DOM. The Go side re-normalizes on save (normalizeTagsUncapped), so this is
// purely for a clean editing experience.

export function normalizeTag(raw: string): string {
  return raw.trim().replace(/^#+/, '').trim().toLowerCase()
}

// addTags appends one or more tags from a raw entry (which may be comma-
// separated, e.g. from typing or pasting "a, b, c") to `existing`, normalizing
// each and dropping empties and duplicates. Returns a new array.
export function addTags(existing: string[], raw: string): string[] {
  const out = [...existing]
  for (const part of raw.split(',')) {
    const tag = normalizeTag(part)
    if (tag && !out.includes(tag)) out.push(tag)
  }
  return out
}
