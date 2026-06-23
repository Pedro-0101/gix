import { motion } from 'motion/react'
import ReactMarkdown from 'react-markdown'

export function MessageCard({ role, content, label, pending }:
  { role: 'user' | 'assistant'; content: string; label: string; pending?: boolean }) {
  const isUser = role === 'user'
  return (
    <motion.div
      initial={{ opacity: 0, y: 8, filter: 'blur(4px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={{ type: 'spring', duration: 0.35, bounce: 0 }}
      className={`flex flex-col ${isUser ? 'items-end' : 'items-start'}`}
    >
      <span className="mb-1 px-1 text-[11px] font-semibold tracking-wide text-muted">{label}</span>
      <div
        className={
          `max-w-[78%] rounded-card px-3 py-2 font-mono text-sm leading-relaxed shadow-[var(--shadow-border)] ` +
          `[&_p]:[text-wrap:pretty] [&_*]:whitespace-pre-wrap ${
            isUser
              ? 'rounded-tr-sm bg-accent text-white'
              : 'rounded-tl-sm bg-bubble text-fg'
          }`
        }
      >
        {pending ? (
          <span className="inline-flex items-center gap-1 text-muted">
            {content}
            <span className="inline-flex gap-0.5">
              <Dot delay={0} />
              <Dot delay={0.15} />
              <Dot delay={0.3} />
            </span>
          </span>
        ) : (
          <ReactMarkdown>{content}</ReactMarkdown>
        )}
      </div>
    </motion.div>
  )
}

function Dot({ delay }: { delay: number }) {
  return (
    <motion.span
      className="inline-block size-1 rounded-full bg-current"
      animate={{ opacity: [0.25, 1, 0.25] }}
      transition={{ duration: 1, repeat: Infinity, ease: 'easeInOut', delay }}
    />
  )
}
