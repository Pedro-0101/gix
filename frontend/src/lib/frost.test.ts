import { describe, it, expect } from 'vitest'
import { frostColor } from './frost'

describe('frostColor', () => {
  it('lifts toward white in both themes (never a black overlay)', () => {
    expect(frostColor('light', 100)).toMatch(/^rgba\(255, 255, 255,/)
    expect(frostColor('dark', 100)).toMatch(/^rgba\(255, 255, 255,/)
  })

  it('scales alpha with opacity, brighter on light than dark', () => {
    expect(frostColor('light', 100)).toBe('rgba(255, 255, 255, 0.55)')
    expect(frostColor('dark', 100)).toBe('rgba(255, 255, 255, 0.18)')
    expect(frostColor('light', 50)).toBe('rgba(255, 255, 255, 0.275)')
  })

  it('is fully transparent at opacity 0', () => {
    expect(frostColor('light', 0)).toBe('rgba(255, 255, 255, 0)')
    expect(frostColor('dark', 0)).toBe('rgba(255, 255, 255, 0)')
  })

  it('clamps out-of-range opacity', () => {
    expect(frostColor('light', 150)).toBe(frostColor('light', 100))
    expect(frostColor('dark', -20)).toBe(frostColor('dark', 0))
  })

  it('treats any non-dark theme as light', () => {
    expect(frostColor('light', 80)).toBe(frostColor('whatever', 80))
  })
})
