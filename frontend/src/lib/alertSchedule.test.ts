import { describe, it, expect, beforeEach, vi } from 'vitest'
import * as schedule from './alertSchedule'

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

  it('rejeita se listFn falhar e não chama reconcileFn', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const listFn = vi.fn().mockRejectedValue(new Error('falha de rede'))
    const reconcileFn = vi.fn().mockResolvedValue(undefined)
    await expect(schedule.syncAlertSchedule(listFn, reconcileFn)).rejects.toThrow('falha de rede')
    expect(listFn).toHaveBeenCalledOnce()
    expect(reconcileFn).not.toHaveBeenCalled()
    expect(consoleSpy).toHaveBeenCalled()
    consoleSpy.mockRestore()
  })
})
