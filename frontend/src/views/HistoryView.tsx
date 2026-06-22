import { useEffect, useState } from 'react'
import { HistoryService } from '../../bindings/gix/internal/app'
import { MessageCard } from '../components/MessageCard'

export function HistoryView({ onClose }: { onClose: () => void }) {
  const [convs, setConvs] = useState<any[]>([])
  const [detail, setDetail] = useState<any[]>([])

  const reload = () => HistoryService.List().then((c) => setConvs(c ?? []))
  useEffect(() => { reload() }, [])

  return (
    <div className="flex h-full bg-bg text-fg font-mono">
      <div className="w-2/5 border-r border-surface overflow-y-auto">
        <button className="m-2 text-muted" onClick={onClose}>← voltar</button>
        {convs.length === 0 && <div className="p-3 text-muted">Nenhuma conversa salva.</div>}
        {convs.map((c) => (
          <div key={c.ID} className="flex items-center justify-between px-3 py-2 hover:bg-surface cursor-pointer">
            <span className="truncate" onClick={() => HistoryService.Messages(c.ID).then((m) => setDetail(m ?? []))}>{c.Title}</span>
            <button className="text-red-500" onClick={() => HistoryService.Delete(c.ID).then(reload)}>✕</button>
          </div>
        ))}
      </div>
      <div className="flex-1 overflow-y-auto p-3 space-y-3">
        {detail.map((m, i) => (
          <MessageCard key={i} role={m.Role === 'user' ? 'user' : 'assistant'} content={m.Content}
            label={m.Role === 'user' ? 'Você' : 'IA'} />
        ))}
      </div>
    </div>
  )
}
