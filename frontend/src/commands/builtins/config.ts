import type { Command } from '../types'

export const configCommand: Command = {
  name: 'config',
  aliases: ['configuracoes', 'settings'],
  descriptionKey: 'cmd_config_desc',
  run: (ctx) => ctx.setView('settings'),
}
