import { describe, it, expect, beforeEach, vi } from 'vitest'
import * as schedule from './alertSchedule'
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

describe('syncAlertSchedule', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('mapeia e encaminha corretamente, descartando campos extras', async () => {
    const listFn = vi.fn().mockResolvedValue([
      { id: 1, message: 'teste', fireAt: '2026-06-30T10:00:00Z', status: 'active', extra: 'ignorado' },
    ])
    const reconcileFn = vi.fn().mockResolvedValue(undefined)
    await expect(schedule.syncAlertSchedule(listFn, reconcileFn)).resolves.toBeUndefined()
    expect(listFn).toHaveBeenCalledOnce()
    expect(reconcileFn).toHaveBeenCalledOnce()
    expect(reconcileFn).toHaveBeenCalledWith([
      { id: 1, message: 'teste', fireAt: '2026-06-30T10:00:00Z', status: 'active' },
    ])
  })

  it('engole listFn rejeitada e não chama reconcileFn', async () => {
    const listFn = vi.fn().mockRejectedValue(new Error('falha de rede'))
    const reconcileFn = vi.fn().mockResolvedValue(undefined)
    await expect(schedule.syncAlertSchedule(listFn, reconcileFn)).resolves.toBeUndefined()
    expect(listFn).toHaveBeenCalledOnce()
    expect(reconcileFn).not.toHaveBeenCalled()
  })
})
