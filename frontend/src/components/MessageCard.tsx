import { motion } from 'motion/react'
import ReactMarkdown from 'react-markdown'
import { softenMarkdown } from '../lib/softenMarkdown'

export function MessageCard({ role, content, label, pending, revealing }:
  { role: 'user' | 'assistant' | 'system'; content: string; label: string; pending?: boolean; revealing?: boolean }) {
  const isUser = role === 'user'
  const isSystem = role === 'system'
  return (
    <motion.div
      initial={{ opacity: 0, y: 8, filter: 'blur(4px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={{ type: 'spring', duration: 0.35, bounce: 0 }}
      className={`flex flex-col ${isUser ? 'items-end' : 'items-start'}`}
    >
      <span
        className={`mb-1 flex items-center gap-1 px-1 text-[11px] font-semibold tracking-wide ${
          isSystem ? 'text-accent' : 'text-muted'
        }`}
      >
        {isSystem && <InfoIcon />}
        {label}
      </span>
      <div
        className={
          `rounded-card px-3 py-2 font-mono text-sm leading-relaxed shadow-[var(--shadow-border)] ` +
          `[&_p]:[text-wrap:pretty] [&_*]:whitespace-pre-wrap ${
            isSystem
              ? 'w-full max-w-full rounded-tl-sm bg-surface text-fg [&_strong]:text-accent'
              : isUser
                ? 'max-w-[78%] rounded-tr-sm bg-accent text-white'
                : 'max-w-[78%] rounded-tl-sm bg-bubble text-fg'
          } ${revealing ? 'reveal-mask' : ''}`
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
          <ReactMarkdown>{revealing ? softenMarkdown(content) : content}</ReactMarkdown>
        )}
      </div>
    </motion.div>
  )
}

function InfoIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" className="size-3">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v5M12 8h.01" />
    </svg>
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
