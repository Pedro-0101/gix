import { describe, it, expect, vi } from 'vitest'
import { askCommand } from './ask'
import type { AskResult, CommandContext, SearchState } from '../types'

function mockCtx(result: Partial<AskResult>) {
  const opened: SearchState[] = []
  const ctx = {
    lang: 'pt',
    setView: vi.fn(),
    openSearch: (s: SearchState) => opened.push(s),
    notes: {
      ask: vi.fn(async () => ({ status: 'ok', summary: '', sources: [], message: '', ...result } as AskResult)),
    },
  } as unknown as CommandContext
  return { ctx, opened, ctxAny: ctx as any }
}

describe('askCommand', () => {
  it('opens the notes view on empty argument and does not ask', async () => {
    const { ctx, ctxAny } = mockCtx({})
    await askCommand.run(ctx, '')
    expect(ctxAny.setView).toHaveBeenCalledWith('notes')
    expect(ctxAny.notes.ask).not.toHaveBeenCalled()
  })

  it('shows loading then the summary and sources', async () => {
    const { ctx, opened } = mockCtx({
      status: 'ok',
      summary: 'resumo do carro',
      sources: [{ noteId: 1, title: 'Carro', snippet: 's', content: 'c', tags: [], score: 1 }],
    })
    await askCommand.run(ctx, 'o que tem do carro?')
    expect(opened).toHaveLength(2)
    expect(opened[0]).toMatchObject({ mode: 'ask', loading: true })
    expect(opened[1]).toMatchObject({ mode: 'ask', loading: false, summary: 'resumo do carro', status: 'ok' })
    expect(opened[1].hits).toHaveLength(1)
  })
})
