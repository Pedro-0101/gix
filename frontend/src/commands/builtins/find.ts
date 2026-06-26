import type { Command } from '../types'

// /find <consulta>: busca híbrida (full-text + semântica) nas notas, sem IA.
// Abre a view de busca com a lista ranqueada. Sem argumento, abre as notas.
export const findCommand: Command = {
  name: 'find',
  aliases: ['buscar', 'f'],
  descriptionKey: 'cmd_find_desc',
  acceptsArgs: true,
  run: async (ctx, arg) => {
    const query = (arg ?? '').trim()
    if (!query) {
      ctx.setView('notes')
      return
    }
    ctx.openSearch({ query, mode: 'find', loading: true, hits: [] })
    const hits = await ctx.notes.find(query)
    ctx.openSearch({ query, mode: 'find', loading: false, hits: hits ?? [] })
  },
}
