import { motion } from 'motion/react'
import type { Choice } from '../commands/interaction'

// The active options card: a titled list where one row is highlighted. Keyboard
// (↑/↓/Enter) drives `selected` from App; hovering or clicking a row works too.
export function ChoiceCard({ title, choices, selected, onHover, onPick }: {
  title: string
  choices: Choice[]
  selected: number
  onHover: (i: number) => void
  onPick: (i: number) => void
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8, filter: 'blur(4px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={{ type: 'spring', duration: 0.35, bounce: 0 }}
      className="flex flex-col items-start"
    >
      <span className="mb-1 flex items-center gap-1 px-1 text-[11px] font-semibold tracking-wide text-accent">
        <ChevronIcon />
        {title}
      </span>
      <div className="w-full overflow-hidden rounded-card bg-surface py-1 shadow-[var(--shadow-border)]">
        {choices.map((c, i) => {
          const active = i === selected
          return (
            <button
              key={c.value}
              type="button"
              onMouseMove={() => onHover(i)}
              onClick={() => onPick(i)}
              className={
                'flex w-full items-center gap-2 px-3 py-1.5 text-left font-mono text-sm outline-none transition-colors duration-100 ' +
                (active ? 'bg-accent text-white' : 'text-fg hover:bg-bubble')
              }
            >
              <span className={`w-2 shrink-0 ${active ? 'opacity-100' : 'opacity-0'}`}>›</span>
              <span className="flex-1 truncate">{c.label}</span>
              {c.hint && <span className={active ? 'text-white/70' : 'text-muted'}>{c.hint}</span>}
            </button>
          )
        })}
      </div>
    </motion.div>
  )
}

// The inert record left in the conversation after a choice is made.
export function ChoiceSummary({ title, chosenLabel }: { title: string; chosenLabel: string }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8, filter: 'blur(4px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={{ type: 'spring', duration: 0.35, bounce: 0 }}
      className="flex items-center gap-2 px-1 font-mono text-xs text-muted"
    >
      <span className="text-accent"><ChevronIcon /></span>
      <span>{title}</span>
      <span className="opacity-40">›</span>
      <span className="font-semibold text-fg">{chosenLabel}</span>
    </motion.div>
  )
}

function ChevronIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4"
      strokeLinecap="round" strokeLinejoin="round" className="size-3">
      <path d="M9 7l5 5-5 5" />
    </svg>
  )
}
