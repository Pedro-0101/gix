# Design — view de notas v1 (somente leitura)

Data: 2026-06-26
Status: aprovado

## Objetivo

Permitir visualizar as notas já existentes dentro do gix. Hoje as notas são
criadas/anexadas via `/note <texto>` (roteamento por IA) e ficam no SQLite, mas
não há como lê-las pela interface. Esta v1 entrega uma view de leitura.

## Contexto

O gix é uma janela única tipo Spotlight/Raycast: barra fixa de 680px que cresce
para baixo, com "views" no painel (`chat`, `settings`, `history`). O `/history`
já é o caso análogo: view mestre-detalhe que lista conversas do SQLite via
binding e fecha com Esc. As notas vivem no mesmo banco e o backend já tem
`db.ListNotes()` e `db.GetNote()`.

Decisões de brainstorming:

- **Escopo:** somente leitura (v1 enxuta). Sem editar/apagar/buscar.
- **Layout:** mestre-detalhe, espelhando o `/history` (lista de títulos à
  esquerda ~40%, conteúdo à direita ~60%).
- **Abertura:** `/note` sem argumento abre a view E um novo `/notes` (alias
  `/notas`) também abre. `/note <texto>` continua capturando.
- **Medium rejeitados:** segunda janela nativa (quebra o overlay único e o
  auto-resize), exportar pro navegador (sai do fluxo/tema/teclado). Mensagem no
  chat serve só pra caso pontual, não pra navegação.

## Arquitetura

### Backend (Go)

Adicionar `NotesService.List() ([]db.Note, error)` que repassa
`db.ListNotes()` (já ordenado por `created_at DESC, id DESC`). Isso expõe o
model `db.Note` nos bindings gerados. Sem mudança de schema — `ListNotes` e
`GetNote` já existem. Regenerar bindings no estilo canônico do projeto
(`wails dev`), como nos commits anteriores de notas.

### Frontend — `NotesView.tsx`

Componente novo espelhando `HistoryView`, mestre-detalhe:

- **Esquerda (~40%):** lista de títulos das notas. Botão "voltar" no topo;
  Esc também fecha (tratado pelo handler global do App). Empty state
  (`notes_empty`) quando a lista é vazia.
- **Direita (~60%):** conteúdo da nota selecionada renderizado como markdown,
  reusando `MessageCard` (mesmo componente que o `HistoryView` usa no detalhe,
  role `assistant`).
- A view chama `NotesService.List()` direto pelo binding (igual o
  `HistoryView` chama `HistoryService`). **Não altera `CommandContext`.**
- Animação de entrada dos itens da lista igual ao history (stagger leve).

### Wiring no shell (`App.tsx` + `commands/types.ts`)

- Adicionar `'notes'` à união `View` nos dois arquivos.
- Renderizar `<NotesView lang={lang} onClose={() => setView('chat')} />` quando
  `view === 'notes'`, com o mesmo tratamento de altura fixa do history
  (`overflow-hidden` + `height: panelMax`).

### Comandos

- `note.ts`: quando `arg` está vazio, em vez de `emitSystemMessage(note_usage)`,
  chamar `ctx.setView('notes')`. Com `arg`, comportamento atual intacto.
- Novo `notes.ts`: comando `name: 'notes'`, `aliases: ['notas']`,
  `descriptionKey: 'cmd_notes_desc'`, `run: (ctx) => ctx.setView('notes')`.
  Registrado em `registry.ts`.

### i18n

Chaves novas em pt e en:

- `cmd_notes_desc` — descrição do `/notes` no `/help`.
- `notes_empty` — estado vazio da lista.

(`note_usage` deixa de ser usada pelo fluxo sem-arg; manter ou remover conforme
ainda referenciada.)

## Fluxo de dados

1. Usuário digita `/note` ou `/notes` → comando chama `setView('notes')`.
2. `NotesView` monta, chama `NotesService.List()` → `db.ListNotes()`.
3. Lista renderiza títulos; clicar seleciona uma nota e mostra `Content` à
   direita como markdown.
4. Esc/voltar → `setView('chat')`.

## Tratamento de erros

- `List()` falha → lista vazia (mesmo padrão tolerante do `HistoryView`, que faz
  `.then((c) => setConvs(c ?? []))`).
- Sem notas → empty state.

## Testes

- `internal/app/notes_test.go`: `List()` devolve as notas criadas, na ordem
  esperada.
- `frontend/src/commands/builtins/note.test.ts`: `/note` sem arg chama
  `setView('notes')` e não emite mensagem de uso; `/note <texto>` mantém a
  captura.
- `registry.test.ts`: `/notes` e `/notas` resolvem para o comando de notas.

## Fora de escopo (v2+)

Editar conteúdo/título, apagar (lixeira reusando o padrão do history), busca,
reordenar, fixar notas.
