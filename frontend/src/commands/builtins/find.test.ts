import { describe, it, expect, vi } from 'vitest'
import { findCommand } from './find'
import type { CommandContext, SearchHit, SearchState } from '../types'

function mockCtx(hits: SearchHit[]) {
  const opened: SearchState[] = []
  const ctx = {
    lang: 'pt',
    setView: vi.fn(),
    openSearch: (s: SearchState) => opened.push(s),
    notes: { find: vi.fn(async () => hits) },
  } as unknown as CommandContext
  return { ctx, opened, ctxAny: ctx as any }
}

const hit: SearchHit = { noteId: 1, title: 'Carro', snippet: 'motor', content: 'motor do carro', tags: ['carro'], score: 1 }

describe('findCommand', () => {
  it('opens the notes view on empty argument and does not search', async () => {
    const { ctx, ctxAny } = mockCtx([hit])
    await findCommand.run(ctx, '  ')
    expect(ctxAny.setView).toHaveBeenCalledWith('notes')
    expect(ctxAny.notes.find).not.toHaveBeenCalled()
  })

  it('shows a loading state then the results', async () => {
    const { ctx, opened } = mockCtx([hit])
    await findCommand.run(ctx, 'carro')
    expect(opened).toHaveLength(2)
    expect(opened[0]).toMatchObject({ mode: 'find', loading: true, hits: [] })
    expect(opened[1]).toMatchObject({ mode: 'find', loading: false })
    expect(opened[1].hits).toHaveLength(1)
  })
})
