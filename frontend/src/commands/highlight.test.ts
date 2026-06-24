import { describe, it, expect } from 'vitest'
import { analyzeBar } from './highlight'
import type { Command } from './types'

const list: Command[] = [
  { name: 'help', aliases: ['ajuda'], descriptionKey: 'd', run: () => {} },
  { name: 'config', aliases: ['settings'], descriptionKey: 'd', run: () => {} },
]

describe('analyzeBar', () => {
  it('completes a partial command to its full name', () => {
    expect(analyzeBar('/he', list)).toEqual({
      isCommand: true,
      completion: 'lp',
      accepted: '/help',
    })
  })

  it('treats a fully typed command as a command with no completion', () => {
    expect(analyzeBar('/help', list)).toEqual({
      isCommand: true,
      completion: '',
      accepted: '/help',
    })
  })

  it('prefers canonical names over aliases when both match a prefix', () => {
    // 'config' (name) and 'settings' (alias) — 'config' wins for '/c'.
    expect(analyzeBar('/c', list).accepted).toBe('/config')
  })

  it('completes via alias when only the alias matches', () => {
    expect(analyzeBar('/set', list).accepted).toBe('/settings')
  })

  it('is case-insensitive on the typed key', () => {
    expect(analyzeBar('/HE', list).accepted).toBe('/help')
  })

  it('is not a command without a leading slash', () => {
    expect(analyzeBar('he', list)).toEqual({ isCommand: false, completion: '', accepted: 'he' })
  })

  it('is not a command once whitespace is typed', () => {
    expect(analyzeBar('/help ', list).isCommand).toBe(false)
  })

  it('is not a command for a lone slash', () => {
    expect(analyzeBar('/', list).isCommand).toBe(false)
  })

  it('is not a command for an unknown prefix', () => {
    expect(analyzeBar('/xyz', list).isCommand).toBe(false)
  })
})
