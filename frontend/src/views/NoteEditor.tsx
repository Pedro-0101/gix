import { useEffect, useRef, useState } from 'react'
import type { Note } from '../api/types'
import { Button } from '../components/Button'
import { addTags } from '../lib/tags'
import { tr } from '../i18n'

const field =
  'w-full rounded-field bg-surface px-2.5 py-1.5 text-sm text-fg shadow-[var(--shadow-border)] outline-none ' +
  'transition-[box-shadow] duration-150 ease-out ' +
  'focus-visible:shadow-[0_0_0_1px_var(--ring-focus),0_0_0_3px_color-mix(in_srgb,var(--ring-focus)_25%,transparent)]'

const sameTags = (a: string[], b: string[]) =>
  a.length === b.length && a.every((t, i) => t === b[i])

// Full-screen editor for one note: title, tags (chips) and body. Saving writes
// the text exactly as typed (no AI) via NotesService.Update. Cancelling with
// unsaved changes asks for confirmation. Rendered by NotesView in place of the
// master-detail browser while editing.
export function NoteEditor({
  lang,
  note,
  defaultCharLimit,
  onSave,
  onCancel,
}: {
  lang: string
  note: Note
  defaultCharLimit: number
  onSave: (title: string, content: string, tags: string[], charLimit: number) => void
  onCancel: () => void
}) {
  const [title, setTitle] = useState(note.title)
  const [content, setContent] = useState(note.content)
  const [tags, setTags] = useState<string[]>(note.tags ?? [])
  const [tagInput, setTagInput] = useState('')
  const [charLimit, setCharLimit] = useState(note.charLimit ?? 0)
  const [confirmingDiscard, setConfirmingDiscard] = useState(false)
  const titleRef = useRef<HTMLInputElement>(null)

  // Count code points (close to the Go rune count the backend enforces) against
  // the effective limit: the per-note override when set, else the global default.
  const charCount = [...content].length
  const effectiveLimit = charLimit > 0 ? charLimit : defaultCharLimit
  const over = effectiveLimit > 0 && charCount > effectiveLimit

  const dirty =
    title !== note.title ||
    content !== note.content ||
    charLimit !== (note.charLimit ?? 0) ||
    !sameTags(tags, note.tags ?? [])

  useEffect(() => {
    titleRef.current?.focus()
  }, [])

  // Try to leave the editor. If there are unsaved changes, ask first instead of
  // discarding them.
  const tryCancel = () => {
    if (dirty) setConfirmingDiscard(true)
    else onCancel()
  }

  // Esc cancels the edit (with the dirty guard) rather than letting App's global
  // handler close the whole notes view. Capture phase to win that race, matching
  // how NotesView intercepts the arrow keys.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      if (confirmingDiscard) setConfirmingDiscard(false)
      else tryCancel()
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [confirmingDiscard, dirty])

  const commitTagInput = () => {
    if (!tagInput.trim()) return
    setTags((t) => addTags(t, tagInput))
    setTagInput('')
  }

  const onTagKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      commitTagInput()
    } else if (e.key === 'Backspace' && tagInput === '' && tags.length > 0) {
      e.preventDefault()
      setTags((t) => t.slice(0, -1))
    }
  }

  const save = () => {
    // Fold any half-typed tag into the list before saving.
    const finalTags = tagInput.trim() ? addTags(tags, tagInput) : tags
    onSave(title.trim(), content, finalTags, charLimit)
  }

  return (
    <div className="flex h-full flex-col font-mono text-fg">
      <div className="flex shrink-0 items-center justify-between p-2">
        <Button variant="ghost" onClick={tryCancel} className="gap-1">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
            strokeLinecap="round" strokeLinejoin="round" className="size-4">
            <path d="M15 18l-6-6 6-6" />
          </svg>
          {tr(lang, 'cancel')}
        </Button>
        <Button variant="accent" onClick={save}>{tr(lang, 'save')}</Button>
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-3 px-4 pb-4">
        <label className="flex flex-col gap-1">
          <span className="text-xs text-muted">{tr(lang, 'editor_title')}</span>
          <input
            ref={titleRef}
            className={field}
            value={title}
            placeholder={tr(lang, 'editor_title_placeholder')}
            spellCheck={false}
            onChange={(e) => setTitle(e.target.value)}
          />
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-xs text-muted">{tr(lang, 'editor_tags')}</span>
          <div className={`${field} flex flex-wrap items-center gap-1.5`}>
            {tags.map((t) => (
              <span key={t} className="inline-flex items-center gap-1 rounded-field bg-fg/8 px-1.5 py-0.5 text-xs">
                {t}
                <button
                  type="button"
                  aria-label={tr(lang, 'remove_tag')}
                  onClick={() => setTags((cur) => cur.filter((x) => x !== t))}
                  className="cursor-pointer text-muted outline-none hover:text-fg"
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2"
                    strokeLinecap="round" strokeLinejoin="round" className="size-3">
                    <path d="M18 6 6 18M6 6l12 12" />
                  </svg>
                </button>
              </span>
            ))}
            <input
              className="min-w-24 flex-1 bg-transparent text-sm text-fg outline-none placeholder:text-muted/70"
              value={tagInput}
              placeholder={tr(lang, 'editor_tag_placeholder')}
              spellCheck={false}
              onChange={(e) => setTagInput(e.target.value)}
              onKeyDown={onTagKeyDown}
              onBlur={commitTagInput}
            />
          </div>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-xs text-muted">
            {tr(lang, 'editor_limit')} <span className="text-muted/70">({tr(lang, 'editor_limit_hint')})</span>
          </span>
          <input
            type="number"
            min={0}
            className={`${field} w-32`}
            value={charLimit || ''}
            placeholder="0"
            onChange={(e) => setCharLimit(Math.max(0, Math.floor(Number(e.target.value) || 0)))}
          />
        </label>

        <textarea
          className={`${field} min-h-0 flex-1 resize-none leading-relaxed`}
          value={content}
          placeholder={tr(lang, 'editor_body_placeholder')}
          spellCheck={false}
          onChange={(e) => setContent(e.target.value)}
        />
        <span className={`self-end text-xs tabular-nums ${over ? 'text-danger' : 'text-muted'}`}>
          {charCount}{effectiveLimit > 0 ? ` / ${effectiveLimit}` : ''}
        </span>
      </div>

      {confirmingDiscard && (
        <div className="flex shrink-0 items-center justify-between gap-3 border-t border-[color:var(--shell-border)] bg-surface/60 p-3">
          <span className="text-sm text-fg">{tr(lang, 'discard_changes')}</span>
          <div className="flex gap-2">
            <Button variant="surface" onClick={() => setConfirmingDiscard(false)}>
              {tr(lang, 'keep_editing')}
            </Button>
            <Button variant="accent" onClick={onCancel}>{tr(lang, 'discard')}</Button>
          </div>
        </div>
      )}
    </div>
  )
}
