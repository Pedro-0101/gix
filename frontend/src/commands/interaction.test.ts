import { describe, it, expect } from 'vitest'
import { moveSelection } from './interaction'

describe('moveSelection', () => {
  it('moves down within bounds', () => {
    expect(moveSelection(4, 0, 1)).toBe(1)
    expect(moveSelection(4, 2, 1)).toBe(3)
  })

  it('moves up within bounds', () => {
    expect(moveSelection(4, 2, -1)).toBe(1)
  })

  it('wraps around the bottom edge', () => {
    expect(moveSelection(4, 3, 1)).toBe(0)
  })

  it('wraps around the top edge', () => {
    expect(moveSelection(4, 0, -1)).toBe(3)
  })

  it('returns current for an empty list', () => {
    expect(moveSelection(0, 0, 1)).toBe(0)
    expect(moveSelection(0, 0, -1)).toBe(0)
  })

  it('stays put on a single-item list', () => {
    expect(moveSelection(1, 0, 1)).toBe(0)
    expect(moveSelection(1, 0, -1)).toBe(0)
  })
})
