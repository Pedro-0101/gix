import type { Command } from '../types'

export const debugCommand: Command = {
  name: 'debug',
  descriptionKey: 'cmd_debug_desc',
  run(ctx) {
    ctx.setView('debug')
  },
}
