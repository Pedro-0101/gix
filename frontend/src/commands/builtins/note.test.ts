import { describe, it, expect, vi } from 'vitest'
import { noteCommand } from './note'
import type { CommandContext, NoteResult } from '../types'

// A mock context capturing the surfaces the note command uses. `routeResults`
// and `overflowResult` script the backend; `chooseValue` scripts the user's
// pick on the overflow card (null = cancel).
function mockCtx(opts: {
  route?: NoteResult
  overflow?: NoteResult
  chooseValue?: string | null
}) {
  const emitted: string[] = []
  const ctx = {
    lang: 'pt',
    setView: vi.fn(),
    emitSystemMessage: (m: string) => emitted.push(m),
    choose: vi.fn(async () => opts.chooseValue ?? null),
    notes: {
      route: vi.fn(async () => opts.route ?? ({ status: 'created', noteTitle: 'X', noteId: 1, message: '' } as NoteResult)),
      resolveOverflow: vi.fn(async () => opts.overflow ?? ({ status: 'appended', noteTitle: 'X', noteId: 1, message: '' } as NoteResult)),
    },
  } as unknown as CommandContext
  return { ctx, emitted, ctxAny: ctx as any }
}

describe('noteCommand', () => {
  it('opens the notes view on empty argument and does not call the backend', async () => {
    const { ctx, ctxAny } = mockCtx({})
    await noteCommand.run(ctx, '   ')
    expect(ctxAny.setView).toHaveBeenCalledWith('notes')
    expect(ctxAny.notes.route).not.toHaveBeenCalled()
  })

  it('confirms a created note', async () => {
    const { ctx, emitted } = mockCtx({ route: { status: 'created', noteTitle: 'Compras', noteId: 3, message: '' } })
    await noteCommand.run(ctx, 'comprar shampoo')
    expect(emitted[0]).toContain('Compras')
  })

  it('confirms an appended note', async () => {
    const { ctx, emitted } = mockCtx({ route: { status: 'appended', noteTitle: 'Lembretes', noteId: 4, message: '' } })
    await noteCommand.run(ctx, 'pagar conta')
    expect(emitted[0]).toContain('Lembretes')
  })

  it('on full: asks to choose, then resolves the overflow with the chosen strategy', async () => {
    const { ctx, emitted, ctxAny } = mockCtx({
      route: { status: 'full', noteTitle: 'Cheia', noteId: 7, message: '' },
      chooseValue: 'part2',
      overflow: { status: 'created', noteTitle: 'Cheia 2', noteId: 8, message: '' },
    })
    await noteCommand.run(ctx, 'mais um item')
    expect(ctxAny.choose).toHaveBeenCalledOnce()
    expect(ctxAny.notes.resolveOverflow).toHaveBeenCalledWith(7, 'mais um item', 'part2')
    expect(emitted[emitted.length - 1]).toContain('Cheia 2')
  })

  it('on full + cancel: does not resolve overflow and reports cancellation', async () => {
    const { ctx, emitted, ctxAny } = mockCtx({
      route: { status: 'full', noteTitle: 'Cheia', noteId: 7, message: '' },
      chooseValue: null,
    })
    await noteCommand.run(ctx, 'item')
    expect(ctxAny.notes.resolveOverflow).not.toHaveBeenCalled()
    expect(emitted[emitted.length - 1]).toBe('Anotação cancelada.')
  })

  it('reports an error status', async () => {
    const { ctx, emitted } = mockCtx({ route: { status: 'error', noteTitle: '', noteId: 0, message: 'boom' } })
    await noteCommand.run(ctx, 'x')
    expect(emitted[0]).toContain('boom')
  })

  it('reports a missing API key', async () => {
    const { ctx, emitted } = mockCtx({ route: { status: 'no_api_key', noteTitle: '', noteId: 0, message: '' } })
    await noteCommand.run(ctx, 'x')
    expect(emitted[0].length).toBeGreaterThan(0)
  })
})
