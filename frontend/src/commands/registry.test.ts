import { describe, it, expect } from 'vitest'
import { resolveCommand } from './registry'
import type { Command } from './types'

// A small fixed list so tests don't depend on the live registry's contents.
const list: Command[] = [
  { name: 'help', aliases: ['ajuda'], descriptionKey: 'd', run: () => {} },
  { name: 'config', aliases: ['configuracoes', 'settings'], descriptionKey: 'd', run: () => {} },
  { name: 'new', descriptionKey: 'd', run: () => {} },
  { name: 'note', aliases: ['nota', 'n'], descriptionKey: 'd', acceptsArgs: true, run: () => {} },
]

describe('resolveCommand', () => {
  it('resolves a bare slash command by canonical name', () => {
    expect(resolveCommand('/help', list)?.cmd.name).toBe('help')
    expect(resolveCommand('/help', list)?.arg).toBe('')
  })

  it('resolves by alias', () => {
    expect(resolveCommand('/ajuda', list)?.cmd.name).toBe('help')
    expect(resolveCommand('/settings', list)?.cmd.name).toBe('config')
  })

  it('is case-insensitive', () => {
    expect(resolveCommand('/HELP', list)?.cmd.name).toBe('help')
    expect(resolveCommand('/Configuracoes', list)?.cmd.name).toBe('config')
  })

  it('trims surrounding whitespace before matching', () => {
    expect(resolveCommand('  /help  ', list)?.cmd.name).toBe('help')
  })

  it('returns null when a no-arg command is given inner whitespace (it is a message)', () => {
    expect(resolveCommand('/help me', list)).toBeNull()
  })

  it('passes the remaining text as arg for an acceptsArgs command', () => {
    const r = resolveCommand('/note comprar shampoo amanhã', list)
    expect(r?.cmd.name).toBe('note')
    expect(r?.arg).toBe('comprar shampoo amanhã')
  })

  it('resolves an acceptsArgs command by alias with arg', () => {
    expect(resolveCommand('/n algo', list)?.cmd.name).toBe('note')
    expect(resolveCommand('/nota algo', list)?.arg).toBe('algo')
  })

  it('allows a bare acceptsArgs command with empty arg', () => {
    const r = resolveCommand('/note', list)
    expect(r?.cmd.name).toBe('note')
    expect(r?.arg).toBe('')
  })

  it('collapses inner whitespace runs into the trimmed arg', () => {
    expect(resolveCommand('/note   muito    espaço  ', list)?.arg).toBe('muito    espaço')
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
