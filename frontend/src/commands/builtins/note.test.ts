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
        status: 'created', noteId: 1, noteTitle: 'X', content: '', tags: [], message: '', ...capture,
      } as CaptureResult)),
      appendTo: vi.fn(async () => ({
        status: 'attached', noteId: 5, noteTitle: 'Carro', content: '', tags: ['carro'], message: '',
      } as CaptureResult)),
      createFromProposal: vi.fn(async () => ({
        status: 'created', noteId: 2, noteTitle: 'Nova', tags: [], message: '',
      })),
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

  it('appends to the AI-picked note when the user confirms attach', async () => {
    const { ctx, ctxAny, emitted } = mockCtx(
      { status: 'attach_proposed', content: 'corpo novo', tags: ['carro'], attach: { targetId: 5, targetTitle: 'Carro' } },
      'attach',
    )
    await noteCommand.run(ctx, 'mais um detalhe do carro')
    expect(ctxAny.notes.appendTo).toHaveBeenCalledWith(5, 'corpo novo', ['carro'])
    expect(ctxAny.notes.createFromProposal).not.toHaveBeenCalled()
    expect(emitted[0]).toContain('Carro') // attached confirmation names the target
  })

  it('creates a new note when the user declines the attach', async () => {
    const { ctx, ctxAny } = mockCtx(
      { status: 'attach_proposed', content: 'corpo novo', noteTitle: 'Nova', tags: [], attach: { targetId: 5, targetTitle: 'Carro' } },
      'create',
    )
    await noteCommand.run(ctx, 'algo diferente')
    expect(ctxAny.notes.createFromProposal).toHaveBeenCalledWith('Nova', 'corpo novo', [])
    expect(ctxAny.notes.appendTo).not.toHaveBeenCalled()
  })

  it('asks for an overflow strategy and resolves it when an append overflows', async () => {
    const choices: (string | null)[] = ['attach', 'split'] // confirm attach, then pick split
    let call = 0
    const resolveOverflow = vi.fn(async () => ({
      status: 'split', noteId: 9, noteTitle: 'Carro', tags: [], message: '', count: 3,
    }))
    const emitted: string[] = []
    const ctx = {
      lang: 'pt',
      setView: vi.fn(),
      emitSystemMessage: (m: string) => emitted.push(m),
      choose: vi.fn(async () => choices[call++] ?? null),
      notes: {
        capture: vi.fn(async () => ({
          status: 'attach_proposed', noteId: 0, noteTitle: 'Carro', content: 'corpo', tags: ['carro'],
          message: '', count: 0, attach: { targetId: 5, targetTitle: 'Carro' },
        })),
        appendTo: vi.fn(async () => ({
          status: 'overflow_proposed', noteId: 0, noteTitle: 'Carro', content: 'corpo', tags: ['carro'],
          message: '', count: 0, overflow: { targetId: 5, targetTitle: 'Carro', length: 9000, limit: 8000 },
        })),
        createFromProposal: vi.fn(),
        resolveOverflow,
      },
      alerts: { create: vi.fn(), createProposed: vi.fn() },
    } as unknown as CommandContext

    await noteCommand.run(ctx, 'mais um detalhe enorme do carro')
    expect(resolveOverflow).toHaveBeenCalledWith(5, 'corpo', ['carro'], 'split')
    expect(emitted[0]).toContain('Carro') // split confirmation names the note
  })

  it('does nothing when the attach choice is cancelled', async () => {
    const { ctx, ctxAny, emitted } = mockCtx(
      { status: 'attach_proposed', content: 'x', attach: { targetId: 5, targetTitle: 'Carro' } },
      null,
    )
    await noteCommand.run(ctx, 'x')
    expect(ctxAny.notes.appendTo).not.toHaveBeenCalled()
    expect(ctxAny.notes.createFromProposal).not.toHaveBeenCalled()
    expect(emitted.length).toBe(0)
  })
})
