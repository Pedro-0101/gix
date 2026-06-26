import { tr } from '../../i18n'
import type { Command } from '../types'

// /note <texto>: captura rápida. O backend roteia (anexar/criar) e devolve um
// status; quando a nota alvo está cheia, oferecemos as estratégias de overflow
// via card de escolha e delegamos a resolução de volta ao backend.
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

    const res = await ctx.notes.route(text)
    switch (res.status) {
      case 'created':
        ctx.emitSystemMessage(`${tr(ctx.lang, 'note_created')} **${res.noteTitle}**`)
        return
      case 'appended':
        ctx.emitSystemMessage(`${tr(ctx.lang, 'note_appended')} **${res.noteTitle}**`)
        return
      case 'no_api_key':
        ctx.emitSystemMessage(tr(ctx.lang, 'no_api_key'))
        return
      case 'full':
        await resolveFull(ctx, res.noteId, res.noteTitle, text)
        return
      default:
        ctx.emitSystemMessage(tr(ctx.lang, 'note_error') + res.message)
    }
  },
}

async function resolveFull(
  ctx: Parameters<Command['run']>[0],
  noteId: number,
  noteTitle: string,
  text: string,
) {
  const strategy = await ctx.choose({
    title: tr(ctx.lang, 'note_full_title'),
    choices: [
      { label: tr(ctx.lang, 'note_opt_summarize'), value: 'summarize' },
      { label: tr(ctx.lang, 'note_opt_part2'), value: 'part2' },
      { label: tr(ctx.lang, 'note_opt_split'), value: 'split' },
    ],
  })
  if (!strategy) {
    ctx.emitSystemMessage(tr(ctx.lang, 'note_cancelled'))
    return
  }

  const res = await ctx.notes.resolveOverflow(noteId, text, strategy)
  if (res.status === 'error') {
    ctx.emitSystemMessage(tr(ctx.lang, 'note_error') + res.message)
    return
  }
  ctx.emitSystemMessage(`${tr(ctx.lang, 'note_appended')} **${res.noteTitle || noteTitle}**`)
}
