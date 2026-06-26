import { tr } from '../../i18n'
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
