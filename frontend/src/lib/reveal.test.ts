import { describe, it, expect } from 'vitest'
import { nextReveal, BASE_CPS, TAU } from './reveal'

describe('nextReveal', () => {
  it('avança no piso BASE_CPS quando a velocidade já está em regime e o backlog é pequeno', () => {
    // backlog 5 → catch-up 5/0.4 = 12.5 c/s < piso 80 c/s; vel já no piso.
    const r = nextReveal(0, BASE_CPS, 5, 0.016, false)
    expect(r.vel).toBeCloseTo(BASE_CPS, 5)
    expect(r.shown).toBeCloseTo(BASE_CPS * 0.016, 5)
  })

  it('amortece o burst: acelera, mas sem saltar para a velocidade-alvo', () => {
    // backlog 1000 → velocidade-alvo 1000/0.4 = 2500 c/s; partindo do piso a
    // velocidade exibida sobe em direção a ela, mas o filtro a segura.
    const r = nextReveal(0, BASE_CPS, 1000, 0.016, false)
    expect(r.vel).toBeGreaterThan(BASE_CPS)       // acelerou
    expect(r.vel).toBeLessThan(1000 / TAU)        // mas amortecido, não pulou pro alvo
  })

  it('a velocidade converge ao alvo ao longo de vários frames', () => {
    const v1 = nextReveal(0, BASE_CPS, 1000, 0.016, false).vel
    const v2 = nextReveal(0, v1, 1000, 0.016, false).vel
    expect(v2).toBeGreaterThan(v1)               // monotônico rumo ao alvo
    expect(v2).toBeLessThan(1000 / TAU)
  })

  it('faz snap para targetLen quando done e o backlog é < 1', () => {
    const r = nextReveal(999.8, BASE_CPS, 1000, 0.001, true)
    expect(r.shown).toBe(1000)
  })

  it('não faz snap se ainda não terminou (done=false), mesmo com backlog < 1', () => {
    const r = nextReveal(999.8, BASE_CPS, 1000, 0.001, false)
    expect(r.shown).toBeGreaterThan(999.8)
    expect(r.shown).toBeLessThan(1000)
  })

  it('nunca ultrapassa targetLen', () => {
    expect(nextReveal(995, BASE_CPS, 1000, 1, false).shown).toBe(1000)
  })

  it('retorna targetLen se já está em dia', () => {
    expect(nextReveal(1000, BASE_CPS, 1000, 0.016, false).shown).toBe(1000)
  })
})
