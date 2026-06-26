# Design — Edição e exclusão de notas

Data: 2026-06-26
Status: aprovado (design); aguardando revisão da spec antes do plano

## Contexto

O sistema de notas já tem captura (`/note` → `NotesService.Capture`), leitura
mestre-detalhe (`NotesView`), busca híbrida (`/find` → `Find`) e Q&A sobre as
notas (`/ask` → `Ask`). O que falta no ciclo de vida da nota: depois de
capturada, ela é **imutável pela UI** — não dá para editar nem apagar. O
`db.DeleteNote` existe na camada de banco mas nem é exposto no `NotesService`.

Este design adiciona **edição manual** e **exclusão com desfazer**, fechando o
CRUD. Também serve de fundação para itens futuros do `docs/todo` (`/tidy`,
config por nota), que precisam de um caminho de atualização de nota.

## Decisões (definidas no brainstorming)

- **UX de edição:** tela/modo de edição dedicado, em tela cheia.
- **Campos editáveis:** título + corpo + tags.
- **Exclusão:** com desfazer (undo via toast), sem confirmação prévia.
- **Ao salvar:** grava o texto **exatamente** como o usuário escreveu (sem
  reformatar com IA, sem custo de API), reembedda **localmente** (ONNX, grátis)
  e atualiza o FTS. Edição manual = sem chamada de IA.
- **Estrutura de código:** modo de edição dentro da `NotesView` (abordagem A) —
  não toca em `App.tsx`. A `NotesView` já é dona do estado `notes`.
- **Tags na edição manual:** sem cap (a captura por IA mantém o limite de 5).
- **Descartar alterações não salvas:** pede confirmação.

## Arquitetura

A `NotesView` (renderizada dentro do painel do `App`) coordena três estados:
navegação (mestre-detalhe atual), edição (`NoteEditor` em tela cheia) e exclusão
pendente (toast de undo). Backend ganha `Update`/`Delete` no `NotesService`,
apoiados num novo `db.UpdateNote`.

### Backend Go

**`db.UpdateNote(id, title, content, tags, vec, dim)`** — espelha `CreateNote`,
numa transação:

- `UPDATE notes SET title=?, content=? WHERE id=?` (preserva `created_at` e `id`)
- `DELETE FROM note_tags WHERE note_id=?` + reinsere as tags novas
- substitui `note_vectors` (`DELETE` + `INSERT`; se `vec` vazio, fica sem vetor)
- ressincroniza FTS: `DELETE FROM notes_fts WHERE rowid=?` + `INSERT INTO
  notes_fts (rowid, title, content, tags) VALUES (id, title, content, tags)`

**`NotesService.Update(id, title, content, tags) (UpdateResult, error)`**:

- trim de título/corpo; se o corpo for vazio, mantém o texto; se o título for
  vazio, deriva via `ExtractTitle` (mesma lógica de fallback do `Capture`)
- normaliza tags com a versão **sem cap** (ver abaixo)
- reembedda localmente `title+"\n"+content` se `s.embedder != nil` (grátis,
  ONNX); senão salva sem vetor (degrada para FTS, igual ao `Capture`)
- chama `db.UpdateNote`. **Sem chamada de IA, sem custo.**
- retorna a nota atualizada (para a UI re-renderizar)

**`NotesService.Delete(id int64) error`** — expõe o `db.DeleteNote` existente.

**Normalização de tags:** extrair o núcleo do `normalizeTags` atual (trim,
minúsculas, remove `#`, dedup) numa função sem truncamento. `Capture` continua
chamando uma variante que corta em 5; `Update` usa a versão sem cap.

**Bindings Wails** regeneradas após as mudanças, expondo `Update` e `Delete` em
`notesservice.ts`.

### Frontend

**`NotesView`** (coordenadora; continua dona de `notes`):

- estado `editingId: number | null`
- no painel de detalhe (direita), cabeçalho com botões **Editar** e **Excluir**
  (estilo `ghost`, como o botão de voltar atual)
- quando `editingId != null`, renderiza `<NoteEditor>` em tela cheia no lugar do
  mestre-detalhe
- hospeda o toast de undo (rodapé do painel, posição absoluta)

**`NoteEditor.tsx`** (novo) — tela cheia dentro do painel:

- cabeçalho: `‹ voltar` (cancelar) à esquerda, **Salvar** (accent) à direita
- campo **Título**: `input` de uma linha
- campo **Tags**: input com chips — `Enter`/vírgula adiciona; `x` remove;
  `Backspace` no vazio remove o último; dedup
- campo **Corpo**: `textarea` grande, auto-cresce, monospace
- props: `Note` atual + `onSave(title, content, tags)` e `onCancel`
- **Salvar** → `NotesService.Update`, atualiza a lista na `NotesView` com a nota
  retornada, sai do modo edição, seleciona a nota editada
- **dirty-state:** se houver alterações não salvas, `Esc`/voltar pede
  confirmação antes de descartar
- **Esc:** capturado em fase de captura dentro do editor (como a `NotesView` já
  faz com as setas) para cancelar a edição em vez de fechar a view inteira

**i18n:** novas chaves em `i18n.ts` (`edit`, `delete`, `save`, `note_deleted`,
`undo`, `discard_changes`, ...), seguindo o padrão `tr(lang, ...)`.

### Exclusão com undo (deferred delete)

A exclusão real é **adiada**, então o undo não precisa de nada no backend:

1. Ao clicar **Excluir**, a nota some da lista só na UI (estado local),
   seleciona-se a vizinha, aparece o toast *"Nota excluída — Desfazer"*.
2. Um timer (~5s) é armado. A nota ainda existe no banco nesse intervalo.
3. Timer expira → chama `NotesService.Delete(id)` de verdade.
4. Desfazer → cancela o timer, reinsere a nota na posição original, reseleciona.
   Nada tocou o banco; `id`, `created_at` e embedding preservados.

**Casos de borda:**

- **Excluir uma segunda nota com toast ativo** → flush imediato da exclusão
  pendente anterior (executa o `Delete`), depois inicia a nova. Um toast por vez.
- **Fechar a NotesView / app com delete pendente** → no unmount da view, flush
  das exclusões pendentes. Se o app for morto à força, a nota sobrevive (falha
  segura).

A lógica de deferred delete é extraída em `frontend/src/lib/deferredDelete.ts`
(função/hook puro e testável: arma timer, expira → delete, undo → restaura,
flush em novo delete/unmount).

O toast é um componente pequeno e local (inline na `NotesView` ou
`NoteDeleteToast`). **Não** será construído um sistema de notificação genérico —
isso é o item "área de comunicação do sistema" do `docs/todo`, para depois.

## Testes

**Backend (Go)** — padrão de `notes_test.go` (banco temporário via `db.Open`,
completer fake):

- `db.UpdateNote`: atualiza título/corpo/tags; preserva `id` e `created_at`; FTS
  reflete o novo texto (termo antigo não acha mais, novo acha); `note_vectors`
  substituído quando há `vec`.
- `NotesService.Update`: normaliza tags sem cap (>5 sobrevivem); fallback de
  título vazio via `ExtractTitle`; reembedda quando o embedder está presente
  (fake embedder, como em `notes_realembed_test.go`) e salva sem vetor quando
  ausente; **não chama a IA** (completer fake não é invocado).
- `NotesService.Delete`: remove a nota e todas as linhas derivadas (`notes`,
  `note_tags`, `note_vectors`, `notes_fts`).

**Frontend (TS)** — padrão de `ask.test.ts`/`find.test.ts`:

- `deferredDelete`: transições com timers fake (arma → expira → delete; undo →
  restaura; flush em novo delete/unmount).
- tag-input: parsing de entrada → chips (Enter/vírgula adiciona, dedup, remove).

A renderização visual fica para verificação manual no app (`wails dev`) ao final.

## Arquivos

| Arquivo | Mudança |
|---|---|
| `internal/db/db.go` | `+UpdateNote` |
| `internal/app/notes.go` | `+Update`, `+Delete`, normalização sem cap |
| `internal/app/notes_test.go` | testes de update/delete |
| `frontend/bindings/...` | regenerar (Update/Delete) |
| `frontend/src/views/NotesView.tsx` | coordena edição + toast undo |
| `frontend/src/views/NoteEditor.tsx` | novo — editor tela cheia |
| `frontend/src/lib/deferredDelete.ts` (+ test) | novo — lógica de undo |
| `frontend/src/i18n.ts` | novas chaves |

## Fora de escopo

- `/tidy` (IA reorganiza/resume nota) — fundação criada aqui, feature depois.
- Config por nota (limite de linhas / modo de integração).
- Sistema de notificação genérico ("área de comunicação do sistema").
- Lixeira / soft delete com `deleted_at` (o deferred delete dispensa).
