import { tr } from '../../i18n'
import type { Command } from '../types'

// /tidy <termo>: encontra a nota mais relevante para o termo, pede à IA para
// reorganizá-la (sem resumir) e, se o usuário confirmar, substitui o conteúdo
// pela versão organizada — com a opção de desfazer logo em seguida. Sem
// argumento, abre o navegador de notas. Irmão do /resumir, mas preserva tudo.
export const tidyCommand: Command = {
  name: 'tidy',
  aliases: ['organizar', 'arrumar'],
  descriptionKey: 'cmd_tidy_desc',
  acceptsArgs: true,
  run: async (ctx, arg) => {
    const query = (arg ?? '').trim()
    if (!query) {
      ctx.setView('notes')
      return
    }

    const hits = await ctx.notes.find(query)
    const hit = hits[0]
    if (!hit) {
      ctx.emitSystemMessage(tr(ctx.lang, 'tidy_empty'))
      return
    }

    const res = await ctx.notes.tidy(hit.noteId)
    if (res.status === 'no_api_key') {
      ctx.emitSystemMessage(tr(ctx.lang, 'no_api_key'))
      return
    }
    if (res.status !== 'ok' || !res.content) {
      ctx.emitSystemMessage(tr(ctx.lang, 'tidy_error'))
      return
    }

    ctx.emitSystemMessage(`**${hit.title}**\n\n${res.content}`)

    const apply = await ctx.choose({
      title: tr(ctx.lang, 'tidy_replace_confirm'),
      choices: [
        { label: tr(ctx.lang, 'tidy_replace'), value: 'yes' },
        { label: tr(ctx.lang, 'cancel'), value: 'no' },
      ],
    })
    if (apply !== 'yes') return

    await ctx.notes.update(hit.noteId, hit.title, res.content, hit.tags)
    ctx.emitSystemMessage(`${tr(ctx.lang, 'note_tidied')} **${hit.title}**`)

    const undo = await ctx.choose({
      title: tr(ctx.lang, 'tidy_undo_confirm'),
      choices: [
        { label: tr(ctx.lang, 'undo'), value: 'undo' },
        { label: tr(ctx.lang, 'keep'), value: 'keep' },
      ],
    })
    if (undo === 'undo') {
      await ctx.notes.update(hit.noteId, hit.title, hit.content, hit.tags)
      ctx.emitSystemMessage(tr(ctx.lang, 'tidy_undone'))
    }
  },
}
