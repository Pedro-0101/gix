# Agendamento de alertas no SO (toast agendado do Windows) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Armar um toast agendado do Windows ao criar/sincronizar um alerta, para o lembrete disparar mesmo com o app fechado e sem rede, sem notificar em dobro com o push do servidor.

**Architecture:** Desktop (gix), Windows-only atrás de build tags. Um pacote `winnotify` (winrt-go no Windows, stub no-op nas outras plataformas) expõe `Arm/CancelByAlert/ListArmed`. Um serviço Wails `AlertSchedulerService` faz o diff testável (desejado vs armado) sobre a interface `Notifier`. O frontend chama o serviço nos pontos de ciclo de vida (criar/concluir/cancelar/adiar, boot) e desduplica o push contra o toast do SO. O `gix-server` não muda.

**Tech Stack:** Go 1.25, Wails v3 (`github.com/wailsapp/wails/v3`), `github.com/saltosystems/winrt-go` (+ `github.com/go-ole/go-ole`), TypeScript/React, Vitest.

## Global Constraints

- Tamanho de arquivo: ≤100 linhas (simples) / ≤300 (complexo) — ver AGENT.md.
- Go: rodar testes com `CGO_ENABLED=1 go test ./...` (msys2 gcc no PATH).
- Frontend antes de commitar: `npm run lint`, `npm run check:lines`, `npm run test:run`.
- Nunca editar `frontend/bindings/**` (gerado pelo Wails).
- Código nativo de notificação é **somente Windows**, atrás de `//go:build windows`; as demais plataformas usam o stub no-op e continuam só com push.
- Mensagens de commit seguem Conventional Commits (commitlint roda no commit-msg hook).
- Módulo Go: `gix`. Serviços Wails são structs simples registrados em `internal/app/shell.go` e expostos ao frontend via `import('../../bindings/gix/internal/app')`.

---

## File Structure

- `internal/app/winnotify/types.go` — tipos agnósticos (`Occurrence`, `Key`) e a interface `Notifier`. Compila em toda plataforma.
- `internal/app/winnotify/scheduler_stub.go` (`//go:build !windows`) — `Notifier` no-op + construtor `New()`.
- `internal/app/winnotify/scheduler_windows.go` (`//go:build windows`) — `Notifier` real via winrt-go + construtor `New()`.
- `internal/app/alertsched.go` — `AlertSchedulerService` (serviço Wails) com o diff testável.
- `internal/app/alertsched_test.go` — testes do serviço com `Notifier` falso.
- `internal/app/shell.go:70-83` — registra o serviço, injetando `winnotify.New()`.
- `frontend/src/lib/alertSchedule.ts` — `keyOf`, registro de "surfaced", forwarders ao serviço.
- `frontend/src/lib/alertSchedule.test.ts` — testes do lib.
- `frontend/src/api/services.ts:140-291` — engata o lib em create/done/cancel/snooze e no push.
- `frontend/src/App.tsx` — `reconcile(alerts)` após o `list()` de boot.

---

## Task 1: Pacote winnotify — tipos, interface e stub no-op

**Files:**
- Create: `internal/app/winnotify/types.go`
- Create: `internal/app/winnotify/scheduler_stub.go`
- Create: `internal/app/winnotify/scheduler_stub_test.go`

**Interfaces:**
- Produces:
  - `type Occurrence struct { AlertID int64; FireAt time.Time; Message string }`
  - `type Key struct { AlertID int64; FireAtUnix int64 }`
  - `type Notifier interface { Arm(Occurrence) error; CancelByAlert(alertID int64) error; ListArmed() ([]Key, error) }`
  - `func New() Notifier` (stub no-op fora de Windows)

- [ ] **Step 1: Escrever os tipos e a interface**

Create `internal/app/winnotify/types.go`:

```go
// Package winnotify arma toasts agendados do Windows para alertas, de modo que
// o lembrete dispare mesmo com o app fechado e offline. No Windows usa winrt-go;
// nas demais plataformas, um stub no-op (o push do servidor segue cobrindo).
package winnotify

import "time"

// Occurrence é um disparo concreto a ser agendado no SO.
type Occurrence struct {
	AlertID int64
	FireAt  time.Time
	Message string
}

// Key identifica uma ocorrência agendada (tag=alertID, group=fireAtUnix).
type Key struct {
	AlertID    int64
	FireAtUnix int64
}

// Notifier abstrai o agendamento nativo. Real no Windows, no-op fora dele.
type Notifier interface {
	Arm(occ Occurrence) error
	CancelByAlert(alertID int64) error
	ListArmed() ([]Key, error)
}
```

- [ ] **Step 2: Escrever o stub no-op**

Create `internal/app/winnotify/scheduler_stub.go`:

```go
//go:build !windows

package winnotify

// noopNotifier: fora do Windows não há agendamento nativo; o push do servidor
// continua sendo o único caminho.
type noopNotifier struct{}

func New() Notifier { return noopNotifier{} }

func (noopNotifier) Arm(Occurrence) error          { return nil }
func (noopNotifier) CancelByAlert(int64) error      { return nil }
func (noopNotifier) ListArmed() ([]Key, error)      { return nil, nil }
```

- [ ] **Step 3: Escrever o teste do stub**

Create `internal/app/winnotify/scheduler_stub_test.go`:

```go
//go:build !windows

package winnotify

import (
	"testing"
	"time"
)

func TestStubIsNoop(t *testing.T) {
	n := New()
	if err := n.Arm(Occurrence{AlertID: 1, FireAt: time.Now(), Message: "x"}); err != nil {
		t.Fatalf("Arm: %v", err)
	}
	if err := n.CancelByAlert(1); err != nil {
		t.Fatalf("CancelByAlert: %v", err)
	}
	got, err := n.ListArmed()
	if err != nil {
		t.Fatalf("ListArmed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListArmed = %v, quero vazio", got)
	}
}
```

- [ ] **Step 4: Rodar os testes**

Run: `CGO_ENABLED=1 go test ./internal/app/winnotify/...`
Expected: PASS (no Windows o `_test` do stub não roda, mas o pacote compila).

- [ ] **Step 5: Commit**

```bash
git add internal/app/winnotify/types.go internal/app/winnotify/scheduler_stub.go internal/app/winnotify/scheduler_stub_test.go
git commit -m "feat(winnotify): tipos, interface Notifier e stub no-op"
```

---

## Task 2: AlertSchedulerService — diff testável (Reconcile/ArmOne/CancelOne)

**Files:**
- Create: `internal/app/alertsched.go`
- Create: `internal/app/alertsched_test.go`

**Interfaces:**
- Consumes: `winnotify.{Occurrence, Key, Notifier}` (Task 1).
- Produces:
  - `type ScheduledAlert struct { ID int64; Message string; FireAt string; Status string }` (JSON tags `id`,`message`,`fireAt`,`status`)
  - `type AlertSchedulerService struct { ... }`
  - `func NewAlertSchedulerService(n winnotify.Notifier) *AlertSchedulerService`
  - `func (s *AlertSchedulerService) Reconcile(alerts []ScheduledAlert) error`
  - `func (s *AlertSchedulerService) ArmOne(a ScheduledAlert) error`
  - `func (s *AlertSchedulerService) CancelOne(alertID int64) error`

Regras: só status `"active"` e `FireAt` no futuro entram no conjunto desejado; `FireAt` é RFC3339; comparação por instante absoluto (Unix). `Reconcile` cancela o que sobrou ou mudou de horário e arma o que falta. `ArmOne` no passado é no-op. `CancelOne` = `CancelByAlert`.

- [ ] **Step 1: Escrever os testes com um Notifier falso**

Create `internal/app/alertsched_test.go`:

```go
package app

import (
	"testing"
	"time"

	"gix/internal/app/winnotify"
)

// fakeNotifier guarda o estado agendado em memória (uma ocorrência por alerta).
type fakeNotifier struct {
	armed map[int64]int64 // alertID -> fireAtUnix
}

func newFake() *fakeNotifier { return &fakeNotifier{armed: map[int64]int64{}} }

func (f *fakeNotifier) Arm(o winnotify.Occurrence) error {
	f.armed[o.AlertID] = o.FireAt.Unix()
	return nil
}
func (f *fakeNotifier) CancelByAlert(id int64) error { delete(f.armed, id); return nil }
func (f *fakeNotifier) ListArmed() ([]winnotify.Key, error) {
	out := make([]winnotify.Key, 0, len(f.armed))
	for id, u := range f.armed {
		out = append(out, winnotify.Key{AlertID: id, FireAtUnix: u})
	}
	return out, nil
}

func future(d time.Duration) string { return time.Now().Add(d).UTC().Format(time.RFC3339) }
func past(d time.Duration) string   { return time.Now().Add(-d).UTC().Format(time.RFC3339) }

func TestArmOneFuture(t *testing.T) {
	f := newFake()
	s := NewAlertSchedulerService(f)
	if err := s.ArmOne(ScheduledAlert{ID: 1, Message: "x", FireAt: future(time.Hour), Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.armed[1]; !ok {
		t.Fatal("esperava alerta 1 armado")
	}
}

func TestArmOnePastIsNoop(t *testing.T) {
	f := newFake()
	s := NewAlertSchedulerService(f)
	if err := s.ArmOne(ScheduledAlert{ID: 1, FireAt: past(time.Hour), Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if len(f.armed) != 0 {
		t.Fatalf("passado não deve armar: %v", f.armed)
	}
}

func TestCancelOne(t *testing.T) {
	f := newFake()
	f.armed[7] = 123
	s := NewAlertSchedulerService(f)
	if err := s.CancelOne(7); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.armed[7]; ok {
		t.Fatal("esperava alerta 7 cancelado")
	}
}

func TestReconcileArmsCancelsAndUpdates(t *testing.T) {
	f := newFake()
	f.armed[2] = 111            // será removido (não está mais na lista)
	f.armed[3] = 222            // mudou de horário -> cancela e rearma
	s := NewAlertSchedulerService(f)

	soon := future(time.Hour)
	alerts := []ScheduledAlert{
		{ID: 1, FireAt: soon, Status: "active"},          // novo -> arma
		{ID: 3, FireAt: future(2 * time.Hour), Status: "active"}, // mudou
		{ID: 4, FireAt: past(time.Minute), Status: "active"},     // passado -> ignora
		{ID: 5, FireAt: soon, Status: "done"},                    // não-ativo -> ignora
	}
	if err := s.Reconcile(alerts); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.armed[2]; ok {
		t.Fatal("alerta 2 deveria ter sido cancelado")
	}
	if _, ok := f.armed[1]; !ok {
		t.Fatal("alerta 1 deveria ter sido armado")
	}
	want3 := mustUnix(t, future(2*time.Hour))
	if f.armed[3] == 222 || abs(f.armed[3]-want3) > 2 {
		t.Fatalf("alerta 3 deveria ter rearmado para o novo horário, got %d", f.armed[3])
	}
	if _, ok := f.armed[4]; ok {
		t.Fatal("alerta 4 (passado) não deveria estar armado")
	}
	if _, ok := f.armed[5]; ok {
		t.Fatal("alerta 5 (done) não deveria estar armado")
	}
}

func mustUnix(t *testing.T, iso string) int64 {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t.Fatal(err)
	}
	return tm.Unix()
}
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
```

- [ ] **Step 2: Rodar para ver falhar**

Run: `CGO_ENABLED=1 go test ./internal/app/ -run 'Arm|Cancel|Reconcile' -v`
Expected: FAIL — `undefined: NewAlertSchedulerService` / `ScheduledAlert`.

- [ ] **Step 3: Implementar o serviço**

Create `internal/app/alertsched.go`:

```go
package app

import (
	"time"

	"gix/internal/app/winnotify"
)

// ScheduledAlert é a forma mínima que o frontend envia ao serviço (vinda de
// AlertsService.list()/create*). FireAt é RFC3339.
type ScheduledAlert struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
	FireAt  string `json:"fireAt"`
	Status  string `json:"status"`
}

// AlertSchedulerService arma/cancela toasts agendados do Windows espelhando os
// alertas do servidor. O diff (desejado vs armado) é testável; o I/O nativo fica
// atrás de winnotify.Notifier. Erros do Notifier são engolidos por chamador —
// agendar é best-effort e nunca bloqueia o fluxo de alertas.
type AlertSchedulerService struct {
	n winnotify.Notifier
}

func NewAlertSchedulerService(n winnotify.Notifier) *AlertSchedulerService {
	return &AlertSchedulerService{n: n}
}

// desired converte um ScheduledAlert numa Occurrence futura, ou (zero,false) se
// não-ativo ou no passado.
func desired(a ScheduledAlert) (winnotify.Occurrence, bool) {
	if a.Status != "active" {
		return winnotify.Occurrence{}, false
	}
	t, err := time.Parse(time.RFC3339, a.FireAt)
	if err != nil || !t.After(time.Now()) {
		return winnotify.Occurrence{}, false
	}
	return winnotify.Occurrence{AlertID: a.ID, FireAt: t, Message: a.Message}, true
}

// ArmOne arma a próxima ocorrência de um alerta (no-op se passado/não-ativo).
func (s *AlertSchedulerService) ArmOne(a ScheduledAlert) error {
	occ, ok := desired(a)
	if !ok {
		return nil
	}
	return s.n.Arm(occ)
}

// CancelOne remove qualquer ocorrência agendada do alerta.
func (s *AlertSchedulerService) CancelOne(alertID int64) error {
	return s.n.CancelByAlert(alertID)
}

// Reconcile alinha o conjunto agendado do SO com a lista de alertas: cancela o
// que sumiu ou mudou de horário e arma o que falta. Uma ocorrência por alerta.
func (s *AlertSchedulerService) Reconcile(alerts []ScheduledAlert) error {
	armed, err := s.n.ListArmed()
	if err != nil {
		return err
	}
	armedBy := make(map[int64]int64, len(armed))
	for _, k := range armed {
		armedBy[k.AlertID] = k.FireAtUnix
	}
	want := make(map[int64]winnotify.Occurrence)
	for _, a := range alerts {
		if occ, ok := desired(a); ok {
			want[a.ID] = occ
		}
	}
	for id, unix := range armedBy {
		w, ok := want[id]
		if !ok || w.FireAt.Unix() != unix {
			if err := s.n.CancelByAlert(id); err != nil {
				return err
			}
		}
	}
	for id, occ := range want {
		if unix, ok := armedBy[id]; !ok || unix != occ.FireAt.Unix() {
			if err := s.n.Arm(occ); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Rodar até passar**

Run: `CGO_ENABLED=1 go test ./internal/app/ -run 'Arm|Cancel|Reconcile' -v`
Expected: PASS (4 testes).

- [ ] **Step 5: Commit**

```bash
git add internal/app/alertsched.go internal/app/alertsched_test.go
git commit -m "feat(alertsched): serviço de diff para agendar alertas no SO"
```

---

## Task 3: winnotify Windows — toast agendado via winrt-go

**Files:**
- Create: `internal/app/winnotify/scheduler_windows.go`
- Modify: `go.mod` / `go.sum` (adiciona `github.com/saltosystems/winrt-go` e `github.com/go-ole/go-ole`)

**Interfaces:**
- Consumes: `winnotify.{Occurrence, Key, Notifier}` (Task 1).
- Produces: `func New() Notifier` (Windows) — implementação real.

> **Verificação obrigatória antes de codar:** os nomes exatos dos símbolos do
> `winrt-go` (`ToastNotificationManagerCreateToastNotifierWithId`,
> `NewScheduledToastNotification`, `AddToSchedule`, `GetScheduledToastNotifications`,
> `RemoveFromSchedule`, setters `SetTag`/`SetGroup`) devem ser confirmados via
> `go doc github.com/saltosystems/winrt-go/windows/ui/notifications` após o
> `go get`. Ajuste o casing/assinatura ao que o pacote expõe. A estrutura,
> o XML e a conversão de tempo abaixo estão corretos; só os identificadores
> podem variar de casing.

- [ ] **Step 1: Adicionar as dependências**

Run:
```bash
go get github.com/saltosystems/winrt-go@latest
go get github.com/go-ole/go-ole@latest
```
Expected: `go.mod`/`go.sum` atualizados.

- [ ] **Step 2: Confirmar a API real**

Run: `go doc github.com/saltosystems/winrt-go/windows/ui/notifications | grep -iE 'Scheduled|Schedule|CreateToastNotifier'`
Expected: lista os símbolos usados no Step 3 (anote o casing exato).

- [ ] **Step 3: Implementar o Notifier do Windows**

Create `internal/app/winnotify/scheduler_windows.go` (ajuste os símbolos ao que o Step 2 mostrou):

```go
//go:build windows

package winnotify

import (
	"fmt"
	"strconv"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go/windows/data/xml/dom"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/ui/notifications"
)

// aumid deve casar com o AppUserModelID que o instalador (build/windows) registra
// para o atalho do gix; é o mesmo usado pelo toast imediato do Wails. Confirmar
// no Step de verificação da Task 4.
const aumid = "com.gix.app"

// ticks de 100ns entre 1601-01-01 (época WinRT) e 1970-01-01 (Unix).
const winEpochDiffSec = 11644473600

type winNotifier struct{}

func New() Notifier { return winNotifier{} }

// toDateTime converte um instante Go para o foundation.DateTime do WinRT
// (UniversalTime = ticks de 100ns desde 1601-01-01 UTC).
func toDateTime(t time.Time) foundation.DateTime {
	ut := (t.Unix()+winEpochDiffSec)*int64(time.Second/100) + int64(t.Nanosecond())/100
	return foundation.DateTime{UniversalTime: ut}
}

func toastXML(message string) string {
	return `<toast><visual><binding template="ToastGeneric">` +
		`<text>gix</text><text>` + xmlEscape(message) + `</text>` +
		`</binding></visual></toast>`
}

func xmlEscape(s string) string {
	r := []rune{}
	for _, c := range s {
		switch c {
		case '&':
			r = append(r, []rune("&amp;")...)
		case '<':
			r = append(r, []rune("&lt;")...)
		case '>':
			r = append(r, []rune("&gt;")...)
		default:
			r = append(r, c)
		}
	}
	return string(r)
}

func notifier() (*notifications.ToastNotifier, error) {
	if err := ole.RoInitialize(1); err != nil {
		return nil, fmt.Errorf("RoInitialize: %w", err)
	}
	return notifications.ToastNotificationManagerCreateToastNotifierWithId(aumid)
}

func (winNotifier) Arm(occ Occurrence) error {
	n, err := notifier()
	if err != nil {
		return err
	}
	doc, err := dom.NewXmlDocument()
	if err != nil {
		return err
	}
	if err := doc.LoadXml(toastXML(occ.Message)); err != nil {
		return err
	}
	st, err := notifications.NewScheduledToastNotification(doc, toDateTime(occ.FireAt))
	if err != nil {
		return err
	}
	if err := st.SetTag(strconv.FormatInt(occ.AlertID, 10)); err != nil {
		return err
	}
	if err := st.SetGroup(strconv.FormatInt(occ.FireAt.Unix(), 10)); err != nil {
		return err
	}
	return n.AddToSchedule(st)
}

func (winNotifier) CancelByAlert(alertID int64) error {
	n, err := notifier()
	if err != nil {
		return err
	}
	list, err := n.GetScheduledToastNotifications()
	if err != nil {
		return err
	}
	size, err := list.GetSize()
	if err != nil {
		return err
	}
	tag := strconv.FormatInt(alertID, 10)
	for i := uint32(0); i < size; i++ {
		st, err := list.GetAt(i)
		if err != nil {
			return err
		}
		t, err := st.GetTag()
		if err != nil {
			return err
		}
		if t == tag {
			if err := n.RemoveFromSchedule(st); err != nil {
				return err
			}
		}
	}
	return nil
}

func (winNotifier) ListArmed() ([]Key, error) {
	n, err := notifier()
	if err != nil {
		return nil, err
	}
	list, err := n.GetScheduledToastNotifications()
	if err != nil {
		return nil, err
	}
	size, err := list.GetSize()
	if err != nil {
		return nil, err
	}
	out := make([]Key, 0, size)
	for i := uint32(0); i < size; i++ {
		st, err := list.GetAt(i)
		if err != nil {
			return nil, err
		}
		tag, err := st.GetTag()
		if err != nil {
			return nil, err
		}
		grp, err := st.GetGroup()
		if err != nil {
			return nil, err
		}
		id, _ := strconv.ParseInt(tag, 10, 64)
		unix, _ := strconv.ParseInt(grp, 10, 64)
		out = append(out, Key{AlertID: id, FireAtUnix: unix})
	}
	return out, nil
}
```

- [ ] **Step 4: Compilar para Windows**

Run: `CGO_ENABLED=1 go build ./internal/app/winnotify/...`
Expected: compila sem erro. Se algum símbolo winrt-go diferir, corrigir o casing/assinatura conforme o Step 2 e recompilar.

- [ ] **Step 5: Verificação manual (smoke)**

Escreva um `main` temporário (ou um teste `//go:build windows` com `t.Skip` por padrão) que chama `New().Arm(Occurrence{AlertID:1, FireAt: time.Now().Add(2*time.Minute), Message:"teste gix"})`, feche tudo e confirme o toast em ~2 min. Depois `ListArmed()` deve listar a chave; `CancelByAlert(1)` deve removê-la. Remova o `main`/skip antes do commit.

- [ ] **Step 6: Commit**

```bash
git add internal/app/winnotify/scheduler_windows.go go.mod go.sum
git commit -m "feat(winnotify): agenda toast nativo do Windows via winrt-go"
```

---

## Task 4: Registrar o AlertSchedulerService no shell

**Files:**
- Modify: `internal/app/shell.go:68-83`

**Interfaces:**
- Consumes: `NewAlertSchedulerService` (Task 2), `winnotify.New` (Tasks 1/3).

- [ ] **Step 1: Instanciar e registrar o serviço**

Em `internal/app/shell.go`, após o bloco do `tokenSvc` (linha ~74) adicione:

```go
	// Agendador de alertas no SO: arma toasts agendados do Windows (no-op em
	// outras plataformas) para o lembrete disparar com o app fechado/offline.
	alertSchedSvc := NewAlertSchedulerService(winnotify.New())
```

E inclua o serviço na slice `Services` (junto de cfgSvc/notifSvc/tokenSvc):

```go
			application.NewService(alertSchedSvc),
```

Adicione o import `"gix/internal/app/winnotify"` no bloco de imports.

- [ ] **Step 2: Compilar**

Run: `CGO_ENABLED=1 go build ./...`
Expected: compila sem erro.

- [ ] **Step 3: Confirmar a AUMID (resolve o risco do spec)**

Run: `grep -rniE "appusermodelid|aumid|com\.gix|gix" build/windows`
Expected: localizar o AppUserModelID do instalador/atalho. Ajustar a `const aumid` em `scheduler_windows.go` para casar exatamente. Recompilar (`CGO_ENABLED=1 go build ./...`).

- [ ] **Step 4: Commit**

```bash
git add internal/app/shell.go internal/app/winnotify/scheduler_windows.go
git commit -m "feat(shell): registra AlertSchedulerService e alinha AUMID"
```

---

## Task 5: Frontend — lib/alertSchedule.ts (chave, surfaced, forwarders)

**Files:**
- Create: `frontend/src/lib/alertSchedule.ts`
- Create: `frontend/src/lib/alertSchedule.test.ts`

**Interfaces:**
- Consumes: binding `AlertSchedulerService` via `import('../../bindings/gix/internal/app')` (gerado pelo Wails; chamadas com optional-chaining degradam se ausente, como `TokenService` em `client.ts`).
- Produces (de `alertSchedule.ts`):
  - `keyOf(alertId: number, fireAt: string): string` → `"<id>:<unixSeconds>"`
  - `markSurfaced(key: string): void`
  - `wasSurfaced(key: string): boolean`
  - `reconcile(alerts: ScheduledAlertInput[]): Promise<void>`
  - `armOne(a: ScheduledAlertInput): Promise<void>`
  - `cancelOne(alertId: number): Promise<void>`
  - `type ScheduledAlertInput = { id: number; message: string; fireAt: string; status: string }`

- [ ] **Step 1: Escrever os testes**

Create `frontend/src/lib/alertSchedule.test.ts`:

```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { keyOf, markSurfaced, wasSurfaced, _resetSurfaced } from './alertSchedule'

describe('keyOf', () => {
  it('monta id:unixSeconds a partir de RFC3339', () => {
    expect(keyOf(7, '1970-01-01T00:00:10Z')).toBe('7:10')
  })
  it('é estável para o mesmo instante em offsets diferentes', () => {
    expect(keyOf(1, '2026-06-30T12:00:00Z')).toBe(keyOf(1, '2026-06-30T09:00:00-03:00'))
  })
})

describe('surfaced set', () => {
  beforeEach(() => _resetSurfaced())
  it('marca e consulta', () => {
    expect(wasSurfaced('1:10')).toBe(false)
    markSurfaced('1:10')
    expect(wasSurfaced('1:10')).toBe(true)
  })
})
```

- [ ] **Step 2: Rodar para ver falhar**

Run: `cd frontend && npm run test:run -- alertSchedule`
Expected: FAIL — módulo `./alertSchedule` não existe.

- [ ] **Step 3: Implementar o lib**

Create `frontend/src/lib/alertSchedule.ts`:

```ts
// alertSchedule centraliza o agendamento de alertas no SO (toast agendado do
// Windows). As funções de side-effect chamam o AlertSchedulerService do Wails
// via import dinâmico (no-op fora de Windows / se o binding não existir). O
// registro "surfaced" desduplica o push contra o toast do SO.

export type ScheduledAlertInput = {
  id: number
  message: string
  fireAt: string
  status: string
}

// keyOf identifica uma ocorrência por id + instante absoluto (segundos Unix),
// imune a fuso/representação. Casa com winnotify.Key (tag:group) no Go.
export function keyOf(alertId: number, fireAt: string): string {
  const unix = Math.floor(new Date(fireAt).getTime() / 1000)
  return `${alertId}:${unix}`
}

const surfaced = new Set<string>()

export function markSurfaced(key: string): void {
  surfaced.add(key)
}
export function wasSurfaced(key: string): boolean {
  return surfaced.has(key)
}
// _resetSurfaced é só para testes.
export function _resetSurfaced(): void {
  surfaced.clear()
}

async function svc(): Promise<any> {
  try {
    const mod: any = await import('../../bindings/gix/internal/app')
    return mod?.AlertSchedulerService ?? null
  } catch {
    return null
  }
}

export async function reconcile(alerts: ScheduledAlertInput[]): Promise<void> {
  try {
    await (await svc())?.Reconcile?.(alerts)
  } catch {
    /* best-effort: o push do servidor segue cobrindo */
  }
}

export async function armOne(a: ScheduledAlertInput): Promise<void> {
  try {
    await (await svc())?.ArmOne?.(a)
  } catch {
    /* best-effort */
  }
}

export async function cancelOne(alertId: number): Promise<void> {
  try {
    await (await svc())?.CancelOne?.(alertId)
  } catch {
    /* best-effort */
  }
}
```

- [ ] **Step 4: Rodar até passar + lint + linhas**

Run: `cd frontend && npm run test:run -- alertSchedule && npm run lint && npm run check:lines`
Expected: PASS / sem erros de lint / dentro do limite de linhas.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/alertSchedule.ts frontend/src/lib/alertSchedule.test.ts
git commit -m "feat(frontend): lib de agendamento de alertas no SO + dedup key"
```

---

## Task 6: Frontend — engatar lib em services.ts e no boot

**Files:**
- Modify: `frontend/src/api/services.ts:140-291`
- Modify: `frontend/src/App.tsx` (efeito de boot pós-login, após `AlertsService.list()`)

**Interfaces:**
- Consumes: `armOne`, `cancelOne`, `reconcile`, `keyOf`, `markSurfaced` (Task 5); `Alert`/`CreateAlertResult`/`Delivery` (já existentes em `types.ts`).

Notas de mapeamento:
- `CreateAlertResult` tem `{ alertId, message, fireAtLocal, recurrence }` (sem `fireAt` RFC3339 nem `status`). Para armar após criar, montar `{ id: res.alertId, message: res.message, fireAt: <ISO>, status: 'active' }`. O `fireAtLocal` é local formatado — **não** serve para `keyOf`. Portanto, após qualquer create, em vez de tentar reconstruir o ISO, chamar `AlertsService.list()` e `reconcile(...)` (fonte da verdade, traz `fireAt` RFC3339 e `status`). Isso mantém uma única regra: **toda mutação de alerta → reconcile**.

- [ ] **Step 1: Adicionar um helper de reconcile a partir do servidor em services.ts**

Em `frontend/src/api/services.ts`, importe no topo:

```ts
import { reconcile, cancelOne, keyOf, markSurfaced } from '../lib/alertSchedule'
```

E adicione, logo após o objeto `AlertsService`:

```ts
// syncAlertSchedule busca a lista atual e reconcilia o agendamento no SO. É o
// único ponto que arma/cancela após mutações — fireAt/status vêm do servidor.
export async function syncAlertSchedule(): Promise<void> {
  try {
    const alerts = await AlertsService.list()
    await reconcile(
      alerts.map((a) => ({ id: a.id, message: a.message, fireAt: a.fireAt, status: a.status })),
    )
  } catch {
    /* best-effort */
  }
}
```

- [ ] **Step 2: Disparar o sync após cada mutação**

Ainda em `services.ts`, encadeie `void syncAlertSchedule()` no retorno das mutações de `AlertsService`. Reescreva o objeto assim (mantendo as assinaturas):

```ts
export const AlertsService = {
  list(): Promise<Alert[]> {
    return request<Alert[]>('GET', '/v1/alerts')
  },
  async create(text: string): Promise<CreateAlertResult> {
    const res = await request<CreateAlertResult>('POST', '/v1/alerts/parse', { body: { text } })
    void syncAlertSchedule()
    return res
  },
  async createProposed(message: string, fireAt: string, recurrence: string, noteId: number | null): Promise<CreateAlertResult> {
    const res = await request<CreateAlertResult>('POST', '/v1/alerts', { body: { message, fireAt, recurrence, noteId } })
    void syncAlertSchedule()
    return res
  },
  async createForNote(noteId: number, whenText: string): Promise<CreateAlertResult> {
    const res = await request<CreateAlertResult>('POST', `/v1/notes/${noteId}/alert`, { body: { text: whenText } })
    void syncAlertSchedule()
    return res
  },
  async done(id: number): Promise<void> {
    await request<void>('POST', `/v1/alerts/${id}/done`)
    void cancelOne(id)
  },
  async cancel(id: number): Promise<void> {
    await request<void>('POST', `/v1/alerts/${id}/cancel`)
    void cancelOne(id)
  },
  async snooze(id: number, minutes: number): Promise<void> {
    await request<void>('POST', `/v1/alerts/${id}/snooze`, { body: { minutes } })
    void syncAlertSchedule()
  },
}
```

> Atenção ao limite de linhas: se `services.ts` passar do teto, mova
> `syncAlertSchedule` para `lib/alertSchedule.ts` (recebendo `AlertsService.list`
> por parâmetro) e importe-a aqui.

- [ ] **Step 3: Dedup no handler de push**

No `startPush`, no callback do evento `alert` (hoje em `services.ts:257-261`), antes de `showDeliveryToast(d)` marque a ocorrência e cancele o toast do SO daquela ocorrência (o push assumiu):

```ts
            if (eventName !== 'alert') return
            const d = JSON.parse(data) as Delivery
            emitAlertFired({ id: d.alertId ?? 0, message: d.message, noteId: d.noteId })
            if (d.alertId) {
              markSurfaced(keyOf(d.alertId, d.fireAt))
              void cancelOne(d.alertId)
            }
            // Toast nativo: best-effort (o serviço pode não estar registrado).
            try { showDeliveryToast(d) } catch { /* sem toast, segue */ }
```

- [ ] **Step 4: Reconcile no boot**

Em `frontend/src/App.tsx`, no efeito que roda após login/boot e já chama `AlertsService.list()` (ou crie um efeito dedicado que roda uma vez quando autenticado), adicione a chamada:

```ts
    void syncAlertSchedule()
```

importando-a de `../api/services`. Deve rodar uma vez quando o usuário está autenticado (mesma condição que abre o push).

- [ ] **Step 5: Lint, linhas e testes**

Run: `cd frontend && npm run lint && npm run check:lines && npm run test:run`
Expected: tudo verde.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/api/services.ts frontend/src/App.tsx
git commit -m "feat(frontend): reconcilia agendamento no SO em mutações, boot e dedup do push"
```

---

## Validação manual final (após todas as tasks)

1. Criar alerta para +2 min (`/alerta teste em 2 minutos`), fechar o app, ativar modo avião → o toast do Windows dispara no horário.
2. Com o app aberto e conectado, o alerta dispara **uma** vez (push) — sem toast duplicado do SO.
3. `done`/`cancel` num alerta futuro removem o toast agendado (confirmar via `ListArmed` no smoke da Task 3, ou que não dispara).
4. `snooze` reagenda para o novo horário.
5. Reabrir o app reconcilia sem duplicar nem deixar alerta futuro sem armar.
6. Em macOS/Linux: `CGO_ENABLED=1 go build ./...` e `go test ./...` passam (stub no-op), comportamento atual preservado.

---

## Self-review (preenchido)

- **Cobertura do spec:** problema/objetivo → Tasks 1-6 + validação; arquitetura (winnotify + AlertSchedulerService) → Tasks 1-4; frontend (lib + services + boot) → Tasks 5-6; recorrência "uma ocorrência" → `desired()`/`Reconcile` (Task 2); dedup por estado → push cancela+marca (Task 6), `tag/group` iguais (Tasks 3/5); ciclo de vida (create/done/cancel/snooze/boot/shutdown) → Task 6 (shutdown coberto por "armado permanece até cancelar"); erros (COM falha, passado, DST, cap, AUMID) → Tasks 2/3/4; testes → Tasks 1,2,5 + validação manual.
- **Placeholders:** nenhum "TBD"; o único ponto aberto-por-natureza é o casing de símbolos winrt-go, com Step de verificação explícito (`go doc`) e código concreto a ajustar.
- **Consistência de tipos:** `Notifier{Arm,CancelByAlert,ListArmed}`, `Occurrence`, `Key`, `ScheduledAlert{id,message,fireAt,status}` e `keyOf → "id:unix"` (= `Key.tag:group`) batem entre Go e TS em todas as tasks.
