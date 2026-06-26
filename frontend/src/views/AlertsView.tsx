import { useCallback, useEffect, useRef, useState } from 'react'
import { motion } from 'motion/react'
import { AlertsService } from '../../bindings/gix/internal/app'
import type { Alert } from '../../bindings/gix/internal/db'
import { Button } from '../components/Button'
import { moveSelection } from '../commands/interaction'
import { recurrenceLabel, formatFireAt } from '../lib/alerts'
import { tr } from '../i18n'

// Alerts manager: pending alerts (fire_at asc) with Snooze / Done / Delete, plus
// a collapsed section of done/cancelled. Arrow keys move the selection (capture
// phase, like NotesView). Esc closes via App's global handler.
export function AlertsView({ lang, focusId, onClose }: { lang: string; focusId: number | null; onClose: () => void }) {
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [activeId, setActiveId] = useState<number | null>(null)
  const [showDone, setShowDone] = useState(false)
  const activeRef = useRef<HTMLButtonElement>(null)

  const load = useCallback(() => {
    AlertsService.List().then((a) => {
      const list = a ?? []
      setAlerts(list)
      setActiveId((cur) => cur ?? (focusId ?? (list.find((x) => x.Status === 'pending')?.ID ?? null)))
    })
  }, [focusId])

  useEffect(() => { load() }, [load])
  useEffect(() => { if (focusId != null) setActiveId(focusId) }, [focusId])

  const pending = alerts.filter((a) => a.Status === 'pending')
  const done = alerts.filter((a) => a.Status !== 'pending')

  useEffect(() => {
    if (pending.length === 0) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
      e.preventDefault(); e.stopPropagation()
      const idx = pending.findIndex((a) => a.ID === activeId)
      const next = moveSelection(pending.length, idx === -1 ? 0 : idx, e.key === 'ArrowDown' ? 1 : -1)
      setActiveId(pending[next].ID)
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [pending, activeId])

  useEffect(() => { activeRef.current?.scrollIntoView({ block: 'nearest' }) }, [activeId])

  const snooze = async (a: Alert) => { await AlertsService.Snooze(a.ID, 10); load() }
  const complete = async (a: Alert) => { await AlertsService.Done(a.ID); load() }
  const remove = async (a: Alert) => { await AlertsService.Delete(a.ID); load() }

  const meta = (a: Alert) => [formatFireAt(a.FireAt, lang), recurrenceLabel(lang, a.Recurrence)].filter(Boolean).join(' · ')

  return (
    <div className="flex h-full flex-col font-mono text-fg">
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
        {pending.length === 0 && (
          <div className="px-1 py-3 text-sm text-muted">{tr(lang, 'alerts_empty')}</div>
        )}
        <div className="space-y-0.5">
          {pending.map((a, i) => {
            const isActive = activeId === a.ID
            return (
              <motion.div
                key={a.ID}
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25, ease: 'easeOut', delay: Math.min(i * 0.03, 0.2) }}
                className={`rounded-field transition-colors duration-150 ${isActive ? 'bg-surface shadow-[var(--shadow-border)]' : 'hover:bg-surface/60'}`}
              >
                <div className="flex items-center justify-between gap-2 px-2.5 py-2">
                  <button
                    ref={isActive ? activeRef : undefined}
                    onClick={() => setActiveId(a.ID)}
                    className="min-w-0 flex-1 cursor-pointer text-left outline-none"
                  >
                    <div className="truncate text-sm">
                      {a.NoteID ? '📝 ' : ''}{a.Message}
                    </div>
                    <div className="truncate text-xs text-muted">{meta(a)}</div>
                  </button>
                  <div className="flex shrink-0 gap-1">
                    <Button variant="ghost" onClick={() => snooze(a)}>{tr(lang, 'alert_snooze')}</Button>
                    <Button variant="ghost" onClick={() => complete(a)}>{tr(lang, 'alert_done')}</Button>
                    <Button variant="ghost" onClick={() => remove(a)}>{tr(lang, 'alert_cancel')}</Button>
                  </div>
                </div>
              </motion.div>
            )
          })}
        </div>

        {done.length > 0 && (
          <div className="mt-3">
            <button
              onClick={() => setShowDone((v) => !v)}
              className="cursor-pointer px-1 py-1 text-xs text-muted outline-none hover:text-fg"
            >
              {showDone ? '▾' : '▸'} {tr(lang, 'alerts_done_section')} ({done.length})
            </button>
            {showDone && (
              <div className="space-y-0.5 opacity-60">
                {done.map((a) => (
                  <div key={a.ID} className="flex items-center justify-between gap-2 px-2.5 py-1.5">
                    <span className="min-w-0 flex-1 truncate text-sm line-through">{a.Message}</span>
                    <Button variant="ghost" onClick={() => remove(a)}>{tr(lang, 'alert_cancel')}</Button>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
