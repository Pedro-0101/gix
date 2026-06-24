import { describe, it, expect } from 'vitest'
import { nextShown, BASE_CPS, TAU } from './reveal'

describe('nextShown', () => {
  it('aplica o piso BASE_CPS quando o backlog é pequeno', () => {
    // backlog 5 → catch-up 5/0.4 = 12.5 c/s < piso 80 c/s
    const next = nextShown(0, 5, 0.016, false)
    expect(next).toBeCloseTo(BASE_CPS * 0.016, 5)
  })

  it('acelera acima do piso quando o backlog é grande', () => {
    // backlog 1000 → catch-up 1000/0.4 = 2500 c/s ≫ piso
    const next = nextShown(0, 1000, 0.016, false)
    expect(next).toBeCloseTo((1000 / TAU) * 0.016, 5)
    expect(next).toBeGreaterThan(BASE_CPS * 0.016)
  })

  it('faz snap para targetLen quando done e o backlog é < 1', () => {
    const next = nextShown(999.8, 1000, 0.001, true)
    expect(next).toBe(1000)
  })

  it('não faz snap se ainda não terminou (done=false), mesmo com backlog < 1', () => {
    const next = nextShown(999.8, 1000, 0.001, false)
    expect(next).toBeGreaterThan(999.8)
    expect(next).toBeLessThan(1000)
  })

  it('nunca ultrapassa targetLen', () => {
    expect(nextShown(995, 1000, 1, false)).toBe(1000)
  })

  it('retorna targetLen se já está em dia', () => {
    expect(nextShown(1000, 1000, 0.016, false)).toBe(1000)
  })
})
