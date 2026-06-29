import { useEffect, useRef, useState } from 'react'
import { NotesService } from '../api/services'
import type { Note } from '../api/types'

// useNoteRewrite encapsulates the "AI rewrites a note's body, with undo" flow
// shared by the summarize and tidy buttons in the notes view. Call run() with an
// async producer of the new body: it replaces the note via Update, keeps the
// original for `graceMs` so undo() can restore it, and reports `busy` while the
// AI runs (to disable the trigger) and `pending` to drive the undo toast.
// onApplied reconciles the view (list + selection) after both apply and undo.
export function useNoteRewrite(graceMs: number, onApplied: (note: Note) => void) {
  const [busy, setBusy] = useState(false)
  const [pending, setPending] = useState<{ note: Note } | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => { if (timer.current) clearTimeout(timer.current) }, [])

  // run swaps note's body for produce()'s result (null = nothing to apply),
  // keeping the original around so the toast can undo it.
  const run = async (note: Note, produce: (id: number) => Promise<string | null>) => {
    setBusy(true)
    try {
      const body = await produce(note.id)
      if (!body) return
      onApplied(await NotesService.update(note.id, note.title, body, note.tags ?? []))
      setPending({ note })
      if (timer.current) clearTimeout(timer.current)
      timer.current = setTimeout(() => setPending(null), graceMs)
    } finally {
      setBusy(false)
    }
  }

  const undo = async () => {
    if (!pending) return
    const { note } = pending
    onApplied(await NotesService.update(note.id, note.title, note.content, note.tags ?? []))
    setPending(null)
    if (timer.current) clearTimeout(timer.current)
  }

  return { busy, pending, run, undo }
}
