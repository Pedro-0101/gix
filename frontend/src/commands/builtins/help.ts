import { tr } from '../../i18n'
import type { Command } from '../types'

// Pure: builds the /help markdown from a command list. Kept separate from the
// command object so it can be unit-tested without a CommandContext.
export function formatHelp(commands: Command[], lang: string): string {
  const lines = commands
    .filter((c) => !c.hidden)
    .map((c) => {
      const names = [c.name, ...(c.aliases ?? [])].map((n) => `/${n}`).join(' · ')
      return `**${names}** — ${tr(lang, c.descriptionKey)}`
    })
  return `${tr(lang, 'cmd_help_title')}\n\n${lines.join('\n')}`
}

export const helpCommand: Command = {
  name: 'help',
  aliases: ['ajuda'],
  descriptionKey: 'cmd_help_desc',
  run: (ctx) => ctx.emitSystemMessage(formatHelp(ctx.getCommands(), ctx.lang)),
}
