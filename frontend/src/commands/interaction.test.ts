import { describe, it, expect } from 'vitest'
import { moveSelection, moveSlider } from './interaction'

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

describe('moveSlider', () => {
  it('steps right and left within bounds', () => {
    expect(moveSlider(50, 1, 5, 0, 100)).toBe(55)
    expect(moveSlider(50, -1, 5, 0, 100)).toBe(45)
  })

  it('clamps at the maximum without wrapping', () => {
    expect(moveSlider(98, 1, 5, 0, 100)).toBe(100)
    expect(moveSlider(100, 1, 5, 0, 100)).toBe(100)
  })

  it('clamps at the minimum without wrapping', () => {
    expect(moveSlider(2, -1, 5, 0, 100)).toBe(0)
    expect(moveSlider(0, -1, 5, 0, 100)).toBe(0)
  })

  it('honours a non-zero minimum (e.g. interval fields)', () => {
    expect(moveSlider(150, -1, 50, 100, 2000)).toBe(100)
    expect(moveSlider(120, -1, 50, 100, 2000)).toBe(100)
  })
})
