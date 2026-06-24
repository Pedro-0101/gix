import { describe, it, expect } from 'vitest'
import { formatHelp } from './help'
import type { Command } from '../types'

const list: Command[] = [
  { name: 'help', aliases: ['ajuda'], descriptionKey: 'cmd_help_desc', run: () => {} },
  { name: 'new', aliases: ['novo'], descriptionKey: 'cmd_new_desc', run: () => {} },
  { name: 'secret', descriptionKey: 'cmd_help_desc', hidden: true, run: () => {} },
]

describe('formatHelp', () => {
  it('opens with the localized title', () => {
    expect(formatHelp(list, 'pt')).toMatch(/^Comandos disponíveis:/)
    expect(formatHelp(list, 'en')).toMatch(/^Available commands:/)
  })

  it('lists each visible command with its slash names joined', () => {
    const out = formatHelp(list, 'pt')
    expect(out).toContain('**/help · /ajuda** — Lista os comandos disponíveis')
    expect(out).toContain('**/new · /novo** — Inicia uma nova conversa')
  })

  it('omits hidden commands', () => {
    expect(formatHelp(list, 'pt')).not.toContain('/secret')
  })

  it('localizes descriptions per language', () => {
    expect(formatHelp(list, 'en')).toContain('— Lists the available commands')
  })

  it('renders a command with no aliases as just its name', () => {
    const single: Command[] = [{ name: 'config', descriptionKey: 'cmd_config_desc', run: () => {} }]
    expect(formatHelp(single, 'pt')).toContain('**/config** — Altera as configurações')
  })
})
