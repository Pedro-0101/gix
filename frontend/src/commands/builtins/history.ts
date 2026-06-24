import type { Command } from '../types'

export const historyCommand: Command = {
  name: 'history',
  aliases: ['historico'],
  descriptionKey: 'cmd_history_desc',
  run: (ctx) => ctx.setView('history'),
}
