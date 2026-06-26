# Notes View (read-only) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only `/notes` view (mestre-detalhe) so users can browse the notes the AI already captured via `/note`.

**Architecture:** Expose the existing `db.ListNotes()` through a new `NotesService.List()` Go method, regenerate the Wails TypeScript bindings, then add a `NotesView.tsx` component mirroring `HistoryView` (title list left, markdown content right). Wire a new `'notes'` view into the shell and open it from `/note` (no arg) and a new `/notes` command.

**Tech Stack:** Go (Wails v3 services, modernc.org/sqlite), React + TypeScript, motion/react, Tailwind, Vitest, Go testing.

## Global Constraints

- Go module path: `gix`. Frontend lives in `frontend/`.
- Bindings under `frontend/bindings/` are generated — never hand-edit; regenerate via the Wails toolchain.
- Commits follow Conventional Commits in Portuguese (e.g. `feat(notes): ...`), enforced by commitlint + lefthook pre-commit (runs `go test`/`go vet`).
- i18n: every user-facing string has a `pt` and an `en` key in `frontend/src/i18n.ts`.
- Window width is fixed at 680px; views use the history pattern (fixed height, `overflow-hidden`).
- Read-only v1: no edit, delete, search, or reorder.

---

### Task 1: Backend `NotesService.List()`

**Files:**
- Modify: `internal/app/notes.go` (add method on `*NotesService`)
- Test: `internal/app/notes_test.go`

**Interfaces:**
- Consumes: `db.Database.ListNotes() ([]db.Note, error)` (exists), `db.Note` struct (exists, fields `ID, Title, Content, LineLimit, IntegrationMode, CreatedAt, UpdatedAt`).
- Produces: `(*NotesService).List() ([]db.Note, error)` — returns all notes ordered by `created_at DESC, id DESC`. Empty DB returns `(nil, nil)`. Drives the generated binding `NotesService.List`.

- [ ] **Step 1: Write the failing test**

Add to `internal/app/notes_test.go`:

```go
func TestListReturnsNotesNewestFirst(t *testing.T) {
	d := notesTestDB(t)
	if _, err := d.CreateNote("Primeira", "- a", 0, "append"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := d.CreateNote("Segunda", "- b", 0, "append"); err != nil {
		t.Fatalf("create: %v", err)
	}
	svc := newNotesSvc(t, d, &fakeCompleter{})

	notes, err := svc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("esperava 2 notas, veio %d", len(notes))
	}
	if notes[0].Title != "Segunda" || notes[1].Title != "Primeira" {
		t.Fatalf("ordem inesperada: %+v", notes)
	}
}

func TestListEmptyReturnsNoError(t *testing.T) {
	d := notesTestDB(t)
	svc := newNotesSvc(t, d, &fakeCompleter{})
	notes, err := svc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("esperava lista vazia, veio %+v", notes)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestList -v`
Expected: FAIL — `svc.List undefined (type *NotesService has no field or method List)`.

- [ ] **Step 3: Write minimal implementation**

In `internal/app/notes.go`, add after the `NewNotesService` constructor (around line 29):

```go
// List devolve todas as notas, mais recentes primeiro. Usada pela view de
// leitura de notas no frontend (binding NotesService.List).
func (s *NotesService) List() ([]db.Note, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.db.ListNotes()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run TestList -v`
Expected: PASS (both `TestListReturnsNotesNewestFirst` and `TestListEmptyReturnsNoError`).

- [ ] **Step 5: Commit**

```bash
git add internal/app/notes.go internal/app/notes_test.go
git commit -m "feat(notes): NotesService.List expoe as notas para leitura"
```

---

### Task 2: Regenerate Wails bindings

**Files:**
- Modify (generated): `frontend/bindings/gix/internal/app/notesservice.ts`
- Modify (generated): `frontend/bindings/gix/internal/app/models.ts` (adds a `Note` model)

**Interfaces:**
- Consumes: `NotesService.List()` from Task 1.
- Produces: a `List()` export in `notesservice.ts` returning `$CancellablePromise<$models.Note[]>`, and a generated `Note` class/interface in `models.ts` with the `db.Note` fields (`ID, Title, Content, LineLimit, IntegrationMode, CreatedAt, UpdatedAt`). Consumed by `NotesView.tsx` in Task 4.

- [ ] **Step 1: Regenerate bindings the canonical way**

The project regenerates bindings by running the Wails dev server (see commit `1f8bee3`). Start it in the background so it scans services and writes the TS bindings:

Run: `task dev` (background — it stays running; you only need the binding-generation pass).

Wait until the bindings are written, then stop the dev server (Ctrl-C / kill the background process).

- [ ] **Step 2: Verify the binding was generated**

Run: `git status --short frontend/bindings/`
Expected: `notesservice.ts` and `models.ts` show as modified.

Confirm `frontend/bindings/gix/internal/app/notesservice.ts` now contains an exported `List` function returning `$CancellablePromise<$models.Note[]>`, and `models.ts` defines a `Note` type with the fields above. If `task dev` is unavailable in this environment, instead run the binding generator directly: `wails3 generate bindings -config ./build/config.yml` and re-check.

- [ ] **Step 3: Typecheck the frontend**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS (no errors introduced by the regenerated bindings).

- [ ] **Step 4: Commit**

```bash
git add frontend/bindings/
git commit -m "feat(notes): regenera bindings com NotesService.List"
```

---

### Task 3: Commands and i18n to open the notes view

**Files:**
- Modify: `frontend/src/commands/types.ts` (add `'notes'` to the `View` union, line 4)
- Modify: `frontend/src/commands/builtins/note.ts` (no-arg opens the view)
- Create: `frontend/src/commands/builtins/notes.ts`
- Modify: `frontend/src/commands/registry.ts` (register `notesCommand`)
- Modify: `frontend/src/i18n.ts` (add `cmd_notes_desc`, `notes_empty` to pt and en)
- Test: `frontend/src/commands/builtins/note.test.ts`
- Test: `frontend/src/commands/registry.test.ts`

**Interfaces:**
- Consumes: `CommandContext.setView(v: View)` (exists).
- Produces: `notesCommand: Command` with `name: 'notes'`, `aliases: ['notas']`. `noteCommand` with empty arg calls `ctx.setView('notes')` instead of emitting usage. i18n keys `cmd_notes_desc`, `notes_empty` (consumed by Task 4's view and `/help`).

- [ ] **Step 1: Update the failing tests**

In `frontend/src/commands/builtins/note.test.ts`, add a `setView` spy to `mockCtx` and replace the empty-argument test. Change the `ctx` object in `mockCtx` to include:

```ts
    setView: vi.fn(),
```

(add this line right after `lang: 'pt',`). Then replace the test at lines 27–32:

```ts
  it('opens the notes view on empty argument and does not call the backend', async () => {
    const { ctx, ctxAny } = mockCtx({})
    await noteCommand.run(ctx, '   ')
    expect(ctxAny.setView).toHaveBeenCalledWith('notes')
    expect(ctxAny.notes.route).not.toHaveBeenCalled()
  })
```

In `frontend/src/commands/registry.test.ts`, add `notesCommand`-style entries to the fixed `list` (after line 10) and a test. Add to the array:

```ts
  { name: 'notes', aliases: ['notas'], descriptionKey: 'd', run: () => {} },
```

Add this test inside the `describe`:

```ts
  it('resolves the notes view command and its alias', () => {
    expect(resolveCommand('/notes', list)?.cmd.name).toBe('notes')
    expect(resolveCommand('/notas', list)?.cmd.name).toBe('notes')
  })
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/commands/builtins/note.test.ts src/commands/registry.test.ts`
Expected: FAIL — note.test expects `setView('notes')` (command still emits usage); registry.test passes only after the fixture change (its own list is local, so that test will already pass — the failing one is note.test). The note-command test fails until Step 4.

- [ ] **Step 3: Add the i18n keys**

In `frontend/src/i18n.ts`, in the `pt` object after `cmd_note_desc` (line 37) add:

```ts
  cmd_notes_desc: 'Abre as notas salvas',
```

and after `history_empty` (line 23) add:

```ts
  notes_empty: 'Nenhuma nota salva.',
```

In the `en` object, after `cmd_note_desc` (line 95) add:

```ts
  cmd_notes_desc: 'Opens the saved notes',
```

and after `history_empty` (line 81) add:

```ts
  notes_empty: 'No saved notes.',
```

- [ ] **Step 4: Implement the command changes**

In `frontend/src/commands/types.ts` line 4, change the `View` union:

```ts
export type View = 'chat' | 'settings' | 'history' | 'notes'
```

In `frontend/src/commands/builtins/note.ts`, replace the empty-arg branch (lines 13–17):

```ts
    const text = (arg ?? '').trim()
    if (!text) {
      ctx.setView('notes')
      return
    }
```

Create `frontend/src/commands/builtins/notes.ts`:

```ts
import type { Command } from '../types'

// /notes (alias /notas): abre a view de leitura das notas salvas. A captura de
// novas notas continua em /note <texto>; sem argumento, /note também cai aqui.
export const notesCommand: Command = {
  name: 'notes',
  aliases: ['notas'],
  descriptionKey: 'cmd_notes_desc',
  run: (ctx) => {
    ctx.setView('notes')
  },
}
```

In `frontend/src/commands/registry.ts`, add the import after line 6:

```ts
import { notesCommand } from './builtins/notes'
```

and add `notesCommand` to the `commands` array right after `noteCommand` (line 13):

```ts
  noteCommand,
  notesCommand,
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/commands/builtins/note.test.ts src/commands/registry.test.ts`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/commands/ frontend/src/i18n.ts
git commit -m "feat(notes): /note sem arg e /notes abrem a view de notas"
```

---

### Task 4: `NotesView` component and shell wiring

**Files:**
- Create: `frontend/src/views/NotesView.tsx`
- Modify: `frontend/src/App.tsx` (local `View` type line 19; render `<NotesView>`; import)

**Interfaces:**
- Consumes: `NotesService.List()` binding (Task 2), `db.Note` model fields (`ID, Title, Content`), `MessageCard` component, `Button` component, `tr` i18n, `notes_empty`/`cancel` keys.
- Produces: `NotesView({ lang, onClose })` default-styled view; rendered by App when `view === 'notes'`.

- [ ] **Step 1: Create the view component**

Create `frontend/src/views/NotesView.tsx` (mirrors `HistoryView`, read-only — no delete button):

```tsx
import { useEffect, useState } from 'react'
import { motion } from 'motion/react'
import { NotesService } from '../../bindings/gix/internal/app'
import { MessageCard } from '../components/MessageCard'
import { Button } from '../components/Button'
import { tr } from '../i18n'

// Read-only notes browser: title list on the left, selected note's markdown on
// the right. Mirrors HistoryView's master-detail layout. Esc closes via App's
// global handler; the back button calls onClose.
export function NotesView({ lang, onClose }: { lang: string; onClose: () => void }) {
  const [notes, setNotes] = useState<any[]>([])
  const [activeId, setActiveId] = useState<any>(null)

  useEffect(() => {
    NotesService.List().then((n) => {
      const list = n ?? []
      setNotes(list)
      if (list.length > 0) setActiveId(list[0].ID)
    })
  }, [])

  const active = notes.find((n) => n.ID === activeId)

  return (
    <div className="flex h-full font-mono text-fg">
      <div className="flex w-2/5 flex-col border-r border-fg/8">
        <div className="shrink-0 p-2">
          <Button variant="ghost" onClick={onClose} className="gap-1">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
              strokeLinecap="round" strokeLinejoin="round" className="size-4">
              <path d="M15 18l-6-6 6-6" />
            </svg>
            {tr(lang, 'cancel')}
          </Button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
          {notes.length === 0 && (
            <div className="px-1 py-3 text-sm text-muted">{tr(lang, 'notes_empty')}</div>
          )}
          <div className="space-y-0.5">
            {notes.map((n, i) => {
              const isActive = activeId === n.ID
              return (
                <motion.div
                  key={n.ID}
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25, ease: 'easeOut', delay: Math.min(i * 0.03, 0.2) }}
                  className={`rounded-field transition-colors duration-150 ${
                    isActive ? 'bg-surface shadow-[var(--shadow-border)]' : 'hover:bg-surface/60'
                  }`}
                >
                  <button
                    onClick={() => setActiveId(n.ID)}
                    className="block w-full cursor-pointer truncate px-2.5 py-2 text-left text-sm outline-none"
                  >
                    {n.Title}
                  </button>
                </motion.div>
              )
            })}
          </div>
        </div>
      </div>
      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-3 selectable">
        {active && (
          <MessageCard role="assistant" content={active.Content} label={active.Title} />
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Wire the view into App**

In `frontend/src/App.tsx`:

Add the import after the `HistoryView` import (line 11):

```ts
import { NotesView } from './views/NotesView'
```

Change the local `View` type (line 19):

```ts
type View = 'chat' | 'settings' | 'history' | 'notes'
```

The expanded-panel height handling treats `history` specially (line 444–445). Apply the same fixed-height treatment to `notes`. Replace those two lines:

```tsx
          className={`min-h-0 border-t border-[color:var(--shell-border)] selectable ${view === 'history' || view === 'notes' ? 'overflow-hidden' : 'overflow-y-auto'}`}
          style={view === 'history' || view === 'notes' ? { height: panelMax } : { maxHeight: panelMax }}
```

Add the render branch right after the `history` branch (line 515):

```tsx
          {view === 'notes' && <NotesView lang={lang} onClose={() => setView('chat')} />}
```

- [ ] **Step 3: Typecheck and build the frontend**

Run: `cd frontend && npx tsc --noEmit`
Expected: PASS — no type errors.

- [ ] **Step 4: Run the full frontend test suite**

Run: `cd frontend && npx vitest run`
Expected: PASS — all existing tests plus Task 3's still green.

- [ ] **Step 5: Manual verification**

Run: `task dev`, open the window, type `/notes` (and separately a bare `/note`). Confirm the view opens, lists existing note titles newest-first, shows the first note's content rendered as markdown on the right, switching selection updates the detail, and Esc / the back button returns to chat. With an empty DB, confirm the `notes_empty` message shows.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/NotesView.tsx frontend/src/App.tsx
git commit -m "feat(notes): view de leitura mestre-detalhe das notas"
```

---

## Self-Review notes

- **Spec coverage:** backend List (Task 1) + bindings (Task 2); master-detail read-only view (Task 4); `/note` no-arg + `/notes`/`/notas` opening (Task 3); i18n `cmd_notes_desc`/`notes_empty` (Task 3); tests across Tasks 1, 3. Out-of-scope items (edit/delete/search) intentionally omitted.
- **Type consistency:** `db.Note` fields (`ID/Title/Content`) used in the binding (Task 2) match `NotesView` usage (Task 4); `View` union extended in both `commands/types.ts` (Task 3) and `App.tsx` (Task 4); `setView('notes')` used in command (Task 3) and handled in App render (Task 4).
- **Note:** `note_usage` i18n key is now unused by the flow but left in place (harmless; referenced by no code path after Task 3).
```
