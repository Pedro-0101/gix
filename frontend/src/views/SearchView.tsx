import { useEffect, useRef, useState } from 'react'
import { motion } from 'motion/react'
import { Markdown } from '../components/Markdown'
import { Button } from '../components/Button'
import { moveSelection } from '../commands/interaction'
import type { SearchState } from '../commands/types'
import { tr } from '../i18n'

// Results browser for /find and /ask: a ranked hit list on the left, the
// selected note on the right. In 'ask' mode the AI summary sits above the note.
// Mirrors NotesView's master-detail layout and arrow-key navigation.
export function SearchView({ lang, state, onClose }: { lang: string; state: SearchState; onClose: () => void }) {
  const { query, mode, loading, hits, summary, status } = state
  const [activeId, setActiveId] = useState<number | null>(null)
  const activeRef = useRef<HTMLButtonElement>(null)

  // Select the first hit whenever a new result set arrives.
  useEffect(() => {
    setActiveId(hits.length > 0 ? hits[0].noteId : null)
  }, [hits])

  // ↑/↓ walk the list (with wrap). Capture phase to beat the bar's history recall.
  useEffect(() => {
    if (hits.length === 0) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
      e.preventDefault()
      e.stopPropagation()
      const idx = hits.findIndex((h) => h.noteId === activeId)
      const next = moveSelection(hits.length, idx === -1 ? 0 : idx, e.key === 'ArrowDown' ? 1 : -1)
      setActiveId(hits[next].noteId)
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [hits, activeId])

  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest' })
  }, [activeId])

  const active = hits.find((h) => h.noteId === activeId)

  return (
    <div className="flex h-full font-mono text-fg">
      <div className="flex w-2/5 flex-col border-r border-fg/8">
        <div className="flex shrink-0 items-center gap-2 p-2">
          <Button variant="ghost" onClick={onClose} className="gap-1">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
              strokeLinecap="round" strokeLinejoin="round" className="size-4">
              <path d="M15 18l-6-6 6-6" />
            </svg>
            {tr(lang, 'cancel')}
          </Button>
          <span className="truncate text-xs text-muted">
            {mode === 'ask' ? tr(lang, 'ask_label') : tr(lang, 'find_label')}: “{query}”
          </span>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
          {loading && <div className="px-1 py-3 text-sm text-muted">{tr(lang, 'search_searching')}</div>}
          {!loading && hits.length === 0 && (
            <div className="px-1 py-3 text-sm text-muted">{tr(lang, 'search_empty')}</div>
          )}
          <div className="space-y-0.5">
            {hits.map((h, i) => {
              const isActive = activeId === h.noteId
              return (
                <motion.div
                  key={h.noteId}
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25, ease: 'easeOut', delay: Math.min(i * 0.03, 0.2) }}
                  className={`rounded-field transition-colors duration-150 ${
                    isActive ? 'bg-surface shadow-[var(--shadow-border)]' : 'hover:bg-surface/60'
                  }`}
                >
                  <button
                    ref={isActive ? activeRef : undefined}
                    onClick={() => setActiveId(h.noteId)}
                    className="block w-full cursor-pointer px-2.5 py-2 text-left outline-none"
                  >
                    <div className="truncate text-sm">{h.title}</div>
                    <div className="truncate text-xs text-muted">{h.snippet}</div>
                    {h.tags?.length > 0 && (
                      <div className="mt-1 truncate text-[11px] text-accent">
                        {h.tags.map((t) => '#' + t).join(' ')}
                      </div>
                    )}
                  </button>
                </motion.div>
              )
            })}
          </div>
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-4 selectable">
        {mode === 'ask' && !loading && (
          <SummaryPanel lang={lang} summary={summary} status={status} />
        )}
        {active && (
          <article>
            <h1 className="mb-3 font-mono text-base font-bold text-fg">{active.title}</h1>
            <Markdown>{active.content}</Markdown>
          </article>
        )}
      </div>
    </div>
  )
}

// The AI answer for /ask, shown above the source note. Handles the empty and
// missing-key statuses with a plain message instead of a summary.
function SummaryPanel({ lang, summary, status }: { lang: string; summary?: string; status?: string }) {
  let body = summary
  if (status === 'no_api_key') body = tr(lang, 'no_api_key')
  else if (status === 'empty') body = tr(lang, 'search_empty')
  if (!body) return null
  return (
    <div className="mb-4 rounded-field border border-fg/8 bg-surface/60 p-3">
      <div className="mb-1.5 text-[11px] font-semibold tracking-wide text-accent">{tr(lang, 'ask_summary')}</div>
      <Markdown>{body}</Markdown>
    </div>
  )
}
