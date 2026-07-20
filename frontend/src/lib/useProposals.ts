import { useEffect, useRef, type Dispatch, type SetStateAction } from 'react'
import { onAlertProposed, onNoteProposed, onChatDelta, onChatDone } from './events'
import { recurrenceLabel, formatFireAt } from './alerts'
import { tr } from '../i18n'
import type { CommandContext } from '../commands/types'
import type { Msg } from '../types'

type Deps = {
  setStreaming: (v: boolean) => void
  setMsgs: Dispatch<SetStateAction<Msg[]>>
  commandContextRef: { current: CommandContext }
}

type Proposal =
  | { type: 'note'; title: string; content: string; tags: string[] }
  | { type: 'alert'; message: string; fireAt: string; recurrence: string }

export function useProposals({ setStreaming, setMsgs, commandContextRef }: Deps) {
  const deps = useRef({ setStreaming, setMsgs, commandContextRef })
  deps.current = { setStreaming, setMsgs, commandContextRef }

  const bufferRef = useRef<Proposal[]>([])
  const hadDeltaRef = useRef(false)

  useEffect(() => {
    bufferRef.current = []
    hadDeltaRef.current = false

    const offDelta = onChatDelta(() => {
      hadDeltaRef.current = true
    })

    const offNote = onNoteProposed((p) => {
      bufferRef.current.push({ type: 'note', title: p.title, content: p.content, tags: p.tags })
    })

    const offAlert = onAlertProposed((p) => {
      bufferRef.current.push({ type: 'alert', message: p.message, fireAt: p.fireAt, recurrence: p.recurrence })
    })

    const offDone = onChatDone(async () => {
      const buf = bufferRef.current
      bufferRef.current = []
      if (buf.length === 0) return

      const { setStreaming, setMsgs, commandContextRef } = deps.current
      setStreaming(false)

      // Remove the empty assistant message when the AI wrote no text
      // (only tool calls). By now useChat's done handler already set
      // "(ações executadas)" on it, so we check hadDeltaRef instead.
      if (!hadDeltaRef.current) {
        setMsgs((m) => {
          const copy = [...m]
          const last = copy[copy.length - 1]
          if (last && last.role === 'assistant') copy.pop()
          return copy
        })
      }
      hadDeltaRef.current = false

      const ctx = commandContextRef.current

      for (const proposal of buf) {
        if (proposal.type === 'note') {
          const tags = proposal.tags.length ? `  _#${proposal.tags.join(' #')}_` : ''
          const ok = await ctx.choose({
            title: `${tr(ctx.lang, 'note_confirm')} **${proposal.title}**${tags}`,
            choices: [
              { label: tr(ctx.lang, 'alert_yes'), value: 'yes' },
              { label: tr(ctx.lang, 'alert_no'), value: 'no' },
            ],
          })
          if (ok !== 'yes') continue
          const res = await ctx.notes.createFromProposal(proposal.title, proposal.content, proposal.tags)
          if (res.status === 'created') {
            ctx.emitSystemMessage(`${tr(ctx.lang, 'note_created')} **${res.noteTitle}**${tags}`)
          }
        } else {
          const when = formatFireAt(proposal.fireAt, ctx.lang)
          const rec = recurrenceLabel(ctx.lang, proposal.recurrence)
          const suffix = [when, rec].filter(Boolean).join(' · ')
          const ok = await ctx.choose({
            title: `${tr(ctx.lang, 'alert_confirm')} ${suffix}`,
            choices: [
              { label: tr(ctx.lang, 'alert_yes'), value: 'yes' },
              { label: tr(ctx.lang, 'alert_no'), value: 'no' },
            ],
          })
          if (ok !== 'yes') continue
          try {
            const res = await ctx.alerts.createProposed({
              message: proposal.message,
              fireAt: proposal.fireAt,
              recurrence: proposal.recurrence,
            })
            if (res.status === 'created') {
              const w = formatFireAt(res.fireAtLocal, ctx.lang)
              const r = recurrenceLabel(ctx.lang, res.recurrence)
              const sfx = [w, r].filter(Boolean).join(' · ')
              ctx.emitSystemMessage(`${tr(ctx.lang, 'alert_created')} **${res.message}**${sfx ? `  _${sfx}_` : ''}`)
            }
          } catch {
            ctx.emitSystemMessage(tr(ctx.lang, 'alert_error'))
          }
        }
      }
    })

    return () => { offDelta(); offNote(); offAlert(); offDone() }
  }, [])
}
