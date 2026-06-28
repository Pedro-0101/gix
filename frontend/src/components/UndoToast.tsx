import { motion, AnimatePresence } from "motion/react"

// A bottom-centered toast with an undo action, shown while a reversible action
// (delete, summarize) is within its grace period.
export function UndoToast({
  open,
  message,
  title,
  undoLabel,
  onUndo,
}: {
  open: boolean
  message: string
  title: string
  undoLabel: string
  onUndo: () => void
}) {
  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 12 }}
          transition={{ type: "spring", duration: 0.3, bounce: 0 }}
          className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-3 rounded-field bg-surface px-3 py-2 text-sm shadow-[var(--shadow-border)]"
        >
          <span className="text-muted">
            {message}
            <span className="ml-1 text-fg">{title}</span>
          </span>
          <button
            onClick={onUndo}
            className="cursor-pointer font-medium text-accent outline-none hover:brightness-110"
          >
            {undoLabel}
          </button>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
