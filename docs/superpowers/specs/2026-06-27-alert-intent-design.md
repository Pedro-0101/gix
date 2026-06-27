# Alertas em linguagem natural (sem `/alerta`)

Data: 2026-06-27
Branch: `feat/alertas`

## Problema

Hoje um alerta só nasce do comando explícito `/alerta <texto>`. O usuário espera
descrever um lembrete em linguagem natural e ver o alerta criado — tanto dentro
de `/note` quanto em uma mensagem comum (sem comando). Ao digitar
`/note Criar alarme para daqui 1 minuto…` o texto virou apenas uma **nota**
(comportamento correto de `/note`), e nenhum alerta foi agendado.

## Objetivo

Detectar a intenção de "lembrete" em dois pontos de entrada e, após **confirmação
do usuário**, criar o alerta:

1. **`/note`** — quando o texto descreve um lembrete com horário/data concreto,
   salvar a nota normalmente **e** propor um alerta vinculado a ela.
2. **Mensagem comum (sem comando)** — o próprio chat decide, via *tool calling*,
   se a mensagem é um lembrete e propõe o alerta.

Decisões do usuário que orientam o design:

- Detecção no chat = a IA do chat decide na mesma chamada (tool calling), não um
  classificador separado antes do chat.
- `/note`: **sempre** cria a nota; o alerta só é proposto quando a IA identifica
  um horário/data concreto.
- **Confirmação antes de criar**: mostrar um chip de escolha (Sim/Não) usando o
  primitivo `choose` que já existe. Nunca criar silenciosamente.

Fora de escopo: mudar o `/alerta` (continua funcionando igual); reescrever o
agendador; tornar o chat multi-turn com resultados de ferramenta.

## Conceito central: "alert proposal"

Os dois produtores convergem para a mesma estrutura — uma **proposta**
`{ message, fireAt, recurrence, noteId? }` — que passa por **um** caminho único
de confirmar-e-criar:

```
producer ──► proposal ──► chip de confirmação (choose) ──► [Sim] ──► grava alerta
```

Como há confirmação, o passo de criação **não** chama a IA de novo: ele grava os
campos já parseados. Detecção e criação ficam separadas.

## Arquitetura

### Backend (Go)

**`internal/ai/client.go` — suporte a tool calling**

- Novo tipo `Tool` (JSON schema da função) e campos de tool call em `Message`.
- `chatRequest` ganha `Tools []Tool` (omitempty — requests sem ferramentas ficam
  idênticos aos de hoje).
- O parser de streaming acumula `choices[].delta.tool_calls[]`: cada delta traz
  pedaços incrementais de `function.arguments` (string) que são concatenados por
  índice. Ao final do stream, retorna as tool calls completas (nome + arguments
  JSON) junto do `Usage`.
- `Stream` ganha um canal de retorno para tool calls. Assinatura proposta:
  `Stream(ctx, model, msgs, tools, onDelta) (*Usage, []ToolCall, error)`.
  O parsing de texto (`delta.content`) permanece inalterado.

**`internal/app/alerts.go` — separar parse de gravação**

- Refatorar `createFromWhen` em duas partes:
  - `parseWhen(...)` (já existe) — a chamada de IA.
  - **novo** `storeDecision(dec alertDecision, defaultMessage string, noteID *int64) CreateAlertResult`
    — validação (`fire_at` válido, mensagem não vazia, regra de "past"), monta o
    `db.Alert` e grava. Sem IA.
- **novo** método exportado
  `CreateProposed(message, fireAtISO, recurrence string, noteID *int64) (CreateAlertResult, error)`
  — monta um `alertDecision` a partir dos campos já parseados e chama
  `storeDecision`. É o caminho compartilhado de criação para os dois produtores.
- `Create` e `CreateForNote` passam a usar `parseWhen` + `storeDecision`.

**`internal/app/notes.go` — capture detecta alerta**

- `captureDecision` ganha `Alert *alertDecision` (`json:"alert"`).
- `CaptureResult` ganha `Alert *AlertProposal` (struct nova:
  `Message string`, `FireAtLocal string`, `Recurrence string`), preenchido só
  quando o modelo retornou um alerta **válido, futuro e com horário concreto**.
  Validação reusa o parse de `fire_at` (RFC3339) e a checagem de "past".
- O prompt de captura (`buildCapturePrompt`) é estendido para emitir o campo
  `alert` (null quando a nota não tem tempo concreto). A nota é **sempre** criada
  independentemente do alerta.

**`internal/app/chat.go` — chat propõe alerta**

- `ChatService.Send` passa a ferramenta `create_alert`
  (`{message, fire_at, recurrence}`, mesma forma do `alertDecision`) para o
  client.
- Se o stream retornar uma tool call `create_alert`: em vez de tratar como texto,
  emite o evento `alert:proposed` com os argumentos parseados; o turno do
  assistente é registrado no histórico como uma nota curta de sistema (ex.:
  "(propôs um alerta)") para manter a conversa coerente. **Não** há segundo
  round-trip enviando resultado de ferramenta ao modelo — a confirmação é local.
- Se não houver tool call: fluxo atual intacto (`chat:delta`/`chat:done`).
- Se vier texto **e** tool call: emite o texto normalmente e também propõe.

### Frontend (TS)

- `lib/events.ts` — `onAlertProposed(cb)` para o evento `alert:proposed`;
  payload `{ message, fireAt, recurrence, noteId? }` (tipo `AlertProposedPayload`).
- `commands/types.ts` —
  - `CaptureResult` ganha `alert?: { message: string; fireAt: string; recurrence: string }`.
  - `CommandContext.alerts` ganha
    `createProposed(p: { message: string; fireAt: string; recurrence: string; noteId?: number }): Promise<CreateAlertResult>`.
- `App.tsx` —
  - `commandContext.alerts.createProposed` → `AlertsService.CreateProposed(...)`.
  - Novo efeito `onAlertProposed` (espelha `onAlertFired`): roda o chip
    `choose("Criar alerta — <when>?", [Sim, Não])`; em Sim chama `createProposed`
    e posta a mensagem de sistema de confirmação; em Não/Esc não grava nada.
- `commands/builtins/note.ts` — após `capture` bem-sucedido, se `res.alert`
  presente → chip `choose` → `ctx.alerts.createProposed({ ...res.alert, noteId })`
  → mensagem de sistema.

## Fluxo de dados

### `/note comprar leite amanhã 9h`

```
note cmd → NotesService.Capture
  → 1 chamada de IA → {title, content, tags, alert:{msg, fire_at, recurrence}}
  → grava nota (sempre)
  → CaptureResult{ ..., alert:{message, fireAtLocal, recurrence} }
note cmd: res.alert presente → ctx.choose("Criar alerta — amanhã 09:00?")
  → Sim → ctx.alerts.createProposed({message, fireAt, recurrence, noteId})
        → AlertsService.CreateProposed → storeDecision → DB
        → emitSystemMessage("Agendado: …")
  → Não → nada gravado (nota permanece)
```

### `me lembra de ligar pro médico amanhã 9h` (chat, sem comando)

```
send() → ChatService.Send (tools: [create_alert])
  → modelo retorna tool_call create_alert{message, fire_at, recurrence}
  → emit alert:proposed{...}
App onAlertProposed → choose("Criar alerta — amanhã 09:00?")
  → Sim → AlertsService.CreateProposed → storeDecision → DB → mensagem de sistema
  → Não → nada gravado
```

Mensagem comum sem tool call → fluxo de chat idêntico ao de hoje.

## Tratamento de erros e casos de borda

- **Tempo no passado / sem recorrência** → `storeDecision` retorna `past`; o chip
  não é exibido (`/note`) ou a proposta é descartada com uma nota curta de sistema
  (chat). Só propomos chip para propostas válidas e futuras.
- **Campos faltando/inválidos** na tool call → descarta no chat (nada extra); em
  `/note` a nota continua salva.
- **`no_api_key`** → comportamento atual; detecção de alerta não é tentada.
- **Texto + tool call juntos** → mostra o texto e também propõe.
- **Usuário cancela o chip (Esc/Não)** → nada gravado; nota (se houver) permanece.
- **Modelo ignora a ferramenta** → degrada para resposta de chat normal. É o
  gatilho do fallback documentado abaixo.

## Risco e fallback

Tool calling sobre o endpoint de **streaming** do OpenRouter exige acumular
`tool_calls` em deltas. É padrão e factível, mas se um modelo específico se
comportar de forma inconsistente, o fallback é instruir o modelo (via system
prompt) a emitir um bloco estruturado (ex.: ```` ```alert {…} ``` ````) dentro da
resposta, que o backend extrai e converte em proposta. Construir tool calling
primeiro; manter o fallback documentado.

## Testes

- `alerts_test.go` — `CreateProposed`: one-shot futuro gravado; one-shot passado →
  `past`; recorrente passado → gravado; mensagem vazia + fallback para título da
  nota; vínculo `noteId`. Puro, sem IA.
- `notes_test.go` — capture com `Completer` falso retornando bloco `alert` →
  `CaptureResult.Alert` preenchido; capture sem tempo → `Alert` nil; nota sempre
  criada nos dois casos.
- `ai/client_test.go` — parser de streaming junta deltas de `tool_calls.arguments`
  em uma call completa; stream só com conteúdo não produz tool calls (comportamento
  inalterado).
- `chat_test.go` — streamer falso emitindo tool call → `ChatService` emite
  `alert:proposed` e não emite `chat:done` de texto naquele turno; stream normal
  ainda emite `chat:done`.

## Critérios de aceite

- `/note comprar leite amanhã 9h` cria a nota e, após Sim no chip, um alerta
  vinculado à nota.
- `/note ideia: comprar leite` cria só a nota, sem chip.
- `me lembra de X amanhã 9h` (sem comando) mostra o chip e, após Sim, cria o
  alerta.
- Mensagem comum sem intenção de lembrete continua sendo só uma resposta de chat.
- `/alerta` segue funcionando como antes.
- Nenhum alerta é criado sem confirmação explícita.
