import { describe, it, expect, beforeEach } from 'vitest'
import { keyOf, markSurfaced, wasSurfaced, _resetSurfaced } from './alertSchedule'

describe('keyOf', () => {
  it('monta id:unixSeconds a partir de RFC3339', () => {
    expect(keyOf(7, '1970-01-01T00:00:10Z')).toBe('7:10')
  })
  it('é estável para o mesmo instante em offsets diferentes', () => {
    expect(keyOf(1, '2026-06-30T12:00:00Z')).toBe(keyOf(1, '2026-06-30T09:00:00-03:00'))
  })
})

describe('surfaced set', () => {
  beforeEach(() => _resetSurfaced())
  it('marca e consulta', () => {
    expect(wasSurfaced('1:10')).toBe(false)
    markSurfaced('1:10')
    expect(wasSurfaced('1:10')).toBe(true)
  })
})
