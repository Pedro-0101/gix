import { useEffect, useRef, type Dispatch, type SetStateAction } from 'react'
import { onAlertProposed, onNoteProposed } from './events'
import { recurrenceLabel, formatFireAt } from './alerts'
import { tr } from '../i18n'
import type { CommandContext, Msg } from '../types'

type Deps = {
  setStreaming: (v: boolean) => void
  setMsgs: Dispatch<SetStateAction<Msg[]>>
  commandContextRef: { current: CommandContext }
}

export function useProposals({ setStreaming, setMsgs, commandContextRef }: Deps) {
  const deps = useRef({ setStreaming, setMsgs, commandContextRef })
  deps.current = { setStreaming, setMsgs, commandContextRef }

  useEffect(() => {
    const { setStreaming, setMsgs, commandContextRef } = deps.current
    const offNote = onNoteProposed(async (p) => {
      setStreaming(false)
      setMsgs((m) => {
        const last = m[m.length - 1]
        return last && last.role === 'assistant' && !last.content ? m.slice(0, -1) : m
      })
      const ctx = commandContextRef.current
      const tags = p.tags.length ? `  _#${p.tags.join(' #')}_` : ''
      const ok = await ctx.choose({
        title: `${tr(ctx.lang, 'note_confirm')} **${p.title}**${tags}`,
        choices: [
          { label: tr(ctx.lang, 'alert_yes'), value: 'yes' },
          { label: tr(ctx.lang, 'alert_no'), value: 'no' },
        ],
      })
      if (ok !== 'yes') return
      const res = await ctx.notes.createFromProposal(p.title, p.content, p.tags)
      if (res.status === 'created') {
        ctx.emitSystemMessage(`${tr(ctx.lang, 'note_created')} **${res.noteTitle}**${tags}`)
      }
    })
    const offAlert = onAlertProposed(async (p) => {
      setStreaming(false)
      setMsgs((m) => {
        const last = m[m.length - 1]
        return last && last.role === 'assistant' && !last.content ? m.slice(0, -1) : m
      })
      const ctx = commandContextRef.current
      const when = formatFireAt(p.fireAt, ctx.lang)
      const rec = recurrenceLabel(ctx.lang, p.recurrence)
      const suffix = [when, rec].filter(Boolean).join(' · ')
      const ok = await ctx.choose({
        title: `${tr(ctx.lang, 'alert_confirm')} ${suffix}`,
        choices: [
          { label: tr(ctx.lang, 'alert_yes'), value: 'yes' },
          { label: tr(ctx.lang, 'alert_no'), value: 'no' },
        ],
      })
      if (ok !== 'yes') return
      const res = await ctx.alerts.createProposed({ message: p.message, fireAt: p.fireAt, recurrence: p.recurrence })
      if (res.status === 'created') {
        const w = formatFireAt(res.fireAtLocal, ctx.lang)
        const r = recurrenceLabel(ctx.lang, res.recurrence)
        const sfx = [w, r].filter(Boolean).join(' · ')
        ctx.emitSystemMessage(`${tr(ctx.lang, 'alert_created')} **${res.message}**${sfx ? `  _${sfx}_` : ''}`)
      }
    })
    return () => { offNote(); offAlert() }
  }, [])
}
