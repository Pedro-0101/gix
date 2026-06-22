import { useEffect, useRef, useState } from 'react'
import { ChatService } from '../../bindings/gix/internal/app'
import { onChatDelta, onChatDone, onChatError, onChatUsage } from '../lib/events'
import { MessageCard } from '../components/MessageCard'
import { tr } from '../i18n'

type Msg = { role: 'user' | 'assistant'; content: string }

export function ChatView({ lang }: { lang: string }) {
  const [msgs, setMsgs] = useState<Msg[]>([])
  const [input, setInput] = useState('')
  const [usage, setUsage] = useState<{ tokens: number; cost: number } | null>(null)
  const streamingRef = useRef(false)
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const offDelta = onChatDelta((delta) => {
      setMsgs((m) => {
        const copy = [...m]
        const last = copy[copy.length - 1]
        if (last && last.role === 'assistant') last.content += delta
        return copy
      })
    })
    const offDone = onChatDone(() => { streamingRef.current = false })
    const offErr = onChatError((code) => {
      streamingRef.current = false
      setMsgs((m) => {
        const copy = [...m]
        const last = copy[copy.length - 1]
        const text = code === 'no_api_key' ? tr(lang, 'no_api_key') : `${tr(lang, 'error_prefix')}${code}`
        if (last && last.role === 'assistant') last.content = text
        return copy
      })
    })
    const offUsage = onChatUsage((u) => setUsage(u))
    return () => { offDelta(); offDone(); offErr(); offUsage() }
  }, [lang])

  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [msgs])

  const send = () => {
    const text = input.trim()
    if (!text) return
    if (text.startsWith('/')) {
      if (text === '/new') {
        ChatService.NewConversation()
        setMsgs([]); setUsage(null); setInput('')
        return
      }
    }
    setMsgs((m) => [...m, { role: 'user', content: text }, { role: 'assistant', content: tr(lang, 'thinking') }])
    // limpa o placeholder "pensando…" no primeiro delta
    setMsgs((m) => { const c = [...m]; if (c.length) c[c.length - 1].content = ''; return c })
    streamingRef.current = true
    ChatService.Send(text)
    setInput('')
  }

  return (
    <div className="flex h-full flex-col bg-bg">
      {usage && (
        <div className="px-3 py-1 text-xs text-muted font-mono">
          Tokens: {usage.tokens} | ${usage.cost.toFixed(6)}
        </div>
      )}
      <div className="flex-1 overflow-y-auto px-3 py-2 space-y-3">
        {msgs.map((m, i) => (
          <MessageCard key={i} role={m.role} content={m.content}
            label={m.role === 'user' ? tr(lang, 'you') : tr(lang, 'ai')} />
        ))}
        <div ref={endRef} />
      </div>
      <textarea
        className="m-2 rounded-card bg-surface p-2 text-fg font-mono resize-none outline-none"
        rows={2} value={input} placeholder={tr(lang, 'placeholder')}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() } }}
      />
    </div>
  )
}
