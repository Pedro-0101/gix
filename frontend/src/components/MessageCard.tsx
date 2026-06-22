import ReactMarkdown from 'react-markdown'

export function MessageCard({ role, content, label }:
  { role: 'user' | 'assistant'; content: string; label: string }) {
  const isUser = role === 'user'
  return (
    <div className={`flex flex-col ${isUser ? 'items-end' : 'items-start'}`}>
      <span className="text-xs font-bold text-muted mb-1">{label}</span>
      <div className="max-w-[75%] rounded-card bg-bubble px-3 py-2 text-fg font-mono whitespace-pre-wrap">
        <ReactMarkdown>{content}</ReactMarkdown>
      </div>
    </div>
  )
}
