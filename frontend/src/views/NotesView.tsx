import { useEffect, useRef, useState } from 'react'
import { AlertsService, NotesService, Prefs } from '../api/services'
import type { Note } from '../api/types'
import { Markdown } from '../components/Markdown'
import { Button } from '../components/Button'
import { UndoToast } from '../components/UndoToast'
import { NoteEditor } from './NoteEditor'
import { NoteList } from './NoteList'
import { moveSelection } from '../commands/interaction'
import { createDeferredDelete, type DeferredDelete } from '../lib/deferredDelete'
import { useNoteRewrite } from '../lib/useNoteRewrite'
import { tr } from '../i18n'

const DELETE_GRACE_MS = 5000

// Notes browser: title list on the left, selected note's markdown on the right,
// with Edit/Delete actions. Editing swaps in a full-screen NoteEditor; deleting
// is deferred (undo via toast) so it's reversible until the grace period ends.
// Esc closes via App's global handler; the back button calls onClose.
export function NotesView({ lang, onClose, initialActiveId }: { lang: string; onClose: () => void; initialActiveId?: number | null }) {
  const [notes, setNotes] = useState<Note[]>([])
  const [activeId, setActiveId] = useState<number | null>(null)
  const [editingId, setEditingId] = useState<number | null>(null)
  // The note awaiting deletion (kept so undo can restore it at its old spot).
  const [pendingDelete, setPendingDelete] = useState<{ note: Note; index: number } | null>(null)
  // When non-null, the detail pane shows a small "when?" input to schedule an
  // alert for the selected note.
  const [alertFor, setAlertFor] = useState<number | null>(null)
  const [whenText, setWhenText] = useState('')
  // Global default note size limit, shown in the editor's counter when a note has
  // no per-note override.
  const [defaultLimit, setDefaultLimit] = useState(8000)
  const activeRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    Prefs.get().then((p) => {
      if (p.charLimit) setDefaultLimit(p.charLimit)
    }).catch(() => {})
  }, [])

  useEffect(() => {
    NotesService.list().then((n) => {
      const list = n ?? []
      setNotes(list)
      if (list.length > 0) {
        const targetId = initialActiveId && list.some((note) => note.id === initialActiveId)
          ? initialActiveId
          : list[0].id
        setActiveId(targetId)
      }
    })
  }, [initialActiveId])

  // One deferred-delete coordinator for the view's lifetime. commit performs the
  // real backend delete; onChange(null) hides the toast once it's committed.
  const ddRef = useRef<DeferredDelete | null>(null)
  if (ddRef.current === null) {
    ddRef.current = createDeferredDelete({
      delayMs: DELETE_GRACE_MS,
      commit: (id) => { NotesService.delete(id) },
      onChange: (id) => { if (id === null) setPendingDelete(null) },
    })
  }
  // Flush any pending deletion when the view unmounts (closing notes / window).
  useEffect(() => () => ddRef.current?.flush(), [])

  // ↑/↓ percorrem a lista (com wrap), espelhando a navegação do card de escolha.
  // Captura para vencer o recall de histórico da barra. Desativado durante a
  // edição para não roubar as setas do editor.
  useEffect(() => {
    if (notes.length === 0 || editingId !== null) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
      e.preventDefault()
      e.stopPropagation()
      const idx = notes.findIndex((n) => n.id === activeId)
      const next = moveSelection(notes.length, idx === -1 ? 0 : idx, e.key === 'ArrowDown' ? 1 : -1)
      setActiveId(notes[next].id)
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [notes, activeId, editingId])

  // `a` on the selected note opens the alert when-input (disabled while editing
  // or while the input is already open, so typing "a" there isn't intercepted).
  useEffect(() => {
    if (editingId !== null || alertFor !== null || activeId === null) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'a') return
      e.preventDefault(); e.stopPropagation()
      setAlertFor(activeId); setWhenText('')
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [activeId, editingId, alertFor])

  // Mantém a nota selecionada visível ao navegar pelo teclado.
  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest' })
  }, [activeId])

  const active = notes.find((n) => n.id === activeId)

  const handleSave = async (title: string, content: string, tags: string[], charLimit: number) => {
    if (editingId === null) return
    // Persist the override first so the re-fetched note carries the new limit.
    await NotesService.setCharLimit(editingId, charLimit)
    const updated = await NotesService.update(editingId, title, content, tags)
    setNotes((list) => list.map((n) => (n.id === updated.id ? updated : n)))
    setActiveId(updated.id)
    setEditingId(null)
  }

  const handleDelete = (note: Note) => {
    const index = notes.findIndex((n) => n.id === note.id)
    const remaining = notes.filter((n) => n.id !== note.id)
    setNotes(remaining)
    // Select a neighbour so the detail pane isn't left empty.
    if (activeId === note.id) {
      const next = remaining[Math.min(index, remaining.length - 1)]
      setActiveId(next ? next.id : null)
    }
    setPendingDelete({ note, index })
    ddRef.current?.schedule(note.id)
  }

  const submitAlert = async () => {
    if (alertFor === null) return
    const text = whenText.trim()
    if (text) await AlertsService.createForNote(alertFor, text)
    setAlertFor(null); setWhenText('')
  }

  // Summarize and tidy share one "AI rewrites the body, with undo" flow; each gets
  // its own instance. applyRewrite reconciles the list + selection after apply/undo.
  const applyRewrite = (n: Note) => {
    setNotes((list) => list.map((x) => (x.id === n.id ? n : x)))
    setActiveId(n.id)
  }
  const summary = useNoteRewrite(DELETE_GRACE_MS, applyRewrite)
  const tidy = useNoteRewrite(DELETE_GRACE_MS, applyRewrite)

  const handleUndo = () => {
    if (!pendingDelete) return
    const { note, index } = pendingDelete
    setNotes((list) => {
      const copy = [...list]
      copy.splice(Math.min(index, copy.length), 0, note)
      return copy
    })
    setActiveId(note.id)
    ddRef.current?.undo()
  }

  if (editingId !== null && active) {
    return (
      <NoteEditor
        lang={lang}
        note={active}
        defaultCharLimit={defaultLimit}
        onSave={handleSave}
        onCancel={() => setEditingId(null)}
      />
    )
  }

  return (
    <div className="relative flex h-full font-mono text-fg">
      <NoteList
        lang={lang}
        notes={notes}
        activeId={activeId}
        setActiveId={setActiveId}
        activeRef={activeRef}
        onClose={onClose}
      />
      <div className="flex min-h-0 flex-1 flex-col">
        {active && (
          <>
            <div className="flex shrink-0 justify-end gap-1 p-2">
              <Button variant="ghost" onClick={() => { setAlertFor(active.id); setWhenText('') }} className="gap-1">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
                  strokeLinecap="round" strokeLinejoin="round" className="size-4">
                  <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" />
                  <path d="M13.7 21a2 2 0 0 1-3.4 0" />
                </svg>
                {tr(lang, 'alert_from_note')}
              </Button>
              <Button variant="ghost" disabled={summary.busy}
                onClick={() => summary.run(active, async (id) => {
                  const r = await NotesService.summarize(id)
                  return r.status === 'ok' ? r.summary : null
                })} className="gap-1">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
                  strokeLinecap="round" strokeLinejoin="round" className="size-4">
                  <path d="M4 6h16M4 12h16M4 18h10" />
                </svg>
                {tr(lang, 'summarize')}
              </Button>
              <Button variant="ghost" disabled={tidy.busy}
                onClick={() => tidy.run(active, async (id) => {
                  const r = await NotesService.tidy(id)
                  return r.status === 'ok' ? r.content : null
                })} className="gap-1">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
                  strokeLinecap="round" strokeLinejoin="round" className="size-4">
                  <path d="M3 6h18M7 12h10M10 18h4" />
                </svg>
                {tr(lang, 'tidy')}
              </Button>
              <Button variant="ghost" onClick={() => setEditingId(active.id)} className="gap-1">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
                  strokeLinecap="round" strokeLinejoin="round" className="size-4">
                  <path d="M12 20h9" />
                  <path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z" />
                </svg>
                {tr(lang, 'edit')}
              </Button>
              <Button variant="ghost" onClick={() => handleDelete(active)} className="gap-1">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
                  strokeLinecap="round" strokeLinejoin="round" className="size-4">
                  <path d="M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2m2 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" />
                </svg>
                {tr(lang, 'delete')}
              </Button>
            </div>
            {alertFor === active.id && (
              <div className="shrink-0 px-4 pb-2">
                <input
                  autoFocus
                  value={whenText}
                  onChange={(e) => setWhenText(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') { e.preventDefault(); submitAlert() }
                    if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); setAlertFor(null) }
                  }}
                  placeholder={tr(lang, 'alert_when_placeholder')}
                  className="w-full rounded-field bg-surface px-2.5 py-1.5 text-sm outline-none placeholder:text-muted/70"
                />
              </div>
            )}
            <article className="min-h-0 flex-1 overflow-y-auto px-4 pb-4 selectable">
              <h1 className="mb-3 font-mono text-base font-bold text-fg">{active.title}</h1>
              <Markdown>{active.content}</Markdown>
            </article>
          </>
        )}
      </div>

      <UndoToast
        open={!!pendingDelete}
        message={tr(lang, 'note_deleted')}
        title={pendingDelete?.note.title ?? ''}
        undoLabel={tr(lang, 'undo')}
        onUndo={handleUndo}
      />
      <UndoToast
        open={!!summary.pending}
        message={tr(lang, 'note_summarized')}
        title={summary.pending?.note.title ?? ''}
        undoLabel={tr(lang, 'undo')}
        onUndo={summary.undo}
      />
      <UndoToast
        open={!!tidy.pending}
        message={tr(lang, 'note_tidied')}
        title={tidy.pending?.note.title ?? ''}
        undoLabel={tr(lang, 'undo')}
        onUndo={tidy.undo}
      />
    </div>
  )
}
