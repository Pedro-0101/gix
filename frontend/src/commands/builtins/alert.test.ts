import { describe, it, expect, vi } from 'vitest'
import { alertCommand } from './alert'
import type { CommandContext, CreateAlertResult } from '../types'

function mockCtx(create?: Partial<CreateAlertResult>) {
  const emitted: string[] = []
  const ctx = {
    lang: 'pt',
    setView: vi.fn(),
    emitSystemMessage: (m: string) => emitted.push(m),
    alerts: {
      create: vi.fn(async () => ({
        status: 'created', alertId: 1, message: 'X', fireAtLocal: '2026-06-26T09:00:00-03:00', recurrence: '', ...create,
      } as CreateAlertResult)),
    },
  } as unknown as CommandContext
  return { ctx, emitted, ctxAny: ctx as any }
}

describe('alertCommand', () => {
  it('opens the alerts view on empty argument and does not create', async () => {
    const { ctx, ctxAny } = mockCtx()
    await alertCommand.run(ctx, '   ')
    expect(ctxAny.setView).toHaveBeenCalledWith('alerts')
    expect(ctxAny.alerts.create).not.toHaveBeenCalled()
  })

  it('confirms a created alert with its message', async () => {
    const { ctx, emitted } = mockCtx({ status: 'created', message: 'Médico' })
    await alertCommand.run(ctx, 'médico amanhã às 9h')
    expect(emitted[0]).toContain('Médico')
  })

  it('reports a missing API key', async () => {
    const { ctx, emitted } = mockCtx({ status: 'no_api_key' })
    await alertCommand.run(ctx, 'x')
    expect(emitted[0].length).toBeGreaterThan(0)
  })

  it('reports an unparseable date', async () => {
    const { ctx, emitted } = mockCtx({ status: 'unparseable' })
    await alertCommand.run(ctx, '???')
    expect(emitted[0].length).toBeGreaterThan(0)
  })
})
