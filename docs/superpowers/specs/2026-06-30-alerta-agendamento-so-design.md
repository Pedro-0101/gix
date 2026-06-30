# Agendamento de alertas no SO (toast agendado do Windows)

Data: 2026-06-30
Plataforma alvo: **Windows** (somente, por enquanto)

## Problema

Hoje o alerta só vira notificação do SO **enquanto o desktop está aberto e
conectado**. O servidor (`gix-server`) dispara o alerta server-side e empurra um
push SSE; o frontend (`services.ts` → `showDeliveryToast`) ergue o toast nativo
via `notifications.New()` do Wails — que só faz toast **imediato**. Logo, se o
app estiver fechado ou sem rede no horário, o toast do SO não aparece (fica só na
outbox do servidor, `alert_deliveries`, para catch-up quando o app reabrir).

O desktop virou um **canal fino** (fase 3): não tem mais DB local de alertas,
nem lógica de recorrência (ambos vivem no `gix-server`).

## Objetivo

Ao **criar** um alerta (e ao abrir o app / sincronizar), o desktop **arma um
toast agendado no Windows** para a próxima ocorrência. O Windows dispara na hora
exata **mesmo com o app fechado e sem rede**. O servidor continua sendo a fonte
da verdade e a fonte multi-canal (WhatsApp/Telegram/Android); o agendamento no SO
é **complementar**, com de-duplicação para não notificar duas vezes.

Decisões de produto que orientam o design:

- **Complementar + dedup** — servidor segue como autoridade; o SO é cobertura
  para "app fechado". Os dois coexistem sem tocar 2x.
- **Somente Windows** agora — macOS/Linux seguem só com push do servidor.
- **Interop Go nativo** via `winrt-go` (sem PowerShell, sem helper C#/.NET).
- **Armar só a próxima ocorrência** (o `fireAt` atual do servidor) — sem
  reintroduzir cálculo de recorrência no thin client.

Fora de escopo: mudar o servidor; recorrência local de N ocorrências; macOS;
Linux; ações ricas no toast (botões além do clique padrão de abrir o overlay).

## Conceito central: "ocorrência" e sua chave

Cada disparo é uma **ocorrência** identificada por
`K = alertId:fireAtUnix` (id do alerta + instante de disparo em epoch). Essa
chave é carregada nos dois caminhos (toast agendado do SO e push SSE) e é o que
permite de-duplicar.

```
criar/sincronizar ──► arma ScheduledToastNotification(tag=alertId, group=fireAtUnix)
disparo (app fechado) ──► Windows ergue o toast sozinho
disparo (app aberto)  ──► push SSE ergue o toast; o agendado iminente foi desarmado
```

## Arquitetura

Tudo no **desktop (gix)**, Windows-only atrás de build tags. O `gix-server` não
muda.

```
list()/create/done/cancel/snooze  ──►  AlertSchedulerService (Go, agnóstico)
                                              │ Reconcile (diff testável)
                                              ▼ interface Notifier
                                    winnotify (winrt-go, //go:build windows)
                                              │ Arm/Cancel/ListArmed
                                              ▼
                                        Windows Notification Platform
```

### Backend (Go)

**`internal/app/winnotify/scheduler_windows.go`** (`//go:build windows`)

Wrapper fino sobre `winrt-go`. Implementa a interface `Notifier`:

- `Arm(occ Occurrence) error` — cria um `ScheduledToastNotification` agendado
  para `occ.FireAt` (instante absoluto), via o `ToastNotifier` da AUMID do app.
  Define `Tag = strconv(alertId)` e `Group = strconv(fireAtUnix)`; o conteúdo do
  toast traz `occ.Message`. Inicializa COM/WinRT sob demanda.
- `Cancel(key Key) error` — remove de `GetScheduledToastNotifications()` o toast
  cujo `Tag`/`Group` casa com a chave.
- `ListArmed() ([]Key, error)` — devolve as chaves atualmente agendadas, para a
  reconciliação fazer o diff.

`Occurrence{ AlertID int64; FireAt time.Time; Message string }` e
`Key{ AlertID int64; FireAtUnix int64 }` ficam num arquivo agnóstico
(`winnotify/types.go`) para o stub e o serviço compartilharem.

**`internal/app/winnotify/scheduler_stub.go`** (`//go:build !windows`)

Implementação no-op do `Notifier` (Arm/Cancel devolvem nil, ListArmed devolve
vazio), para o projeto compilar e testar em qualquer plataforma.

**`internal/app/alertsched.go`** — `AlertSchedulerService` (serviço Wails)

Camada de orquestração agnóstica de plataforma, com a lógica **testável** de
diff. Depende da interface `Notifier` (injetada: real no Windows, falso nos
testes). Métodos expostos ao frontend:

- `Reconcile(alerts []Alert) error` — calcula o conjunto desejado (uma `Key` por
  alerta futuro, a partir de `fireAt`), compara com `Notifier.ListArmed()`:
  arma o que falta, cancela o que sobra (alerta sumiu ou `fireAt` mudou).
- `ArmOne(alert Alert) error` — arma a próxima ocorrência de um alerta (no-op se
  `fireAt` no passado).
- `CancelOne(alertID int64) error` — cancela qualquer ocorrência agendada do
  alerta.

Erros do `Notifier` (COM/WinRT indisponível) são **logados e engolidos**: nunca
propagam a ponto de bloquear a criação de alerta. Degrada para push-only.

**`internal/app/shell.go`** — registra `AlertSchedulerService` junto dos demais
serviços (`cfgSvc`, `notifSvc`, `tokenSvc`). No Windows injeta o `winnotify`
real; nas outras plataformas, o stub.

### Frontend (TS)

**`frontend/src/lib/alertSchedule.ts`** — centraliza as chamadas ao serviço e a
lógica de dedup. Funções:

- `reconcile(alerts)` → `AlertSchedulerService.Reconcile(alerts)`.
- `armOne(alert)` / `cancelOne(id)`.
- `markSurfaced(key)` / `wasSurfaced(key)` — registro local pequeno (em memória +
  espelho opcional no config dir) das ocorrências já exibidas, para o dedup.
- `keyOf(alertId, fireAt)` — monta `alertId:fireAtUnix`.

**`frontend/src/api/services.ts`** — engata o lib nos pontos de ciclo de vida:

- Após `create` / `createProposed` / `createForNote` com sucesso → `armOne`.
- Em `done` / `cancel` → `cancelOne`. Em `snooze` → `cancelOne` + `armOne` (novo
  `fireAt`).
- No handler de push (`startPush`, evento `alert`): antes de `showDeliveryToast`,
  `markSurfaced(keyOf(d.alertId, d.fireAt))` e `cancelOne` daquela ocorrência (o
  push assumiu).

**`frontend/src/App.tsx`** (ou onde o boot pós-login mora) — após o `list()`
inicial, chamar `reconcile(alerts)` uma vez. Idem ao reconectar o push após uma
lacuna longa.

## Dedup — autoridade por estado

- **App aberto + conectado:** o **push é a autoridade**. Ocorrências dentro da
  janela iminente têm o toast do SO desarmado; quem mostra é o push/card in-app.
- **App fechado / desconectado:** o **toast agendado do SO é a autoridade**.
- **Rede de segurança:** `tag`/`group` iguais nos dois caminhos → se o agendado
  já disparou e o push chega logo em seguida, o Windows **substitui** a mesma
  entrada na Central de Ações em vez de empilhar uma segunda.
- **Reconciliar em toda mudança de estado:** abrir o app, criar/editar/concluir/
  cancelar/adiar, e no shutdown gracioso (rearma o próximo para não deixar
  buraco).

## Recorrência (decisão de produto)

Armamos **exatamente uma ocorrência por alerta**: o `fireAt` atual que o servidor
devolve. Sem materializar N ocorrências nem cálculo de recorrência no desktop.
Depois que uma ocorrência recorrente dispara, o **servidor** avança o `fireAt`;
na próxima abertura do app, `list()` traz o novo `fireAt` e o `Reconcile` rearma.

**Limitação consciente:** com o app dias fechado, só a **próxima** ocorrência de
um recorrente dispara localmente; as seguintes ficam por conta do servidor (que
já as cobre). Aceitável no modelo complementar.

## Tratamento de erros e casos de borda

- **COM/WinRT falha na init** → loga e degrada para push-only; nunca bloqueia
  criar alerta.
- **`fireAt` no passado** → não arma.
- **Fuso/DST** → arma pelo **instante absoluto** (RFC3339), imune a DST.
- **Cap de toasts agendados do Windows** → armamos 1 por alerta ativo; contagem
  baixa, sem problema.
- **Race na borda:** se a rede cair exatamente no instante do disparo com o app
  aberto (toast iminente já desarmado), fica só o card in-app — janela pequena,
  aceita.
- **Múltiplos desktops na mesma conta** → cada um arma localmente; o dedup é
  por-máquina; sem conflito.
- **Identidade (AUMID):** toast agendado exige AUMID + atalho no Menu Iniciar. O
  toast imediato já funciona hoje, então a identidade existe — **risco a
  verificar**: confirmar a AUMID no build do instalador (`build/windows`). Dev/
  unpackaged pode não exibir toasts.

## Testes

- **`alertsched_test.go`** — `Reconcile`/`ArmOne`/`CancelOne` com um `Notifier`
  falso em memória: arma/cancela os conjuntos corretos; `fireAt` passado não
  arma; `fireAt` mudado cancela o antigo e arma o novo; alerta removido cancela.
  Go puro, sem WinRT.
- **Frontend** (`alertSchedule.test.ts`) — `create/done/cancel/snooze` chamam o
  serviço com a chave certa; push marca surfaced + cancela; `wasSurfaced`
  suprime toast duplicado; `keyOf` estável.
- **Manual** — criar alerta para +2 min, fechar o app, ativar modo avião →
  confirmar que o toast dispara offline e que reabrir o app não duplica.

## Critérios de aceite

- Criar um alerta para daqui a 2 min, fechar o app e tirar a rede → o toast do SO
  dispara no horário.
- Com o app aberto e conectado, o alerta dispara **uma** vez (push), sem toast
  duplicado do SO.
- `done`/`cancel` removem o toast agendado; `snooze` reagenda para o novo horário.
- Reabrir o app reconcilia o estado do SO com `list()` sem duplicar nem deixar
  alerta futuro sem armar.
- Em macOS/Linux, nada quebra: o stub no-op compila e o comportamento atual
  (push-only) permanece.
- Falha de WinRT não impede criar alerta (degrada para push-only).
