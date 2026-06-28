import { tr } from '../../i18n'
import { recurrenceLabel, formatFireAt } from '../../lib/alerts'
import type { Command, CommandContext, CaptureResult, CreateAlertResult } from '../types'

// /note <texto>: captura rápida. O backend formata o texto, extrai tags e decide
// (roteamento semântico) se a nota complementa uma existente — quando complementa,
// pede confirmação para anexar em vez de criar. Sem argumento, abre o navegador.
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
      case 'created':
        await settleNote(ctx, text, res.noteId, res.noteTitle, res.tags, false, res.alert)
        return
      case 'attach_proposed':
        await routeAttach(ctx, text, res)
        return
      case 'no_api_key':
        ctx.emitSystemMessage(tr(ctx.lang, 'no_api_key'))
        return
      default:
        ctx.emitSystemMessage(tr(ctx.lang, 'note_error') + res.message)
    }
  },
}

// routeAttach asks the user whether to append the capture to the note the AI
// picked or create a new one, then settles whichever they chose.
async function routeAttach(ctx: CommandContext, text: string, res: CaptureResult) {
  const target = res.attach!
  const choice = await ctx.choose({
    title: `${tr(ctx.lang, 'note_attach_q')} ${target.targetTitle}`,
    choices: [
      { label: tr(ctx.lang, 'note_attach_yes'), value: 'attach' },
      { label: tr(ctx.lang, 'note_attach_no'), value: 'create' },
    ],
  })
  if (choice === null) return // cancelled with Esc

  const attach = choice === 'attach'
  const final = attach
    ? await ctx.notes.appendTo(target.targetId, res.content, res.tags)
    : await ctx.notes.createFromProposal(res.noteTitle, res.content, res.tags)
  if (final.status !== 'attached' && final.status !== 'created') {
    ctx.emitSystemMessage(tr(ctx.lang, 'note_error') + final.message)
    return
  }
  await settleNote(ctx, text, final.noteId, final.noteTitle, final.tags, attach, res.alert)
}

// settleNote posts the created/attached confirmation and then runs the alert
// flow (a capture can carry a reminder regardless of which note it landed in).
async function settleNote(
  ctx: CommandContext, text: string, noteId: number, title: string,
  tags: string[], attached: boolean, alert: CaptureResult['alert'],
) {
  const tagStr = tags?.length ? `  _${tags.map((t) => '#' + t).join(' ')}_` : ''
  const key = attached ? 'note_attached' : 'note_created'
  ctx.emitSystemMessage(`${tr(ctx.lang, key)} **${title}**${tagStr}`)
  await maybeAlert(ctx, text, noteId, alert)
}

// maybeAlert proposes the alert the capture detected (confirmed by the user), or
// falls back to an explicit "crie um alerta" instruction in the raw text.
async function maybeAlert(ctx: CommandContext, text: string, noteId: number, alert: CaptureResult['alert']) {
  if (alert) {
    const suffix = [formatFireAt(alert.fireAt, ctx.lang), recurrenceLabel(ctx.lang, alert.recurrence)]
      .filter(Boolean).join(' · ')
    const ok = await ctx.choose({
      title: `${tr(ctx.lang, 'alert_confirm')} ${suffix}`,
      choices: [
        { label: tr(ctx.lang, 'alert_yes'), value: 'yes' },
        { label: tr(ctx.lang, 'alert_no'), value: 'no' },
      ],
    })
    if (ok === 'yes') {
      emitAlertCreated(ctx, await ctx.alerts.createProposed({
        message: alert.message, fireAt: alert.fireAt, recurrence: alert.recurrence, noteId,
      }))
    }
    return
  }
  if (/crie\s+um\s+(alerta|lembrete|alarme)/i.test(text)) {
    emitAlertCreated(ctx, await ctx.alerts.create(text))
  }
}

// emitAlertCreated posts the confirmation line for a created alert.
function emitAlertCreated(ctx: CommandContext, ar: CreateAlertResult) {
  if (ar.status !== 'created') return
  const sfx = [formatFireAt(ar.fireAtLocal, ctx.lang), recurrenceLabel(ctx.lang, ar.recurrence)]
    .filter(Boolean).join(' · ')
  ctx.emitSystemMessage(`${tr(ctx.lang, 'alert_created')} **${ar.message}**${sfx ? `  _${sfx}_` : ''}`)
}
