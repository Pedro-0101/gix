import { describe, it, expect, vi } from 'vitest'
import { noteCommand } from './note'
import type { CommandContext, CaptureResult } from '../types'

function mockCtx(capture?: Partial<CaptureResult>) {
  const emitted: string[] = []
  const ctx = {
    lang: 'pt',
    setView: vi.fn(),
    emitSystemMessage: (m: string) => emitted.push(m),
    notes: {
      capture: vi.fn(async () => ({
        status: 'created', noteId: 1, noteTitle: 'X', tags: [], message: '', ...capture,
      } as CaptureResult)),
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
})
