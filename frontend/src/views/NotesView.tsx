import { useEffect, useRef, useState } from 'react'
import { motion } from 'motion/react'
import { AlertsService, NotesService } from '../../bindings/gix/internal/app'
import type { Note } from '../../bindings/gix/internal/db'
import { Markdown } from '../components/Markdown'
import { Button } from '../components/Button'
import { UndoToast } from '../components/UndoToast'
import { NoteEditor } from './NoteEditor'
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
  const activeRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    NotesService.List().then((n) => {
      const list = n ?? []
      setNotes(list)
      if (list.length > 0) {
        const targetId = initialActiveId && list.some((note) => note.ID === initialActiveId)
          ? initialActiveId
          : list[0].ID
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
      commit: (id) => { NotesService.Delete(id) },
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
      const idx = notes.findIndex((n) => n.ID === activeId)
      const next = moveSelection(notes.length, idx === -1 ? 0 : idx, e.key === 'ArrowDown' ? 1 : -1)
      setActiveId(notes[next].ID)
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

  const active = notes.find((n) => n.ID === activeId)

  const handleSave = async (title: string, content: string, tags: string[]) => {
    if (editingId === null) return
    const updated = await NotesService.Update(editingId, title, content, tags)
    setNotes((list) => list.map((n) => (n.ID === updated.ID ? updated : n)))
    setActiveId(updated.ID)
    setEditingId(null)
  }

  const handleDelete = (note: Note) => {
    const index = notes.findIndex((n) => n.ID === note.ID)
    const remaining = notes.filter((n) => n.ID !== note.ID)
    setNotes(remaining)
    // Select a neighbour so the detail pane isn't left empty.
    if (activeId === note.ID) {
      const next = remaining[Math.min(index, remaining.length - 1)]
      setActiveId(next ? next.ID : null)
    }
    setPendingDelete({ note, index })
    ddRef.current?.schedule(note.ID)
  }

  const submitAlert = async () => {
    if (alertFor === null) return
    const text = whenText.trim()
    if (text) await AlertsService.CreateForNote(alertFor, text)
    setAlertFor(null); setWhenText('')
  }

  // Summarize and tidy share one "AI rewrites the body, with undo" flow; each gets
  // its own instance. applyRewrite reconciles the list + selection after apply/undo.
  const applyRewrite = (n: Note) => {
    setNotes((list) => list.map((x) => (x.ID === n.ID ? n : x)))
    setActiveId(n.ID)
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
    setActiveId(note.ID)
    ddRef.current?.undo()
  }

  if (editingId !== null && active) {
    return (
      <NoteEditor
        lang={lang}
        note={active}
        onSave={handleSave}
        onCancel={() => setEditingId(null)}
      />
    )
  }

  return (
    <div className="relative flex h-full font-mono text-fg">
      <div className="flex w-2/5 flex-col border-r border-fg/8">
        <div className="shrink-0 p-2">
          <Button variant="ghost" onClick={onClose} className="gap-1">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
              strokeLinecap="round" strokeLinejoin="round" className="size-4">
              <path d="M15 18l-6-6 6-6" />
            </svg>
            {tr(lang, 'cancel')}
          </Button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
          {notes.length === 0 && (
            <div className="px-1 py-3 text-sm text-muted">{tr(lang, 'notes_empty')}</div>
          )}
          <div className="space-y-0.5">
            {notes.map((n, i) => {
              const isActive = activeId === n.ID
              return (
                <motion.div
                  key={n.ID}
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25, ease: 'easeOut', delay: Math.min(i * 0.03, 0.2) }}
                  className={`rounded-field transition-colors duration-150 ${
                    isActive ? 'bg-surface shadow-[var(--shadow-border)]' : 'hover:bg-surface/60'
                  }`}
                >
                  <button
                    ref={isActive ? activeRef : undefined}
                    onClick={() => setActiveId(n.ID)}
                    className="block w-full cursor-pointer truncate px-2.5 py-2 text-left text-sm outline-none"
                  >
                    {n.Title}
                  </button>
                </motion.div>
              )
            })}
          </div>
        </div>
      </div>
      <div className="flex min-h-0 flex-1 flex-col">
        {active && (
          <>
            <div className="flex shrink-0 justify-end gap-1 p-2">
              <Button variant="ghost" onClick={() => { setAlertFor(active.ID); setWhenText('') }} className="gap-1">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
                  strokeLinecap="round" strokeLinejoin="round" className="size-4">
                  <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" />
                  <path d="M13.7 21a2 2 0 0 1-3.4 0" />
                </svg>
                {tr(lang, 'alert_from_note')}
              </Button>
              <Button variant="ghost" disabled={summary.busy}
                onClick={() => summary.run(active, async (id) => {
                  const r = await NotesService.Summarize(id)
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
                  const r = await NotesService.Tidy(id)
                  return r.status === 'ok' ? r.content : null
                })} className="gap-1">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
                  strokeLinecap="round" strokeLinejoin="round" className="size-4">
                  <path d="M3 6h18M7 12h10M10 18h4" />
                </svg>
                {tr(lang, 'tidy')}
              </Button>
              <Button variant="ghost" onClick={() => setEditingId(active.ID)} className="gap-1">
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
            {alertFor === active.ID && (
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
              <h1 className="mb-3 font-mono text-base font-bold text-fg">{active.Title}</h1>
              <Markdown>{active.Content}</Markdown>
            </article>
          </>
        )}
      </div>

      <UndoToast
        open={!!pendingDelete}
        message={tr(lang, 'note_deleted')}
        title={pendingDelete?.note.Title ?? ''}
        undoLabel={tr(lang, 'undo')}
        onUndo={handleUndo}
      />
      <UndoToast
        open={!!summary.pending}
        message={tr(lang, 'note_summarized')}
        title={summary.pending?.note.Title ?? ''}
        undoLabel={tr(lang, 'undo')}
        onUndo={summary.undo}
      />
      <UndoToast
        open={!!tidy.pending}
        message={tr(lang, 'note_tidied')}
        title={tidy.pending?.note.Title ?? ''}
        undoLabel={tr(lang, 'undo')}
        onUndo={tidy.undo}
      />
    </div>
  )
}
