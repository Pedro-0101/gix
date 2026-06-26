import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createDeferredDelete } from './deferredDelete'

describe('createDeferredDelete', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('commits the deletion only after the delay elapses', () => {
    const commit = vi.fn()
    const changes: (number | null)[] = []
    const dd = createDeferredDelete({ delayMs: 5000, commit, onChange: (id) => changes.push(id) })

    dd.schedule(7)
    expect(dd.pending()).toBe(7)
    expect(commit).not.toHaveBeenCalled()

    vi.advanceTimersByTime(4999)
    expect(commit).not.toHaveBeenCalled()

    vi.advanceTimersByTime(1)
    expect(commit).toHaveBeenCalledExactlyOnceWith(7)
    expect(dd.pending()).toBeNull()
    expect(changes).toEqual([7, null]) // toast shown, then hidden
  })

  it('undo cancels the deletion before it commits', () => {
    const commit = vi.fn()
    const dd = createDeferredDelete({ delayMs: 5000, commit })

    dd.schedule(7)
    dd.undo()
    expect(dd.pending()).toBeNull()

    vi.advanceTimersByTime(10000)
    expect(commit).not.toHaveBeenCalled()
  })

  it('undo after the deletion already committed is a no-op', () => {
    const commit = vi.fn()
    const dd = createDeferredDelete({ delayMs: 1000, commit })

    dd.schedule(7)
    vi.advanceTimersByTime(1000)
    expect(commit).toHaveBeenCalledTimes(1)

    dd.undo()
    vi.advanceTimersByTime(5000)
    expect(commit).toHaveBeenCalledTimes(1)
  })

  it('scheduling a new deletion flushes the previous one immediately', () => {
    const commit = vi.fn()
    const dd = createDeferredDelete({ delayMs: 5000, commit })

    dd.schedule(1)
    dd.schedule(2) // first must be committed now, second becomes pending
    expect(commit).toHaveBeenCalledExactlyOnceWith(1)
    expect(dd.pending()).toBe(2)

    vi.advanceTimersByTime(5000)
    expect(commit).toHaveBeenCalledTimes(2)
    expect(commit).toHaveBeenLastCalledWith(2)
  })

  it('flush commits the pending deletion right away', () => {
    const commit = vi.fn()
    const dd = createDeferredDelete({ delayMs: 5000, commit })

    dd.schedule(9)
    dd.flush()
    expect(commit).toHaveBeenCalledExactlyOnceWith(9)
    expect(dd.pending()).toBeNull()

    // The armed timer must not fire a second time.
    vi.advanceTimersByTime(5000)
    expect(commit).toHaveBeenCalledTimes(1)
  })

  it('flush with nothing pending does nothing', () => {
    const commit = vi.fn()
    const dd = createDeferredDelete({ delayMs: 5000, commit })
    dd.flush()
    expect(commit).not.toHaveBeenCalled()
  })
})
