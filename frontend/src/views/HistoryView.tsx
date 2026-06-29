import { useEffect, useState } from 'react'
import { motion } from 'motion/react'
import { HistoryService } from '../api/services'
import { MessageCard } from '../components/MessageCard'
import { Button } from '../components/Button'
import { tr } from '../i18n'

function TrashIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
      strokeLinecap="round" strokeLinejoin="round" className="size-4">
      <path d="M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2m2 0v14a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V6" />
      <path d="M10 11v6M14 11v6" />
    </svg>
  )
}

export function HistoryView({ lang, onClose }: { lang: string; onClose: () => void }) {
  const [convs, setConvs] = useState<any[]>([])
  const [detail, setDetail] = useState<any[]>([])
  const [activeId, setActiveId] = useState<any>(null)

  const reload = () => HistoryService.list().then((c) => setConvs(c ?? []))
  useEffect(() => { reload() }, [])

  const open = (id: any) => {
    setActiveId(id)
    HistoryService.messages(id).then((m) => setDetail(m ?? []))
  }

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
          {convs.length === 0 && (
            <div className="px-1 py-3 text-sm text-muted">{tr(lang, 'history_empty')}</div>
          )}
          <div className="space-y-0.5">
            {convs.map((c, i) => {
              const active = activeId === c.id
              return (
                <motion.div
                  key={c.id}
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25, ease: 'easeOut', delay: Math.min(i * 0.03, 0.2) }}
                  className={`group flex items-center gap-1 rounded-field pl-2.5 pr-1 transition-colors duration-150 ${
                    active ? 'bg-surface shadow-[var(--shadow-border)]' : 'hover:bg-surface/60'
                  }`}
                >
                  <button
                    onClick={() => open(c.id)}
                    className="min-w-0 flex-1 cursor-pointer truncate py-2 text-left text-sm outline-none"
                  >
                    {c.title}
                  </button>
                  <button
                    onClick={() => HistoryService.delete(c.id).then(() => {
                      if (activeId === c.id) { setDetail([]); setActiveId(null) }
                      reload()
                    })}
                    aria-label={tr(lang, 'cancel')}
                    className="grid size-9 shrink-0 place-items-center rounded-field text-muted opacity-0 outline-none transition-[scale,opacity,color,background-color] duration-150 ease-out hover:bg-danger/10 hover:text-danger focus-visible:opacity-100 active:scale-[0.96] group-hover:opacity-100 focus-visible:shadow-[0_0_0_2px_var(--color-bg),0_0_0_4px_var(--ring-focus)]"
                  >
                    <TrashIcon />
                  </button>
                </motion.div>
              )
            })}
          </div>
        </div>
      </div>
      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-3 selectable">
        {detail.map((m, i) => (
          <MessageCard key={i} role={m.role === 'user' ? 'user' : 'assistant'} content={m.content}
            label={m.role === 'user' ? tr(lang, 'you') : tr(lang, 'ai')} />
        ))}
      </div>
    </div>
  )
}
