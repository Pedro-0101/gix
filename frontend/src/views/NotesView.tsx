import { useEffect, useState } from 'react'
import { motion } from 'motion/react'
import { NotesService } from '../../bindings/gix/internal/app'
import type { Note } from '../../bindings/gix/internal/db'
import { MessageCard } from '../components/MessageCard'
import { Button } from '../components/Button'
import { tr } from '../i18n'

// Read-only notes browser: title list on the left, selected note's markdown on
// the right. Mirrors HistoryView's master-detail layout. Esc closes via App's
// global handler; the back button calls onClose.
export function NotesView({ lang, onClose }: { lang: string; onClose: () => void }) {
  const [notes, setNotes] = useState<Note[]>([])
  const [activeId, setActiveId] = useState<number | null>(null)

  useEffect(() => {
    NotesService.List().then((n) => {
      const list = n ?? []
      setNotes(list)
      if (list.length > 0) setActiveId(list[0].ID)
    })
  }, [])

  const active = notes.find((n) => n.ID === activeId)

  return (
    <div className="flex h-full font-mono text-fg">
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
      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-3 selectable">
        {active && (
          <MessageCard role="assistant" content={active.Content} label={active.Title} />
        )}
      </div>
    </div>
  )
}
