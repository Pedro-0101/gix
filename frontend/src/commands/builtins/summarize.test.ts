import { describe, it, expect, vi } from 'vitest'
import { summarizeCommand } from './summarize'
import type { CommandContext, SearchHit, SummarizeResult } from '../types'

function mockCtx(opts: {
  hits?: SearchHit[]
  summary?: Partial<SummarizeResult>
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
      summarize: vi.fn(async () => ({ status: 'ok', summary: 'resumo curto', message: '', ...opts.summary } as SummarizeResult)),
      update,
    },
  } as unknown as CommandContext
  return { ctx, ctxAny: ctx as any, update }
}

const hit: SearchHit = { noteId: 7, title: 'Reunião', snippet: 's', content: 'conteúdo original longo', tags: ['t'], score: 1 }

describe('summarizeCommand', () => {
  it('opens the notes view on empty argument and does not summarize', async () => {
    const { ctx, ctxAny } = mockCtx({})
    await summarizeCommand.run(ctx, '')
    expect(ctxAny.setView).toHaveBeenCalledWith('notes')
    expect(ctxAny.notes.summarize).not.toHaveBeenCalled()
  })

  it('reports when no note matches', async () => {
    const { ctx, ctxAny } = mockCtx({ hits: [] })
    await summarizeCommand.run(ctx, 'algo')
    expect(ctxAny.notes.summarize).not.toHaveBeenCalled()
    expect(ctxAny.emitSystemMessage).toHaveBeenCalled()
  })

  it('replaces the note content with the summary when confirmed', async () => {
    const { ctx, update } = mockCtx({ hits: [hit], chooseReplies: ['yes', 'keep'] })
    await summarizeCommand.run(ctx, 'reunião')
    expect(update).toHaveBeenCalledWith(7, 'Reunião', 'resumo curto', ['t'])
  })

  it('does not replace when cancelled', async () => {
    const { ctx, update } = mockCtx({ hits: [hit], chooseReplies: ['no'] })
    await summarizeCommand.run(ctx, 'reunião')
    expect(update).not.toHaveBeenCalled()
  })

  it('restores the original content on undo', async () => {
    const { ctx, update } = mockCtx({ hits: [hit], chooseReplies: ['yes', 'undo'] })
    await summarizeCommand.run(ctx, 'reunião')
    expect(update).toHaveBeenNthCalledWith(1, 7, 'Reunião', 'resumo curto', ['t'])
    expect(update).toHaveBeenNthCalledWith(2, 7, 'Reunião', 'conteúdo original longo', ['t'])
  })
})
