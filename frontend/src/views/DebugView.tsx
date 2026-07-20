import { useState, useEffect, useRef } from 'react'
import type { LogEntry } from '../lib/useInteractionLog'

type Props = {
  log: LogEntry[]
  paused: boolean
  onTogglePause: () => void
  onClear: () => void
  onClose: () => void
}

export function DebugView(p: Props) {
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!p.paused) endRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [p.log, p.paused])

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <div className="flex items-center justify-between border-b border-[color:var(--shell-border)] px-3 py-2">
        <span className="text-[11px] font-semibold tracking-wide text-accent">Debug — Interações</span>
        <div className="flex items-center gap-2">
          <span className="font-mono text-[10px] text-muted">{p.log.length} eventos</span>
          <button onClick={p.onTogglePause}
            className="rounded px-2 py-0.5 text-[10px] font-medium transition-colors hover:bg-surface text-muted hover:text-fg">
            {p.paused ? '▶ Retomar' : '⏸ Pausar'}
          </button>
          <button onClick={p.onClear}
            className="rounded px-2 py-0.5 text-[10px] font-medium transition-colors hover:bg-surface text-muted hover:text-fg">
            🗑 Limpar
          </button>
          <button onClick={p.onClose}
            className="rounded px-2 py-0.5 text-[10px] font-medium transition-colors hover:bg-surface text-muted hover:text-fg">
            ✕ Fechar
          </button>
        </div>
      </div>
      <div className="flex-1 overflow-y-auto font-mono text-[11px] leading-relaxed">
        {p.log.length === 0 ? (
          <div className="flex h-full items-center justify-center text-xs text-muted">
            Nenhuma interação registrada ainda.
          </div>
        ) : (
          <div className="space-y-0">
            {p.log.map((entry) => (
              <LogRow key={entry.id} entry={entry} />
            ))}
          </div>
        )}
        <div ref={endRef} />
      </div>
    </div>
  )
}

function LogRow({ entry }: { entry: LogEntry }) {
  const [expanded, setExpanded] = useState(false)
  return (
    <div className="border-b border-[color:var(--shell-border)] last:border-0">
      <button onClick={() => setExpanded((e) => !e)}
        className="flex w-full items-start gap-2 px-3 py-1.5 text-left transition-colors hover:bg-surface/50">
        <Badge type={entry.type} />
        <span className="shrink-0 text-[10px] text-muted">{entry.timestamp}</span>
        <span className="min-w-0 flex-1 truncate text-fg">{entrySummary(entry)}</span>
        <span className="shrink-0 text-[10px] text-muted">{expanded ? '▲' : '▼'}</span>
      </button>
      {expanded && (
        <pre className="overflow-x-auto px-3 pb-2 text-[10px] text-muted">
          {JSON.stringify(entry, null, 2)}
        </pre>
      )}
    </div>
  )
}

function Badge({ type }: { type: LogEntry['type'] }) {
  const colors: Record<LogEntry['type'], string> = {
    user_msg: 'bg-blue-500/20 text-blue-400',
    delta: 'bg-green-500/20 text-green-400',
    done: 'bg-emerald-500/20 text-emerald-400',
    error: 'bg-red-500/20 text-red-400',
    usage: 'bg-purple-500/20 text-purple-400',
    note_proposed: 'bg-amber-500/20 text-amber-400',
    alert_proposed: 'bg-orange-500/20 text-orange-400',
    clear: 'bg-gray-500/20 text-gray-400',
  }
  const labels: Record<LogEntry['type'], string> = {
    user_msg: 'USER',
    delta: 'DELTA',
    done: 'DONE',
    error: 'ERR',
    usage: 'COST',
    note_proposed: 'NOTE',
    alert_proposed: 'ALERT',
    clear: 'CLR',
  }
  return (
    <span className={`rounded px-1 py-0.5 text-[9px] font-bold ${colors[type]}`}>
      {labels[type]}
    </span>
  )
}

function entrySummary(entry: LogEntry): string {
  switch (entry.type) {
    case 'user_msg': return `Usuário: ${trunc(entry.content ?? '', 80)}`
    case 'delta': return `Δ ${trunc(entry.content ?? '', 80)}`
    case 'done': return `✓ Resposta completa (${(entry.content ?? '').length} chars)`
    case 'error': return `✗ ${entry.content ?? ''}`
    case 'usage': return `$ ${entry.usage?.cost?.toFixed(6) ?? '?'} · ${entry.usage?.tokens ?? '?'} tokens`
    case 'note_proposed': return `📝 Nota: ${entry.note?.title ?? ''}`
    case 'alert_proposed': return `⏰ Alerta: ${entry.alert?.message ?? ''}`
    case 'clear': return 'Log limpo'
  }
}

function trunc(s: string, n: number): string {
  return s.length <= n ? s : s.slice(0, n) + '…'
}
