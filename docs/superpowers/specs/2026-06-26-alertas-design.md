# Design — Criação e notificação de alertas

Data: 2026-06-26
Status: aprovado (seções 1–4 validadas em conversa; 5–6 consolidadas a pedido do usuário)

## Objetivo

Permitir que o usuário crie **alertas** (lembretes com data/hora) por comando em
linguagem natural e a partir de notas existentes, e seja **notificado** quando a
hora chega — via toast nativo do SO com a janela overlay como fallback/destino de
clique. Suporta alertas **pontuais e recorrentes**, com gestão (listar, cancelar,
adiar, concluir).

Casos de uso (definidos com o usuário):
- `/alerta ligar pro médico amanhã às 9h` → cria lembrete pontual.
- `/alerta toda segunda 8h academia` → cria lembrete recorrente.
- A partir de uma nota selecionada na NotesView → cria alerta vinculado à nota.

Não-objetivos (v1): RRULE/iCal completo; condições/triggers por busca; múltiplos
canais (e-mail, etc.); fuso configurável manualmente (usa o fuso do SO).

## Arquitetura geral

Mesmo stack do resto do app: backend Go (serviços Wails v3) + SQLite
(`internal/db`), frontend React. Novo serviço `AlertsService`
(`internal/app/alerts.go`) espelhando `NotesService`. Agendamento por **polling**
no backend (decisão do usuário — approach A), robusto a app reiniciado.

## Seção 1 — Modelo de dados

Nova tabela em `internal/db/db.go` (criada no `Open`, junto das demais):

```sql
CREATE TABLE IF NOT EXISTS alerts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    message     TEXT NOT NULL,                    -- texto do alerta (ou título da nota)
    note_id     INTEGER,                          -- NULL, ou nota de origem
    fire_at     DATETIME NOT NULL,                -- próximo disparo, em UTC
    recurrence  TEXT NOT NULL DEFAULT '',         -- '' = pontual; senão JSON da regra
    status      TEXT NOT NULL DEFAULT 'pending',  -- pending | done | cancelled
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_alerts_due ON alerts(status, fire_at);
```

Decisões:
- `fire_at` em **UTC**; convertido para hora local só na exibição.
- `note_id` é vínculo fraco (sem FK forçada, como `messages`). Se a nota for
  excluída, o alerta sobrevive com seu `message`; ao clicar, se a nota não existir
  mais, mostra só o texto.
- `recurrence` como JSON mínimo e fechado, gerado pela IA:
  ```json
  {"freq":"daily|weekly|monthly|yearly","interval":1,"weekday":"mon","time":"09:00"}
  ```
  `weekday` só para `weekly`; `interval` = "a cada N". Sem RRULE completo (YAGNI).
- `status`: pontual vira `done` ao disparar; recorrente segue `pending` com novo
  `fire_at`; cancelar → `cancelled` (soft) e excluir → DELETE físico.

Tipo Go `db.Alert{ ID, Message, NoteID *int64, FireAt time.Time, Recurrence string,
Status string, CreatedAt string }`.

Métodos novos no `db`:
- `CreateAlert(a Alert) (int64, error)`
- `ListAlerts(statuses ...string) ([]Alert, error)` — ordenado por `fire_at` asc
- `DueAlerts(now time.Time) ([]Alert, error)` — `status='pending' AND fire_at <= now`
- `GetAlert(id) (Alert, error)`
- `UpdateAlertFireAt(id, fireAt time.Time) error`
- `SetAlertStatus(id, status string) error`
- `DeleteAlert(id) error`

## Seção 2 — Scheduler (backend, polling)

`AlertsService` recebe `cfg *ConfigService`, `db *db.Database`, `newClient
func(apiKey) Completer`, `emit func(name string, data any)`, `onShow func()`,
e um `notifier` (interface fina sobre o NotificationService do Wails — ver Seção 3).

Loop iniciado em `shell.go` como goroutine (junto do warm-up de embeddings),
encerrado no shutdown via `context`:

```
tick imediato no boot, depois a cada pollInterval (30s):
    due = db.DueAlerts(now)
    para cada alerta em due:
        dispara(alerta)                       // toast + evento (Seção 3)
        se recorrente:
            next = proximoFireAt(rule, alerta.FireAt, now)
            db.UpdateAlertFireAt(alerta.ID, next)   // continua pending
        senão:
            db.SetAlertStatus(alerta.ID, "done")
```

Pontos-chave:
- **Catch-up grátis:** app fechado no vencimento → dispara no 1º tick após o boot.
- Recorrente vencido várias vezes (ex.: diário, 3 dias fechado): `proximoFireAt`
  avança em loop até passar de `now` → dispara **uma vez** e reagenda para o
  próximo futuro (sem spam de N disparos atrasados).
- **Tick imediato no boot** (não espera 30s).
- `proximoFireAt(rule, last, now) time.Time` é **função pura**, alvo principal de
  testes: toda a aritmética de recorrência (somar dia/semana/mês/ano, casar
  `weekday` e `time`) vive aqui.
- Concorrência: ticker numa única goroutine; SQLite serializa acessos. UI só
  escreve no DB; o próximo tick reflete. Sem canais de sinalização (custo aceito
  do approach A).
- `pollInterval = 30 * time.Second` (constante).

## Seção 3 — Notificação (toast + overlay)

Ao disparar, dois caminhos em paralelo (para nunca perder o aviso):

1. **Toast nativo** via `NotificationService` do Wails v3 (registrado em
   `shell.go`):
   - `RegisterNotificationCategory` no boot com duas actions: `"Adiar 10 min"` e
     `"Concluir"`.
   - `SendNotificationWithActions`: título = `message`, corpo = hora/recorrência.
   - `OnNotificationResponse(callback)`:
     - clique no corpo → `onShow()` (mostra overlay) + emite `alert:open` com o id.
     - action "Adiar 10 min" → `Snooze(id, 10)`.
     - action "Concluir" → `Done(id)`.

2. **Evento `alert:fired`** sempre emitido (independe do toast), payload
   `{id, message, noteId}`:
   - Overlay aberto → card do alerta inline na hora.
   - **Fallback:** toast no Windows exige app **instalado** (atalho no Menu Iniciar
     com AppUserModelID); em `wails dev` ou se `SendNotification` retornar erro, o
     backend chama `onShow()` direto. "toast + overlay" degrada para "só overlay"
     sem código de detecção extra.

`showMain` (hoje closure local no `Run`) é exposto ao serviço via callback
injetado `onShow func()`, no mesmo estilo do `emit`.

`notifier` é uma interface fina (`SendNotificationWithActions`,
`RegisterNotificationCategory`) para o serviço ser testável com um fake.

## Seção 4 — Criação (comando + a partir de nota)

a) Comando `/alerta <texto>` — builtin `frontend/src/commands/builtins/alert.ts`
(aliases `alarme`, `lembrete`, `al`), molde do `/note`:
- sem arg → abre a AlertsView.
- com texto → `AlertsService.Create(text)`.

`AlertsService.Create(text)` (1 chamada de IA): monta prompt injetando **sempre** o
contexto temporal/local (a data interna do modelo é não-confiável — requisito do
usuário):
- data/hora locais atuais: `now.Format("2006-01-02 15:04 (Monday)")`
- fuso: nome + offset (`time.Now().Zone()`)
- locale/idioma: `config.Language` (ex.: `pt-BR`)

IA retorna JSON fechado:
```json
{"message":"<texto curto>","fire_at":"2026-06-26T15:00:00-03:00","recurrence":{...}|null}
```
Backend parseia ISO com offset, converte para UTC, valida futuro (passado e não
recorrente → erro amigável), `db.CreateAlert(...)`.

Resultado pro frontend: `{status, alertId, message, fireAtLocal, recurrenceLabel}`.
`status`: `created`, `no_api_key`, `unparseable`, `past`, `error`.

b) Alerta a partir de nota — na NotesView, com nota selecionada, ação (tecla `a`
ou botão "Criar alerta") abre mini-input pedindo **só o quando** → 
`AlertsService.CreateForNote(noteID, whenText)`: IA parseia só horário/recorrência;
`message` default = título da nota; `note_id` = a nota.

Helper interno único `parseWhen(text, now, tz, locale) → (fireAt, recurrence, err)`
faz a chamada de IA + parse — fonte de verdade compartilhada por `Create` e
`CreateForNote`.

## Seção 5 — Gestão (listar / cancelar / adiar / concluir)

View `AlertsView.tsx` (aberta por `/alerta` sem arg; alias `/alertas`), padrão da
NotesView:
- Lista `pending` por `fire_at` asc: `message`, hora local, rótulo de recorrência,
  ícone 📝 se vier de nota.
- Navegação por setas (reaproveita o padrão da lista de notas).
- Ações por item: **Excluir** (`DeleteAlert`), **Adiar** (`fire_at += 10min`),
  **Concluir** (`status=done`).
- Seção secundária colapsada com `done`/`cancelled` recentes, com opção de limpar.

`AlertsService` expõe: `List()` (pending+done), `Cancel(id)`, `Snooze(id, minutes)`,
`Done(id)`. As mesmas funções são chamadas pelas actions do toast (fonte única).

Semântica de `Done(id)`: para alerta **pontual**, marca `status='done'` (já estava
`done` se disparado pelo scheduler — idempotente). Para alerta **recorrente**, é um
**no-op de agendamento**: a ocorrência atual só é dispensada; o scheduler já
reagendou o próximo `fire_at` e o alerta segue `pending`. Ou seja, "Concluir" no
toast de um recorrente não cancela a recorrência (para isso, usar Excluir/Cancel na
view).

## Seção 6 — Frontend, i18n e testes

- Eventos (`frontend/src/lib/events.ts`): `onAlertFired(cb)`, `onAlertOpen(cb)`
  envolvendo `Events.On('alert:fired'|'alert:open', …)`, estilo `onWindowShown`.
- `App.tsx`: assina `alert:fired` (card inline se aberto) e `alert:open` (troca de
  view e foca alerta/nota).
- Bindings Wails regenerados (`AlertsService.*`).
- i18n: chaves `cmd_alert_desc`, `alert_created`, `alert_unparseable`, `alert_past`,
  `alert_fired`, `alert_recurrence_*`, etc., em pt e demais línguas de `i18n.ts`.
- Testes:
  - Go: `proximoFireAt` (pontual passado; diário/semanal/mensal/anual; catch-up
    multi-período; casamento de weekday/time) — função pura, alvo principal.
    `parseWhen`/`Create`/`CreateForNote` com `Completer` fake (JSON válido,
    inválido, data passada). Métodos de `db` (CRUD + `DueAlerts`) em DB temporário,
    como `db_test.go`.
  - Frontend: `alert.test.ts` (sem arg abre view; status created/no_api_key/
    unparseable) com `ctx` mockado, como `note.test.ts`.

## Registro

Convenções seguidas do projeto: serviços em `internal/app`, DB em `internal/db`,
builtins em `frontend/src/commands/builtins`, eventos em `frontend/src/lib/events.ts`,
i18n em `frontend/src/i18n.ts`. Injeção de dependências (`Completer`, `notifier`,
`emit`, `onShow`) para testabilidade.
