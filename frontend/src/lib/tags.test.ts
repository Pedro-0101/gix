import { describe, it, expect } from 'vitest'
import { normalizeTag, addTags } from './tags'

describe('normalizeTag', () => {
  it('trims, lowercases and strips a leading #', () => {
    expect(normalizeTag('  #Carro ')).toBe('carro')
    expect(normalizeTag('Manutenção')).toBe('manutenção')
  })
})

describe('addTags', () => {
  it('appends a normalized tag', () => {
    expect(addTags([], 'Carro')).toEqual(['carro'])
  })

  it('splits a comma-separated entry into several tags', () => {
    expect(addTags([], 'a, b ,c')).toEqual(['a', 'b', 'c'])
  })

  it('de-dupes against existing tags (case-insensitively)', () => {
    expect(addTags(['carro'], 'Carro')).toEqual(['carro'])
    expect(addTags(['a'], 'a, b, a')).toEqual(['a', 'b'])
  })

  it('ignores empty / whitespace-only entries', () => {
    expect(addTags(['a'], '   ')).toEqual(['a'])
    expect(addTags(['a'], ',, ,')).toEqual(['a'])
  })

  it('does not mutate the input array', () => {
    const existing = ['a']
    addTags(existing, 'b')
    expect(existing).toEqual(['a'])
  })
})
