import { describe, it, expect } from 'vitest'
import { softenMarkdown } from './softenMarkdown'

describe('softenMarkdown', () => {
  it('esconde ** não-fechado', () => {
    expect(softenMarkdown('isto é **importante')).toBe('isto é importante')
  })

  it('mantém ** fechado intacto', () => {
    expect(softenMarkdown('isto é **importante**')).toBe('isto é **importante**')
  })

  it('esconde __ e ~~ e ` não-fechados', () => {
    expect(softenMarkdown('um __sub')).toBe('um sub')
    expect(softenMarkdown('um ~~ris')).toBe('um ris')
    expect(softenMarkdown('rode `cod')).toBe('rode cod')
  })

  it('mostra só o texto de um link parcial', () => {
    expect(softenMarkdown('veja [docs](http://exemplo')).toBe('veja docs')
  })

  it('mantém link completo intacto', () => {
    expect(softenMarkdown('veja [docs](http://exemplo)')).toBe('veja [docs](http://exemplo)')
  })

  it('deixa * e _ de um caractere e listas inalterados', () => {
    expect(softenMarkdown('* item de lista')).toBe('* item de lista')
    expect(softenMarkdown('a * b')).toBe('a * b')
  })

  it('não altera texto sem marcadores', () => {
    expect(softenMarkdown('texto simples e direto')).toBe('texto simples e direto')
  })
})
