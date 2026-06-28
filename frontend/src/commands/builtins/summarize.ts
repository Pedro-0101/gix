import { tr } from '../../i18n'
import type { Command } from '../types'

// /resumir <termo>: encontra a nota mais relevante para o termo, pede um resumo à
// IA e, se o usuário confirmar, substitui o conteúdo da nota pelo resumo — com a
// opção de desfazer logo em seguida. Sem argumento, abre o navegador de notas.
export const summarizeCommand: Command = {
  name: 'summarize',
  aliases: ['resumir'],
  descriptionKey: 'cmd_summarize_desc',
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
      ctx.emitSystemMessage(tr(ctx.lang, 'summarize_empty'))
      return
    }

    const res = await ctx.notes.summarize(hit.noteId)
    if (res.status === 'no_api_key') {
      ctx.emitSystemMessage(tr(ctx.lang, 'no_api_key'))
      return
    }
    if (res.status !== 'ok' || !res.summary) {
      ctx.emitSystemMessage(tr(ctx.lang, 'summarize_error'))
      return
    }

    ctx.emitSystemMessage(`**${hit.title}**\n\n${res.summary}`)

    const apply = await ctx.choose({
      title: tr(ctx.lang, 'summarize_replace_confirm'),
      choices: [
        { label: tr(ctx.lang, 'summarize_replace'), value: 'yes' },
        { label: tr(ctx.lang, 'cancel'), value: 'no' },
      ],
    })
    if (apply !== 'yes') return

    await ctx.notes.update(hit.noteId, hit.title, res.summary, hit.tags)
    ctx.emitSystemMessage(`${tr(ctx.lang, 'note_summarized')} **${hit.title}**`)

    const undo = await ctx.choose({
      title: tr(ctx.lang, 'summarize_undo_confirm'),
      choices: [
        { label: tr(ctx.lang, 'undo'), value: 'undo' },
        { label: tr(ctx.lang, 'keep'), value: 'keep' },
      ],
    })
    if (undo === 'undo') {
      await ctx.notes.update(hit.noteId, hit.title, hit.content, hit.tags)
      ctx.emitSystemMessage(tr(ctx.lang, 'summarize_undone'))
    }
  },
}
