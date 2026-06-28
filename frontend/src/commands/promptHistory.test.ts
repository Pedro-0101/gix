import { describe, it, expect } from 'vitest'
import { emptyHistory, record, prev, next, detach, HISTORY_CAP } from './promptHistory'

describe('promptHistory', () => {
  it('records submitted prompts oldest → newest', () => {
    let h = emptyHistory()
    h = record(h, 'one')
    h = record(h, 'two')
    expect(h.entries).toEqual(['one', 'two'])
  })

  it('ignores blank submissions and trims entries', () => {
    let h = record(emptyHistory(), '   ')
    expect(h.entries).toEqual([])
    h = record(h, '  hi  ')
    expect(h.entries).toEqual(['hi'])
  })

  it('collapses immediate duplicates', () => {
    let h = emptyHistory()
    h = record(h, 'same')
    h = record(h, 'same')
    expect(h.entries).toEqual(['same'])
  })

  it('caps the number of stored entries', () => {
    let h = emptyHistory()
    for (let i = 0; i < HISTORY_CAP + 10; i++) h = record(h, `p${i}`)
    expect(h.entries).toHaveLength(HISTORY_CAP)
    expect(h.entries[0]).toBe('p10')
  })

  it('walks backward through entries with prev, stashing the draft', () => {
    const h = record(record(emptyHistory(), 'a'), 'b')
    let r = prev(h, 'draft')
    expect(r.handled).toBe(true)
    expect(r.value).toBe('b')
    expect(r.history.draft).toBe('draft')
    r = prev(r.history, 'b')
    expect(r.value).toBe('a')
    // Stops at the oldest entry.
    r = prev(r.history, 'a')
    expect(r.value).toBe('a')
  })

  it('reports nothing to recall on an empty history', () => {
    const r = prev(emptyHistory(), 'typing')
    expect(r.handled).toBe(false)
    expect(r.value).toBe('typing')
  })

  it('walks forward with next and restores the draft past the newest', () => {
    const h = record(record(emptyHistory(), 'a'), 'b')
    let up = prev(h, 'draft') // -> 'b'
    up = prev(up.history, 'b') // -> 'a'
    let down = next(up.history) // -> 'b'
    expect(down.value).toBe('b')
    down = next(down.history) // -> draft
    expect(down.value).toBe('draft')
    expect(down.history.index).toBeNull()
  })

  it('does nothing on next when not browsing', () => {
    const r = next(record(emptyHistory(), 'a'))
    expect(r.handled).toBe(false)
  })

  it('detach ends browsing so prev restarts from the newest', () => {
    const h = record(record(emptyHistory(), 'a'), 'b')
    const up = prev(h, '') // index at 'b'
    const d = detach(up.history)
    expect(d.index).toBeNull()
    expect(prev(d, '').value).toBe('b')
  })

  it('seeds entries from a saved list', () => {
    const h = emptyHistory(['x', 'y'])
    expect(prev(h, '').value).toBe('y')
  })
})
