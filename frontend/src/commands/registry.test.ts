import { describe, it, expect } from 'vitest'
import { resolveCommand } from './registry'
import type { Command } from './types'

// A small fixed list so tests don't depend on the live registry's contents.
const list: Command[] = [
  { name: 'help', aliases: ['ajuda'], descriptionKey: 'd', run: () => {} },
  { name: 'config', aliases: ['configuracoes', 'settings'], descriptionKey: 'd', run: () => {} },
  { name: 'new', descriptionKey: 'd', run: () => {} },
]

describe('resolveCommand', () => {
  it('resolves a bare slash command by canonical name', () => {
    expect(resolveCommand('/help', list)?.name).toBe('help')
  })

  it('resolves by alias', () => {
    expect(resolveCommand('/ajuda', list)?.name).toBe('help')
    expect(resolveCommand('/settings', list)?.name).toBe('config')
  })

  it('is case-insensitive', () => {
    expect(resolveCommand('/HELP', list)?.name).toBe('help')
    expect(resolveCommand('/Configuracoes', list)?.name).toBe('config')
  })

  it('trims surrounding whitespace before matching', () => {
    expect(resolveCommand('  /help  ', list)?.name).toBe('help')
  })

  it('returns null when the text has inner whitespace (it is a message, not a command)', () => {
    expect(resolveCommand('/help me', list)).toBeNull()
  })

  it('returns null without a leading slash', () => {
    expect(resolveCommand('help', list)).toBeNull()
  })

  it('returns null for a lone slash', () => {
    expect(resolveCommand('/', list)).toBeNull()
  })

  it('returns null for an unknown command', () => {
    expect(resolveCommand('/nope', list)).toBeNull()
  })

  it('returns null for empty input', () => {
    expect(resolveCommand('', list)).toBeNull()
  })
})
