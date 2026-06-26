import { describe, it, expect } from 'vitest'
import { recurrenceLabel, formatFireAt } from './alerts'

describe('recurrenceLabel', () => {
  it('returns empty for one-shot', () => {
    expect(recurrenceLabel('pt', '')).toBe('')
    expect(recurrenceLabel('pt', 'garbage')).toBe('')
  })

  it('labels a simple daily rule', () => {
    expect(recurrenceLabel('en', '{"freq":"daily","interval":1}')).toBe('daily')
    expect(recurrenceLabel('pt', '{"freq":"daily","interval":1}')).toBe('diariamente')
  })

  it('labels an interval > 1 with the "every N" form', () => {
    expect(recurrenceLabel('en', '{"freq":"weekly","interval":2}')).toBe('every 2 weekly')
  })
})

describe('formatFireAt', () => {
  it('produces a non-empty local string for a valid ISO', () => {
    expect(formatFireAt('2026-06-26T12:00:00Z', 'pt').length).toBeGreaterThan(0)
  })
  it('returns empty for an invalid ISO', () => {
    expect(formatFireAt('not-a-date', 'pt')).toBe('')
  })
})
