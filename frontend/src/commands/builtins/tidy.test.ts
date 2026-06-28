import { describe, it, expect, vi } from 'vitest'
import { tidyCommand } from './tidy'
import type { CommandContext, SearchHit, TidyResult } from '../types'

function mockCtx(opts: {
  hits?: SearchHit[]
  tidy?: Partial<TidyResult>
  chooseReplies?: string[] // values returned by successive ctx.choose calls
}) {
  const chooseReplies = [...(opts.chooseReplies ?? [])]
  const update = vi.fn(async () => {})
  const ctx = {
    lang: 'pt',
    setView: vi.fn(),
    emitSystemMessage: vi.fn(),
    choose: vi.fn(async () => chooseReplies.shift() ?? null),
    notes: {
      find: vi.fn(async () => opts.hits ?? []),
      tidy: vi.fn(async () => ({ status: 'ok', content: 'nota organizada', message: '', ...opts.tidy } as TidyResult)),
      update,
    },
  } as unknown as CommandContext
  return { ctx, ctxAny: ctx as any, update }
}

const hit: SearchHit = { noteId: 7, title: 'Projeto', snippet: 's', content: 'bagunça original', tags: ['t'], score: 1 }

describe('tidyCommand', () => {
  it('opens the notes view on empty argument and does not tidy', async () => {
    const { ctx, ctxAny } = mockCtx({})
    await tidyCommand.run(ctx, '')
    expect(ctxAny.setView).toHaveBeenCalledWith('notes')
    expect(ctxAny.notes.tidy).not.toHaveBeenCalled()
  })

  it('reports when no note matches', async () => {
    const { ctx, ctxAny } = mockCtx({ hits: [] })
    await tidyCommand.run(ctx, 'algo')
    expect(ctxAny.notes.tidy).not.toHaveBeenCalled()
    expect(ctxAny.emitSystemMessage).toHaveBeenCalled()
  })

  it('replaces the note content with the tidied version when confirmed', async () => {
    const { ctx, update } = mockCtx({ hits: [hit], chooseReplies: ['yes', 'keep'] })
    await tidyCommand.run(ctx, 'projeto')
    expect(update).toHaveBeenCalledWith(7, 'Projeto', 'nota organizada', ['t'])
  })

  it('does not replace when cancelled', async () => {
    const { ctx, update } = mockCtx({ hits: [hit], chooseReplies: ['no'] })
    await tidyCommand.run(ctx, 'projeto')
    expect(update).not.toHaveBeenCalled()
  })

  it('restores the original content on undo', async () => {
    const { ctx, update } = mockCtx({ hits: [hit], chooseReplies: ['yes', 'undo'] })
    await tidyCommand.run(ctx, 'projeto')
    expect(update).toHaveBeenNthCalledWith(1, 7, 'Projeto', 'nota organizada', ['t'])
    expect(update).toHaveBeenNthCalledWith(2, 7, 'Projeto', 'bagunça original', ['t'])
  })
})
