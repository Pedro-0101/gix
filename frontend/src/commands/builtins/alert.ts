import { tr } from '../../i18n'
import { recurrenceLabel, formatFireAt } from '../../lib/alerts'
import type { Command } from '../types'

// /alerta <texto>: cria um lembrete. O backend chama a IA para entender a
// data/recorrência e salva o alerta. Sem argumento, abre a view de alertas.
export const alertCommand: Command = {
  name: 'alerta',
  aliases: ['alertas', 'alarme', 'lembrete', 'al'],
  descriptionKey: 'cmd_alert_desc',
  acceptsArgs: true,
  run: async (ctx, arg) => {
    const text = (arg ?? '').trim()
    if (!text) {
      ctx.setView('alerts')
      return
    }

    const res = await ctx.alerts.create(text)
    switch (res.status) {
      case 'created': {
        const when = formatFireAt(res.fireAtLocal, ctx.lang)
        const rec = recurrenceLabel(ctx.lang, res.recurrence)
        const suffix = [when, rec].filter(Boolean).join(' · ')
        ctx.emitSystemMessage(`${tr(ctx.lang, 'alert_created')} **${res.message}**${suffix ? `  _${suffix}_` : ''}`)
        return
      }
      case 'no_api_key':
        ctx.emitSystemMessage(tr(ctx.lang, 'alert_no_api_key'))
        return
      case 'unparseable':
        ctx.emitSystemMessage(tr(ctx.lang, 'alert_unparseable'))
        return
      case 'past':
        ctx.emitSystemMessage(tr(ctx.lang, 'alert_past'))
        return
      default:
        ctx.emitSystemMessage(tr(ctx.lang, 'alert_error'))
    }
  },
}
