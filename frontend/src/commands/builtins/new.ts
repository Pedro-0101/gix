import type { Command } from '../types'

export const newCommand: Command = {
  name: 'new',
  aliases: ['limpar', 'novo'],
  descriptionKey: 'cmd_new_desc',
  run: (ctx) => ctx.newConversation(),
}
