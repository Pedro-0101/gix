import type { Command } from '../types'

export const graphCommand: Command = {
  name: 'graph',
  aliases: ['mapa', 'grafo'],
  descriptionKey: 'cmd_graph_desc',
  run(ctx) {
    ctx.setView('graph')
  },
}
