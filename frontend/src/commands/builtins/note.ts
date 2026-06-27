import { tr } from '../../i18n'
import { recurrenceLabel, formatFireAt } from '../../lib/alerts'
import type { Command } from '../types'

// /note <texto>: captura rápida. O backend formata o texto, extrai tags e salva
// uma nota atômica indexada para busca. Sem argumento, abre o navegador de notas.
export const noteCommand: Command = {
  name: 'note',
  aliases: ['nota', 'n'],
  descriptionKey: 'cmd_note_desc',
  acceptsArgs: true,
  run: async (ctx, arg) => {
    const text = (arg ?? '').trim()
    if (!text) {
      ctx.setView('notes')
      return
    }

    const res = await ctx.notes.capture(text)
    switch (res.status) {
      case 'created': {
        const tags = res.tags?.length ? `  _${res.tags.map((t) => '#' + t).join(' ')}_` : ''
        ctx.emitSystemMessage(`${tr(ctx.lang, 'note_created')} **${res.noteTitle}**${tags}`)
        if (res.alert) {
          const when = formatFireAt(res.alert.fireAt, ctx.lang)
          const rec = recurrenceLabel(ctx.lang, res.alert.recurrence)
          const suffix = [when, rec].filter(Boolean).join(' · ')
          const ok = await ctx.choose({
            title: `${tr(ctx.lang, 'alert_confirm')} ${suffix}`,
            choices: [
              { label: tr(ctx.lang, 'alert_yes'), value: 'yes' },
              { label: tr(ctx.lang, 'alert_no'), value: 'no' },
            ],
          })
          if (ok === 'yes') {
            const ar = await ctx.alerts.createProposed({
              message: res.alert.message, fireAt: res.alert.fireAt, recurrence: res.alert.recurrence, noteId: res.noteId,
            })
            if (ar.status === 'created') {
              const w = formatFireAt(ar.fireAtLocal, ctx.lang)
              const r = recurrenceLabel(ctx.lang, ar.recurrence)
              const sfx = [w, r].filter(Boolean).join(' · ')
              ctx.emitSystemMessage(`${tr(ctx.lang, 'alert_created')} **${ar.message}**${sfx ? `  _${sfx}_` : ''}`)
            }
          }
        }
        return
      }
      case 'no_api_key':
        ctx.emitSystemMessage(tr(ctx.lang, 'no_api_key'))
        return
      default:
        ctx.emitSystemMessage(tr(ctx.lang, 'note_error') + res.message)
    }
  },
}
