import { describe, it, expect, vi } from 'vitest'
import { noteCommand } from './note'
import type { CommandContext, CaptureResult } from '../types'

function mockCtx(capture?: Partial<CaptureResult>, chooseValue: string | null = 'yes') {
  const emitted: string[] = []
  const ctx = {
    lang: 'pt',
    setView: vi.fn(),
    emitSystemMessage: (m: string) => emitted.push(m),
    choose: vi.fn(async () => chooseValue),
    notes: {
      capture: vi.fn(async () => ({
        status: 'created', noteId: 1, noteTitle: 'X', tags: [], message: '', ...capture,
      } as CaptureResult)),
    },
    alerts: {
      create: vi.fn(),
      createProposed: vi.fn(async () => ({
        status: 'created', alertId: 7, message: 'ligar', fireAtLocal: '2099-04-01T09:00:00-03:00', recurrence: '',
      })),
    },
  } as unknown as CommandContext
  return { ctx, emitted, ctxAny: ctx as any }
}

describe('noteCommand', () => {
  it('opens the notes view on empty argument and does not capture', async () => {
    const { ctx, ctxAny } = mockCtx()
    await noteCommand.run(ctx, '   ')
    expect(ctxAny.setView).toHaveBeenCalledWith('notes')
    expect(ctxAny.notes.capture).not.toHaveBeenCalled()
  })

  it('confirms a created note with its title and tags', async () => {
    const { ctx, emitted } = mockCtx({ status: 'created', noteTitle: 'Carro', tags: ['carro', 'manutenção'] })
    await noteCommand.run(ctx, 'o carro tá com barulho')
    expect(emitted[0]).toContain('Carro')
    expect(emitted[0]).toContain('#carro')
  })

  it('reports a missing API key', async () => {
    const { ctx, emitted } = mockCtx({ status: 'no_api_key' })
    await noteCommand.run(ctx, 'x')
    expect(emitted[0].length).toBeGreaterThan(0)
  })

  it('reports an error status', async () => {
    const { ctx, emitted } = mockCtx({ status: 'error', message: 'boom' })
    await noteCommand.run(ctx, 'x')
    expect(emitted[0]).toContain('boom')
  })

  it('proposes an alert when capture returns one and the user confirms', async () => {
    const { ctx, ctxAny } = mockCtx(
      { alert: { message: 'ligar', fireAt: '2099-04-01T09:00:00-03:00', recurrence: '' } },
      'yes',
    )
    await noteCommand.run(ctx, 'ligar pro médico amanhã 9h')
    expect(ctxAny.choose).toHaveBeenCalled()
    expect(ctxAny.alerts.createProposed).toHaveBeenCalledWith(
      expect.objectContaining({ message: 'ligar', noteId: 1 }),
    )
  })

  it('does not create the alert when the user declines', async () => {
    const { ctx, ctxAny } = mockCtx(
      { alert: { message: 'ligar', fireAt: '2099-04-01T09:00:00-03:00', recurrence: '' } },
      'no',
    )
    await noteCommand.run(ctx, 'x')
    expect(ctxAny.choose).toHaveBeenCalled()
    expect(ctxAny.alerts.createProposed).not.toHaveBeenCalled()
  })

  it('does not propose when capture returns no alert', async () => {
    const { ctx, ctxAny } = mockCtx({}, 'yes')
    await noteCommand.run(ctx, 'só uma nota')
    expect(ctxAny.choose).not.toHaveBeenCalled()
  })
})
