// Deferred delete: the "undo" pattern (like Gmail's undo-send). A deletion is
// only committed to the backend after a grace period, so undo is trivially
// correct — nothing was actually deleted. The UI removes the item optimistically
// and shows a toast; on undo it puts the item back, on expiry the real delete
// fires. Keeping the timer/flush logic here (pure, injectable) makes it testable
// with fake timers, independent of React.
//
// `commit` performs the real (irreversible) deletion. `onChange` reports the
// currently-pending id (or null) so the UI can show/hide its toast.
export type DeferredDelete = {
  /** Optimistically delete `id`: flushes any pending deletion, then arms a timer. */
  schedule: (id: number) => void
  /** Cancel the pending deletion before it commits (the UI restores the item). */
  undo: () => void
  /** Commit the pending deletion now (e.g. on unmount, or before a new one). */
  flush: () => void
  /** The id awaiting deletion, or null. */
  pending: () => number | null
}

export function createDeferredDelete(opts: {
  delayMs: number
  commit: (id: number) => void
  onChange?: (pendingId: number | null) => void
}): DeferredDelete {
  const { delayMs, commit, onChange } = opts
  let pendingId: number | null = null
  let timer: ReturnType<typeof setTimeout> | null = null

  const clearTimer = () => {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
  }

  const flush = () => {
    if (pendingId === null) return
    clearTimer()
    const id = pendingId
    pendingId = null
    commit(id)
    onChange?.(null)
  }

  const schedule = (id: number) => {
    // Only one deletion may be pending at a time: commit the previous now.
    if (pendingId !== null) {
      clearTimer()
      const prev = pendingId
      pendingId = null
      commit(prev)
    }
    pendingId = id
    onChange?.(id)
    timer = setTimeout(() => {
      timer = null
      const expired = pendingId
      pendingId = null
      onChange?.(null)
      if (expired !== null) commit(expired)
    }, delayMs)
  }

  const undo = () => {
    if (pendingId === null) return
    clearTimer()
    pendingId = null
    onChange?.(null)
  }

  return { schedule, undo, flush, pending: () => pendingId }
}
