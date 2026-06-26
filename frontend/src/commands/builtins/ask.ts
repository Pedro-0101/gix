import type { Command } from '../types'

// /ask <pergunta>: busca as notas mais relevantes e pede um resumo à IA. Abre a
// view de busca em modo resumo (resposta no topo, notas-fonte abaixo).
export const askCommand: Command = {
  name: 'ask',
  aliases: ['perguntar'],
  descriptionKey: 'cmd_ask_desc',
  acceptsArgs: true,
  run: async (ctx, arg) => {
    const query = (arg ?? '').trim()
    if (!query) {
      ctx.setView('notes')
      return
    }
    ctx.openSearch({ query, mode: 'ask', loading: true, hits: [] })
    const res = await ctx.notes.ask(query)
    ctx.openSearch({
      query,
      mode: 'ask',
      loading: false,
      hits: res.sources ?? [],
      summary: res.summary,
      status: res.status,
    })
  },
}
