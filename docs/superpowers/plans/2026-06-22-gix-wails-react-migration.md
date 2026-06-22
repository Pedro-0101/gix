# Migração da UI do gix para Wails v3 + React — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Substituir a UI Fyne do gix por um app Wails v3 com frontend React, mantendo o núcleo Go (`ai`, `config`, `db`) intacto e destravando liberdade visual (Tailwind + design tokens prontos para temas).

**Architecture:** O backend Go expõe três serviços (Chat, Config, History) via bindings do Wails v3; o streaming da IA é entregue ao frontend por eventos (`chat:delta/usage/done/error`). O frontend React de janela única troca entre views (Chat/Settings/History). Janela frameless always-on-top escondida no boot, com system tray e hotkey global (reaproveitada) para mostrar/centralizar/focar.

**Tech Stack:** Go, Wails v3 (alpha), React + Vite + TypeScript, Tailwind v4, react-markdown, `@wailsio/runtime`, modernc.org/sqlite (já em uso).

## Global Constraints

- **Módulo Go:** `gix` (não renomear). Núcleo `internal/ai`, `internal/config`, `internal/db` permanece **intacto** — nenhuma alteração de assinatura.
- **Wails v3:** fixar versão exata no `go.mod` e no CLI; é alpha, validar API contra a doc instalada.
- **Janela única:** settings e history são views React, nunca janelas OS separadas.
- **Tokens obrigatórios:** nenhum componente usa cor/raio/espaçamento literal — tudo via token Tailwind mapeado para CSS variable. `Config.Theme` seleciona o conjunto de tokens (`data-theme`).
- **Comportamento preservado:** duplo-Espaço abre/centraliza/foca; duplo-Esc esconde e cancela streaming; fechar a janela esconde (não encerra); `/new` reinicia a conversa; rótulos pt/en.
- **Commits:** Conventional Commits (o repo tem commitlint + lefthook rodando `go test`/`go vet` no pre-commit). Mensagens em pt, escopo entre parênteses.

---

## Visão de arquivos

**Mantidos intactos:** `internal/ai/*`, `internal/config/*`, `internal/db/*`.

**Criados:**
- `internal/hotkey/` — hook global movido de `internal/ui/hotkey*.go` (renomeia o pacote para `hotkey`, exporta `Start`).
- `internal/app/chat.go` — `ChatService` (streaming via eventos).
- `internal/app/config.go` — `ConfigService`.
- `internal/app/history.go` — `HistoryService`.
- `internal/app/shell.go` — bootstrap: janela frameless, tray, hotkey, esc-hide.
- `internal/app/*_test.go` — testes dos serviços.
- `frontend/` — app React (Vite + TS + Tailwind).

**Removidos no fim:** todo `internal/ui/*` (Fyne).

**Modificados:** `cmd/gix/main.go` (bootstrap Wails), `go.mod`, `.goreleaser.yaml`, `.lefthook.yml` (build do frontend).

---

## FASE 0 — Pré-requisitos e scaffold

### Task 0: Instalar toolchain Wails v3 e validar ambiente

**Files:** nenhum commit de código; valida ferramentas.

- [ ] **Step 1: Instalar o CLI do Wails v3**

Run:
```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```
Expected: binário `wails3` no `$(go env GOPATH)/bin`.

- [ ] **Step 2: Rodar o doctor**

Run:
```bash
wails3 doctor
```
Expected: relatório sem erros bloqueantes (Go, Node, npm, WebView2 no Windows presentes). Se algo faltar (ex.: Node), instalar antes de prosseguir.

- [ ] **Step 3: Confirmar Node/npm**

Run:
```bash
node --version && npm --version
```
Expected: Node ≥ 20, npm presente.

Sem commit — é checagem de ambiente.

---

### Task 1: Scaffold do app Wails v3 + React dentro do repo

**Files:**
- Create: `frontend/**` (template React)
- Create: `main.go` temporário do template (será substituído na Task 13 por `cmd/gix/main.go`)
- Modify: `go.mod` (adiciona dependência `github.com/wailsapp/wails/v3`)

**Interfaces:**
- Produces: estrutura `frontend/` com Vite+React+TS e o runtime `@wailsio/runtime`; um app Wails mínimo que abre uma janela.

- [ ] **Step 1: Gerar o template num diretório temporário irmão**

Run:
```bash
cd .. && wails3 init -n gix-scaffold -t react && cd gix
```
Expected: pasta `../gix-scaffold` criada com `frontend/`, `main.go`, `Taskfile.yml`, `build/`.

- [ ] **Step 2: Copiar `frontend/` e arquivos de bootstrap para o repo**

Run:
```bash
cp -r ../gix-scaffold/frontend ./frontend
cp ../gix-scaffold/main.go ./main.go
cp ../gix-scaffold/Taskfile.yml ./Taskfile.yml
cp -r ../gix-scaffold/build ./build
```
Expected: `frontend/`, `main.go`, `Taskfile.yml`, `build/` presentes no repo.

- [ ] **Step 3: Mesclar dependências Go**

Run:
```bash
go mod tidy
```
Expected: `go.mod` ganha `github.com/wailsapp/wails/v3` e o build resolve.

- [ ] **Step 4: Instalar deps do frontend**

Run:
```bash
cd frontend && npm install && cd ..
```
Expected: `frontend/node_modules` criado, sem erros.

- [ ] **Step 5: Rodar o app de scaffold uma vez**

Run:
```bash
wails3 dev
```
Expected: abre uma janela do template React. Fechar a janela para encerrar.

- [ ] **Step 6: Remover o diretório temporário**

Run:
```bash
rm -rf ../gix-scaffold
```

- [ ] **Step 7: Ajustar `.gitignore`**

Garantir que contenha:
```
frontend/node_modules
frontend/dist
build/bin
```

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "chore(wails): scaffold inicial do app Wails v3 + React"
```

---

## FASE 1 — Serviços Go (TDD)

### Task 2: Mover o hook de hotkey para `internal/hotkey`

**Files:**
- Create: `internal/hotkey/hotkey.go` (de `internal/ui/hotkey.go`)
- Create: `internal/hotkey/hotkey_windows.go` (de `internal/ui/hotkey_windows.go`)
- Create: `internal/hotkey/hotkey_linux.go`, `internal/hotkey/hotkey_other.go` (dos equivalentes em `internal/ui`)
- Modify: nenhuma alteração ainda em `internal/ui` (será deletado na Task 14)

**Interfaces:**
- Produces:
  - `func Start(openKey string, intervalMs int, onTrigger func())` — inicia o listener global; chama `onTrigger` no duplo-pressionar de `openKey`.
  - `func Apply(openKey string, intervalMs int)` — reaplica config em runtime.
  - `type DoublePressDetector` com `Press()` — reutilizável pelo frontend-bridge se necessário.

- [ ] **Step 1: Criar `internal/hotkey/hotkey.go`**

Copiar o conteúdo de `internal/ui/hotkey.go`, trocando `package ui` por `package hotkey`, e exportar a API:

```go
package hotkey

import (
	"runtime"
	"time"
)

// Start inicia o listener de hotkey global para o SO atual.
func Start(openKey string, intervalMs int, onTrigger func()) {
	switch runtime.GOOS {
	case "windows":
		go startWindowsHook(openKey, intervalMs, onTrigger)
	case "linux":
		go startLinuxHook(openKey, intervalMs, onTrigger)
	}
}

type DoublePressDetector struct {
	lastPress time.Time
	fn        func()
	interval  time.Duration
}

func (d *DoublePressDetector) Press() {
	now := time.Now()
	if !d.lastPress.IsZero() && now.Sub(d.lastPress) <= d.interval {
		d.lastPress = time.Time{}
		if d.fn != nil {
			d.fn()
		}
		return
	}
	d.lastPress = now
}
```

- [ ] **Step 2: Criar `internal/hotkey/hotkey_windows.go`**

Copiar `internal/ui/hotkey_windows.go`, trocar `package ui` por `package hotkey`, ajustar a assinatura de `startWindowsHook` e `Apply`:

```go
//go:build windows

package hotkey

// (mantém os blocos syscall/var/const exatamente como em internal/ui/hotkey_windows.go)

func startWindowsHook(openKey string, intervalMs int, fn func()) {
	winHookKeyCode = vkCodeMap[openKey]
	winHookDetector = &DoublePressDetector{
		fn:       fn,
		interval: time.Duration(intervalMs) * time.Millisecond,
	}
	hookCallback = syscall.NewCallback(winLowLevelKeyboardProc)
	// ... resto idêntico ao original (SetWindowsHookExW + loop GetMessageW)
}

func startLinuxHook(openKey string, intervalMs int, fn func()) {}

func Apply(openKey string, intervalMs int) {
	winHookKeyCode = vkCodeMap[openKey]
	if winHookDetector != nil {
		winHookDetector.interval = time.Duration(intervalMs) * time.Millisecond
	}
}
```

Trocar o tipo de `winHookDetector` para `*DoublePressDetector`.

- [ ] **Step 3: Criar stubs `hotkey_linux.go` e `hotkey_other.go`**

Copiar de `internal/ui/hotkey_linux.go` e `internal/ui/hotkey_other.go`, trocando `package ui` por `package hotkey` e adaptando assinaturas (`startLinuxHook(openKey string, intervalMs int, fn func())`, e um `Apply` no-op para não-windows via build tag).

Adicionar em `hotkey_other.go` (build `!windows && !linux`) e `hotkey_linux.go` uma versão no-op de `Apply` para o build compilar fora do Windows:
```go
//go:build !windows
package hotkey
func Apply(openKey string, intervalMs int) {}
```

- [ ] **Step 4: Verificar compilação**

Run:
```bash
go build ./internal/hotkey/
```
Expected: sem erros.

- [ ] **Step 5: Commit**

```bash
git add internal/hotkey
git commit -m "refactor(hotkey): extrair hook global para pacote internal/hotkey"
```

---

### Task 3: `ConfigService`

**Files:**
- Create: `internal/app/config.go`
- Test: `internal/app/config_test.go`

**Interfaces:**
- Consumes: `config.Load() *config.Config`, `(*config.Config).Save() error`, `config.Models []string`.
- Produces:
  - `type ConfigService struct{ ... }`
  - `func NewConfigService() *ConfigService`
  - `func (s *ConfigService) Get() *config.Config`
  - `func (s *ConfigService) Save(c config.Config) error`
  - `func (s *ConfigService) Models() []string`
  - `func (s *ConfigService) Current() *config.Config` (acesso interno thread-safe pelo ChatService/shell)
  - `func (s *ConfigService) OnSave(fn func(*config.Config))` — registra callback (usado pelo shell para reaplicar hotkey)

- [ ] **Step 1: Escrever o teste que falha**

`internal/app/config_test.go`:
```go
package app

import (
	"testing"

	"gix/internal/config"
)

func TestConfigServiceSaveUpdatesCurrent(t *testing.T) {
	s := NewConfigService()
	c := *s.Get()
	c.SystemPrompt = "novo prompt"

	called := false
	s.OnSave(func(cfg *config.Config) { called = true })

	if err := s.Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if s.Current().SystemPrompt != "novo prompt" {
		t.Fatalf("Current não atualizou: %q", s.Current().SystemPrompt)
	}
	if !called {
		t.Fatal("callback OnSave não foi chamado")
	}
}

func TestConfigServiceModels(t *testing.T) {
	s := NewConfigService()
	if len(s.Models()) == 0 {
		t.Fatal("Models vazio")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run:
```bash
go test ./internal/app/ -run TestConfigService -v
```
Expected: FAIL (pacote/identificadores inexistentes).

- [ ] **Step 3: Implementar `internal/app/config.go`**

```go
package app

import (
	"sync"

	"gix/internal/config"
)

type ConfigService struct {
	mu       sync.RWMutex
	cfg      *config.Config
	onSave   []func(*config.Config)
}

func NewConfigService() *ConfigService {
	return &ConfigService{cfg: config.Load()}
}

func (s *ConfigService) Get() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := *s.cfg
	return &c
}

func (s *ConfigService) Current() *config.Config {
	return s.Get()
}

func (s *ConfigService) Models() []string {
	return config.Models
}

func (s *ConfigService) OnSave(fn func(*config.Config)) {
	s.mu.Lock()
	s.onSave = append(s.onSave, fn)
	s.mu.Unlock()
}

func (s *ConfigService) Save(c config.Config) error {
	if err := c.Save(); err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg = &c
	cbs := append([]func(*config.Config){}, s.onSave...)
	s.mu.Unlock()
	for _, fn := range cbs {
		fn(&c)
	}
	return nil
}
```

- [ ] **Step 4: Rodar e ver passar**

Run:
```bash
go test ./internal/app/ -run TestConfigService -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/config.go internal/app/config_test.go
git commit -m "feat(app): ConfigService com Get/Save/Models e callback OnSave"
```

---

### Task 4: `HistoryService`

**Files:**
- Create: `internal/app/history.go`
- Test: `internal/app/history_test.go`

**Interfaces:**
- Consumes: `*db.Database` com `ListConversations()`, `GetMessages(id)`, `DeleteConversation(id)`, e `db.Open(path)`.
- Produces:
  - `type HistoryService struct{ db *db.Database }`
  - `func NewHistoryService(database *db.Database) *HistoryService`
  - `func (s *HistoryService) List() ([]db.Conversation, error)`
  - `func (s *HistoryService) Messages(id int64) ([]db.Message, error)`
  - `func (s *HistoryService) Delete(id int64) error`
  (Todos retornam vazio/no-op se `db == nil`.)

- [ ] **Step 1: Escrever o teste que falha**

`internal/app/history_test.go`:
```go
package app

import (
	"path/filepath"
	"testing"

	"gix/internal/db"
)

func TestHistoryServiceListAndDelete(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	id, err := d.CreateConversation("titulo", "model-x")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := d.AddMessage(id, "user", "oi"); err != nil {
		t.Fatalf("add: %v", err)
	}

	s := NewHistoryService(d)

	convs, err := s.List()
	if err != nil || len(convs) != 1 {
		t.Fatalf("List = %v, %v", convs, err)
	}
	msgs, err := s.Messages(id)
	if err != nil || len(msgs) != 1 || msgs[0].Content != "oi" {
		t.Fatalf("Messages = %v, %v", msgs, err)
	}
	if err := s.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	convs, _ = s.List()
	if len(convs) != 0 {
		t.Fatalf("esperava 0 após delete, veio %d", len(convs))
	}
}

func TestHistoryServiceNilDB(t *testing.T) {
	s := NewHistoryService(nil)
	if convs, err := s.List(); err != nil || convs != nil {
		t.Fatalf("List com db nil = %v, %v", convs, err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run:
```bash
go test ./internal/app/ -run TestHistoryService -v
```
Expected: FAIL.

- [ ] **Step 3: Implementar `internal/app/history.go`**

```go
package app

import "gix/internal/db"

type HistoryService struct {
	db *db.Database
}

func NewHistoryService(database *db.Database) *HistoryService {
	return &HistoryService{db: database}
}

func (s *HistoryService) List() ([]db.Conversation, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.db.ListConversations()
}

func (s *HistoryService) Messages(id int64) ([]db.Message, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.db.GetMessages(id)
}

func (s *HistoryService) Delete(id int64) error {
	if s.db == nil {
		return nil
	}
	return s.db.DeleteConversation(id)
}
```

- [ ] **Step 4: Rodar e ver passar**

Run:
```bash
go test ./internal/app/ -run TestHistoryService -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/history.go internal/app/history_test.go
git commit -m "feat(app): HistoryService com List/Messages/Delete"
```

---

### Task 5: `ChatService` — streaming via eventos (TDD com fakes)

**Files:**
- Create: `internal/app/chat.go`
- Test: `internal/app/chat_test.go`

**Interfaces:**
- Consumes: `config` (Config, ModelPrices), `db.Database`, `ai.Message`, `ai.Usage`, `ConfigService.Current()`.
- Produces:
  - `type Emitter func(name string, data any)`
  - `type Streamer interface { Stream(ctx context.Context, model string, msgs []ai.Message, onDelta func(string)) (*ai.Usage, error) }` (satisfeito por `*ai.Client`)
  - `type ChatService struct{ ... }`
  - `func NewChatService(cfg *ConfigService, database *db.Database, emit Emitter, newClient func(apiKey string) Streamer) *ChatService`
  - `func (s *ChatService) Send(text string)` — async; emite `chat:delta` (string), `chat:usage` (`UsagePayload`), `chat:done` (`DonePayload`), `chat:error` (string)
  - `func (s *ChatService) Cancel()`
  - `func (s *ChatService) NewConversation()`
  - tipos de payload: `type UsagePayload struct{ Tokens int; Cost float64 }`, `type DonePayload struct{ Content string }`
- Eventos emitidos (nomes exatos): `"chat:delta"`, `"chat:usage"`, `"chat:done"`, `"chat:error"`.

- [ ] **Step 1: Escrever o teste que falha**

`internal/app/chat_test.go`:
```go
package app

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gix/internal/ai"
	"gix/internal/db"
)

type fakeStreamer struct {
	deltas []string
	usage  *ai.Usage
}

func (f *fakeStreamer) Stream(ctx context.Context, model string, msgs []ai.Message, onDelta func(string)) (*ai.Usage, error) {
	for _, d := range f.deltas {
		onDelta(d)
	}
	return f.usage, nil
}

func TestChatServiceSendEmitsSequence(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	var mu sync.Mutex
	events := map[string]int{}
	var doneContent string
	emit := func(name string, data any) {
		mu.Lock()
		defer mu.Unlock()
		events[name]++
		if name == "chat:done" {
			doneContent = data.(DonePayload).Content
		}
	}

	cfgSvc := NewConfigService()
	cur := cfgSvc.Current()
	cur.APIKey = "k" // garante ResolveAPIKey != ""
	_ = cfgSvc.Save(*cur)

	fake := &fakeStreamer{deltas: []string{"Olá", " mundo"}, usage: &ai.Usage{TotalTokens: 5, PromptTokens: 2, CompletionTokens: 3}}
	s := NewChatService(cfgSvc, d, emit, func(string) Streamer { return fake })

	s.Send("oi")

	// Send é async; aguardar o done.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := events["chat:done"]
		mu.Unlock()
		if done > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if events["chat:delta"] != 2 {
		t.Fatalf("esperava 2 deltas, veio %d", events["chat:delta"])
	}
	if events["chat:done"] != 1 {
		t.Fatalf("esperava 1 done, veio %d", events["chat:done"])
	}
	if doneContent != "Olá mundo" {
		t.Fatalf("conteúdo final = %q", doneContent)
	}
	if events["chat:usage"] != 1 {
		t.Fatalf("esperava 1 usage, veio %d", events["chat:usage"])
	}
	convs, _ := d.ListConversations()
	if len(convs) != 1 {
		t.Fatalf("esperava 1 conversa persistida, veio %d", len(convs))
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run:
```bash
go test ./internal/app/ -run TestChatService -v
```
Expected: FAIL.

- [ ] **Step 3: Implementar `internal/app/chat.go`**

```go
package app

import (
	"context"
	"strings"
	"sync"

	"gix/internal/ai"
	"gix/internal/config"
	"gix/internal/db"
)

type Emitter func(name string, data any)

type Streamer interface {
	Stream(ctx context.Context, model string, msgs []ai.Message, onDelta func(string)) (*ai.Usage, error)
}

type UsagePayload struct {
	Tokens int     `json:"tokens"`
	Cost   float64 `json:"cost"`
}

type DonePayload struct {
	Content string `json:"content"`
}

type ChatService struct {
	cfg       *ConfigService
	db        *db.Database
	emit      Emitter
	newClient func(apiKey string) Streamer

	mu         sync.Mutex
	convID     int64
	history    []ai.Message
	streaming  bool
	cancelFunc context.CancelFunc
	gen        uint64
	tokens     int
	cost       float64
}

func NewChatService(cfg *ConfigService, database *db.Database, emit Emitter, newClient func(apiKey string) Streamer) *ChatService {
	return &ChatService{cfg: cfg, db: database, emit: emit, newClient: newClient}
}

func (s *ChatService) NewConversation() {
	s.mu.Lock()
	s.convID = 0
	s.history = nil
	s.gen++
	s.tokens = 0
	s.cost = 0
	s.mu.Unlock()
}

func (s *ChatService) Cancel() {
	s.mu.Lock()
	cancel := s.cancelFunc
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *ChatService) Send(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	s.mu.Lock()
	if s.streaming {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		s.emit("chat:error", "no_api_key")
		return
	}

	s.mu.Lock()
	if s.convID == 0 && s.db != nil {
		if id, err := s.db.CreateConversation(db.ExtractTitle(text), cfg.Model); err == nil {
			s.convID = id
		}
	}
	cid := s.convID
	s.history = append(s.history, ai.Message{Role: "user", Content: text})
	msgs := make([]ai.Message, 0, len(s.history)+1)
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		msgs = append(msgs, ai.Message{Role: "system", Content: cfg.SystemPrompt})
	}
	msgs = append(msgs, s.history...)
	s.streaming = true
	gen := s.gen
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	s.mu.Unlock()

	if s.db != nil && cid != 0 {
		_ = s.db.AddMessage(cid, "user", text)
	}

	go func() {
		client := s.newClient(apiKey)
		var sb strings.Builder
		usage, streamErr := client.Stream(ctx, cfg.Model, msgs, func(delta string) {
			sb.WriteString(delta)
			s.emit("chat:delta", delta)
		})
		full := sb.String()

		s.mu.Lock()
		s.streaming = false
		s.cancelFunc = nil
		if usage != nil {
			s.tokens += usage.TotalTokens
			if p, ok := config.ModelPrices[cfg.Model]; ok {
				s.cost += p.CalculateCost(usage.PromptTokens, usage.CompletionTokens)
			}
		}
		tokens, cost := s.tokens, s.cost
		s.mu.Unlock()

		s.emit("chat:usage", UsagePayload{Tokens: tokens, Cost: cost})

		switch {
		case streamErr != nil && ctx.Err() == context.Canceled:
			if full != "" {
				s.persist(cid, gen, full)
			}
		case streamErr != nil:
			s.emit("chat:error", streamErr.Error())
		default:
			if full == "" {
				full = "(sem resposta)"
			}
			s.persist(cid, gen, full)
			s.emit("chat:done", DonePayload{Content: full})
		}
	}()
}

func (s *ChatService) persist(cid int64, gen uint64, full string) {
	if s.db != nil && cid != 0 {
		_ = s.db.AddMessage(cid, "assistant", full)
	}
	s.mu.Lock()
	if s.gen == gen {
		s.history = append(s.history, ai.Message{Role: "assistant", Content: full})
	}
	s.mu.Unlock()
}
```

- [ ] **Step 4: Rodar e ver passar**

Run:
```bash
go test ./internal/app/ -run TestChatService -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/chat.go internal/app/chat_test.go
git commit -m "feat(app): ChatService com streaming via eventos e persistencia"
```

---

## FASE 2 — Bootstrap Wails (janela, serviços, tray, hotkey)

### Task 6: `shell.go` — montar app, registrar serviços e abrir janela

**Files:**
- Create: `internal/app/shell.go`
- Modify: `cmd/gix/main.go`
- Modify: `main.go` (raiz, do scaffold) — apagar, pois o entrypoint volta a ser `cmd/gix/main.go`

**Interfaces:**
- Consumes: `application.New`, `application.NewService`, `app.Window.NewWithOptions`, `app.SystemTray.New`, `app.Event.Emit`, `hotkey.Start/Apply`, `config.Load`, `db.New`.
- Produces: `func Run()` — bootstrap completo; substitui `ui.Run()`.

- [ ] **Step 1: Implementar `internal/app/shell.go`**

```go
package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"gix/internal/ai"
	"gix/internal/db"
	"gix/internal/hotkey"
)

var (
	wailsApp *application.App
	mainWin  *application.WebviewWindow
)

func Run() {
	database, err := db.New()
	if err != nil {
		database = nil
	}

	cfgSvc := NewConfigService()
	histSvc := NewHistoryService(database)

	emit := func(name string, data any) {
		if wailsApp != nil {
			wailsApp.Event.Emit(name, data)
		}
	}
	chatSvc := NewChatService(cfgSvc, database, emit,
		func(apiKey string) Streamer { return ai.New(apiKey) })

	wailsApp = application.New(application.Options{
		Name: "gix",
		Services: []application.Service{
			application.NewService(cfgSvc),
			application.NewService(histSvc),
			application.NewService(chatSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets), // assets embed da Task 12
		},
	})

	mainWin = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:         "gix",
		Width:         640,
		Height:        480,
		Frameless:     true,
		AlwaysOnTop:   true,
		Hidden:        true,
		DisableResize: true,
	})

	// Fechar a janela esconde em vez de encerrar.
	mainWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		chatSvc.Cancel()
		mainWin.Hide()
		e.Cancel()
	})

	// System tray.
	tray := wailsApp.SystemTray.New()
	tray.SetIcon(trayIcon) // trayIcon embed da Task 12
	menu := wailsApp.NewMenu()
	menu.Add("Exibir").OnClick(func(_ *application.Context) { showMain() })
	menu.Add("Sair").OnClick(func(_ *application.Context) { wailsApp.Quit() })
	tray.SetMenu(menu)

	// Hotkey global: duplo-Espaço mostra/centraliza/foca.
	cur := cfgSvc.Current()
	hotkey.Start(cur.OpenKey, cur.OpenIntervalMs, func() {
		showMain()
	})
	cfgSvc.OnSave(func(c *config.Config) {
		hotkey.Apply(c.OpenKey, c.OpenIntervalMs)
	})

	if err := wailsApp.Run(); err != nil {
		panic(err)
	}

	if database != nil {
		database.Close()
	}
}

func showMain() {
	if mainWin == nil {
		return
	}
	mainWin.Show()
	mainWin.Center()
	mainWin.Focus()
}
```

> Nota de execução: o import de `config` é necessário no callback `OnSave`. Adicionar `"gix/internal/config"` ao bloco de imports. Validar contra a doc instalada os nomes exatos: `WebviewWindowOptions.Hidden`, `DisableResize`, `events.Common.WindowClosing`, `SystemTray.New`, `Menu.Add(...).OnClick`. Ajustar se a versão alpha divergir.

- [ ] **Step 2: Reescrever `cmd/gix/main.go`**

```go
package main

import (
	"gix/internal/app"
	"gix/internal/config"
)

func main() {
	config.LoadDotEnv()
	app.Run()
}
```

- [ ] **Step 3: Apagar o `main.go` da raiz (scaffold)**

Run:
```bash
rm -f main.go
```

- [ ] **Step 4: Verificar compilação (sem assets ainda — esperado falhar só por `assets`/`trayIcon`)**

Run:
```bash
go build ./internal/app/ 2>&1 | head
```
Expected: erros apenas sobre `assets` e `trayIcon` indefinidos — resolvidos na Task 12. Demais símbolos devem resolver.

- [ ] **Step 5: Commit**

```bash
git add internal/app/shell.go cmd/gix/main.go
git commit -m "feat(app): bootstrap Wails com servicos, janela frameless, tray e hotkey"
```

---

## FASE 3 — Frontend React

### Task 7: Configurar Tailwind v4 + design tokens (base de temas)

**Files:**
- Modify: `frontend/package.json` (deps tailwind)
- Create: `frontend/src/styles/tokens.css`
- Modify: `frontend/src/index.css` (ou equivalente do template)
- Modify: `frontend/vite.config.ts` (plugin tailwind v4)

**Interfaces:**
- Produces: classes utilitárias mapeadas a tokens (`bg-surface`, `text-fg`, `bg-bubble`, `rounded-card`, `font-mono`) e dois temas via `[data-theme="light|dark"]`.

- [ ] **Step 1: Instalar Tailwind v4**

Run:
```bash
cd frontend && npm install -D tailwindcss @tailwindcss/vite && cd ..
```

- [ ] **Step 2: Adicionar o plugin no `frontend/vite.config.ts`**

Importar e registrar:
```ts
import tailwindcss from '@tailwindcss/vite'
// ...
export default defineConfig({
  plugins: [react(), tailwindcss()],
})
```

- [ ] **Step 3: Criar `frontend/src/styles/tokens.css`**

```css
@import "tailwindcss";

@theme {
  --font-mono: Consolas, "Courier New", monospace;
  --radius-card: 12px;
}

:root, [data-theme="light"] {
  --color-bg: #ffffff;
  --color-surface: #f4f4f5;
  --color-bubble: #e9e9ee;
  --color-fg: #18181b;
  --color-muted: #6b7280;
  --color-accent: #2563eb;
}

[data-theme="dark"] {
  --color-bg: #18181b;
  --color-surface: #27272a;
  --color-bubble: #3f3f46;
  --color-fg: #fafafa;
  --color-muted: #a1a1aa;
  --color-accent: #60a5fa;
}

@theme inline {
  --color-bg: var(--color-bg);
  --color-surface: var(--color-surface);
  --color-bubble: var(--color-bubble);
  --color-fg: var(--color-fg);
  --color-muted: var(--color-muted);
  --color-accent: var(--color-accent);
}
```

- [ ] **Step 4: Importar tokens no entrypoint CSS**

Em `frontend/src/main.tsx` (ou `index.tsx`), garantir `import './styles/tokens.css'` e remover o CSS antigo do template.

- [ ] **Step 5: Verificar que o dev server sobe**

Run:
```bash
cd frontend && npm run dev
```
Expected: Vite sobe sem erro de Tailwind. Encerrar (Ctrl+C).

- [ ] **Step 6: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/vite.config.ts frontend/src/styles/tokens.css frontend/src/main.tsx
git commit -m "feat(frontend): Tailwind v4 com design tokens e temas light/dark"
```

---

### Task 8: Gerar bindings e camada de eventos tipada

**Files:**
- Create: `frontend/bindings/**` (gerado)
- Create: `frontend/src/lib/events.ts`

**Interfaces:**
- Consumes: serviços Go registrados (Task 6).
- Produces:
  - bindings em `frontend/bindings/gix/internal/app/*`
  - `events.ts` exportando helpers tipados: `onChatDelta`, `onChatUsage`, `onChatDone`, `onChatError` e constantes de nome de evento.

- [ ] **Step 1: Gerar bindings**

Run:
```bash
wails3 generate bindings
```
Expected: `frontend/bindings` criado com módulos para `ChatService`, `ConfigService`, `HistoryService`.

- [ ] **Step 2: Criar `frontend/src/lib/events.ts`**

```ts
import { Events } from '@wailsio/runtime'

export type UsagePayload = { tokens: number; cost: number }
export type DonePayload = { content: string }

export const onChatDelta = (cb: (delta: string) => void) =>
  Events.On('chat:delta', (e) => cb(e.data as string))

export const onChatUsage = (cb: (u: UsagePayload) => void) =>
  Events.On('chat:usage', (e) => cb(e.data as UsagePayload))

export const onChatDone = (cb: (d: DonePayload) => void) =>
  Events.On('chat:done', (e) => cb(e.data as DonePayload))

export const onChatError = (cb: (msg: string) => void) =>
  Events.On('chat:error', (e) => cb(e.data as string))
```

- [ ] **Step 3: Verificar typecheck**

Run:
```bash
cd frontend && npx tsc --noEmit && cd ..
```
Expected: sem erros (ajustar caminho de import do runtime se o template usar outro alias).

- [ ] **Step 4: Commit**

```bash
git add frontend/bindings frontend/src/lib/events.ts
git commit -m "feat(frontend): bindings gerados e camada de eventos tipada"
```

---

### Task 9: View Chat com markdown e streaming

**Files:**
- Create: `frontend/src/views/ChatView.tsx`
- Create: `frontend/src/components/MessageCard.tsx`
- Create: `frontend/src/i18n.ts`
- Modify: `frontend/package.json` (react-markdown)

**Interfaces:**
- Consumes: bindings `ChatService.Send/Cancel/NewConversation`, helpers de `events.ts`.
- Produces: `ChatView` montável pelo `App` (Task 11); `MessageCard({ role, content })`.

- [ ] **Step 1: Instalar react-markdown**

Run:
```bash
cd frontend && npm install react-markdown && cd ..
```

- [ ] **Step 2: Criar `frontend/src/i18n.ts`**

Portar o mapa pt/en de `internal/ui/settings.go` (chaves `you`, `ai`, `thinking`, `placeholder`, `no_api_key`, etc.):
```ts
const pt: Record<string, string> = {
  you: 'Você', ai: 'IA', thinking: 'pensando…', placeholder: 'pergunte algo…',
  no_api_key: 'Configure a chave do OpenRouter nas configurações.',
  // ... demais chaves de settings.go
}
const en: Record<string, string> = {
  you: 'You', ai: 'AI', thinking: 'thinking…', placeholder: 'ask something…',
  no_api_key: 'Set your OpenRouter key in settings.',
  // ... demais chaves
}
export function tr(lang: string, key: string): string {
  const m = lang === 'en' ? en : pt
  return m[key] ?? key
}
```

- [ ] **Step 3: Criar `frontend/src/components/MessageCard.tsx`**

```tsx
import ReactMarkdown from 'react-markdown'

export function MessageCard({ role, content, label }:
  { role: 'user' | 'assistant'; content: string; label: string }) {
  const isUser = role === 'user'
  return (
    <div className={`flex flex-col ${isUser ? 'items-end' : 'items-start'}`}>
      <span className="text-xs font-bold text-muted mb-1">{label}</span>
      <div className="max-w-[75%] rounded-card bg-bubble px-3 py-2 text-fg font-mono whitespace-pre-wrap">
        <ReactMarkdown>{content}</ReactMarkdown>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Criar `frontend/src/views/ChatView.tsx`**

```tsx
import { useEffect, useRef, useState } from 'react'
import { ChatService } from '../../bindings/gix/internal/app'
import { onChatDelta, onChatDone, onChatError, onChatUsage } from '../lib/events'
import { MessageCard } from '../components/MessageCard'
import { tr } from '../i18n'

type Msg = { role: 'user' | 'assistant'; content: string }

export function ChatView({ lang }: { lang: string }) {
  const [msgs, setMsgs] = useState<Msg[]>([])
  const [input, setInput] = useState('')
  const [usage, setUsage] = useState<{ tokens: number; cost: number } | null>(null)
  const streamingRef = useRef(false)
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const offDelta = onChatDelta((delta) => {
      setMsgs((m) => {
        const copy = [...m]
        const last = copy[copy.length - 1]
        if (last && last.role === 'assistant') last.content += delta
        return copy
      })
    })
    const offDone = onChatDone(() => { streamingRef.current = false })
    const offErr = onChatError((code) => {
      streamingRef.current = false
      setMsgs((m) => {
        const copy = [...m]
        const last = copy[copy.length - 1]
        const text = code === 'no_api_key' ? tr(lang, 'no_api_key') : `${tr(lang, 'error_prefix')}${code}`
        if (last && last.role === 'assistant') last.content = text
        return copy
      })
    })
    const offUsage = onChatUsage((u) => setUsage(u))
    return () => { offDelta(); offDone(); offErr(); offUsage() }
  }, [lang])

  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [msgs])

  const send = () => {
    const text = input.trim()
    if (!text) return
    if (text.startsWith('/')) {
      if (text === '/new') {
        ChatService.NewConversation()
        setMsgs([]); setUsage(null); setInput('')
        return
      }
    }
    setMsgs((m) => [...m, { role: 'user', content: text }, { role: 'assistant', content: tr(lang, 'thinking') }])
    // limpa o placeholder "pensando…" no primeiro delta
    setMsgs((m) => { const c = [...m]; if (c.length) c[c.length - 1].content = ''; return c })
    streamingRef.current = true
    ChatService.Send(text)
    setInput('')
  }

  return (
    <div className="flex h-full flex-col bg-bg">
      {usage && (
        <div className="px-3 py-1 text-xs text-muted font-mono">
          Tokens: {usage.tokens} | ${usage.cost.toFixed(6)}
        </div>
      )}
      <div className="flex-1 overflow-y-auto px-3 py-2 space-y-3">
        {msgs.map((m, i) => (
          <MessageCard key={i} role={m.role} content={m.content}
            label={m.role === 'user' ? tr(lang, 'you') : tr(lang, 'ai')} />
        ))}
        <div ref={endRef} />
      </div>
      <textarea
        className="m-2 rounded-card bg-surface p-2 text-fg font-mono resize-none outline-none"
        rows={2} value={input} placeholder={tr(lang, 'placeholder')}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() } }}
      />
    </div>
  )
}
```

> Nota: ajustar o caminho de import dos bindings (`../../bindings/gix/internal/app`) conforme o gerador da Task 8 produzir.

- [ ] **Step 5: Typecheck**

Run:
```bash
cd frontend && npx tsc --noEmit && cd ..
```
Expected: sem erros.

- [ ] **Step 6: Commit**

```bash
git add frontend/src frontend/package.json frontend/package-lock.json
git commit -m "feat(frontend): view de chat com markdown e streaming por eventos"
```

---

### Task 10: View Settings

**Files:**
- Create: `frontend/src/views/SettingsView.tsx`

**Interfaces:**
- Consumes: bindings `ConfigService.Get/Save/Models`, tipo `Config` gerado.
- Produces: `SettingsView({ onClose })` que carrega o config, edita e salva.

- [ ] **Step 1: Implementar `SettingsView.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { ConfigService } from '../../bindings/gix/internal/app'

export function SettingsView({ onClose }: { onClose: () => void }) {
  const [cfg, setCfg] = useState<any>(null)
  const [models, setModels] = useState<string[]>([])

  useEffect(() => {
    ConfigService.Get().then(setCfg)
    ConfigService.Models().then(setModels)
  }, [])

  if (!cfg) return null
  const set = (k: string, v: any) => setCfg({ ...cfg, [k]: v })

  const save = async () => { await ConfigService.Save(cfg); onClose() }

  return (
    <div className="flex h-full flex-col bg-bg p-4 text-fg font-mono gap-3 overflow-y-auto">
      <label>Tema
        <select className="bg-surface ml-2" value={cfg.theme} onChange={(e) => set('theme', e.target.value)}>
          <option value="light">Claro</option><option value="dark">Escuro</option>
        </select>
      </label>
      <label>Idioma
        <select className="bg-surface ml-2" value={cfg.language} onChange={(e) => set('language', e.target.value)}>
          <option value="pt">Português</option><option value="en">English</option>
        </select>
      </label>
      <label>Modelo
        <select className="bg-surface ml-2" value={cfg.model} onChange={(e) => set('model', e.target.value)}>
          {models.map((m) => <option key={m} value={m}>{m}</option>)}
        </select>
      </label>
      <label className="flex flex-col">Chave da API
        <input type="password" className="bg-surface" value={cfg.api_key}
          onChange={(e) => set('api_key', e.target.value)} />
      </label>
      <label className="flex flex-col">Prompt de sistema
        <textarea className="bg-surface" value={cfg.system_prompt}
          onChange={(e) => set('system_prompt', e.target.value)} />
      </label>
      <div className="flex gap-2">
        <button className="bg-accent text-bg rounded-card px-3 py-1" onClick={save}>Salvar</button>
        <button className="bg-surface rounded-card px-3 py-1" onClick={onClose}>Cancelar</button>
      </div>
    </div>
  )
}
```

> Nota: os nomes de campo (`theme`, `api_key`, `system_prompt`...) seguem as tags `json` de `config.Config`. Confirmar contra o tipo `Config` gerado nos bindings; ajustar se o gerador exportar em camelCase.

- [ ] **Step 2: Typecheck**

Run:
```bash
cd frontend && npx tsc --noEmit && cd ..
```
Expected: sem erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/SettingsView.tsx
git commit -m "feat(frontend): view de configuracoes"
```

---

### Task 11: View History

**Files:**
- Create: `frontend/src/views/HistoryView.tsx`

**Interfaces:**
- Consumes: bindings `HistoryService.List/Messages/Delete`.
- Produces: `HistoryView({ onClose })` com lista de conversas + detalhe.

- [ ] **Step 1: Implementar `HistoryView.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { HistoryService } from '../../bindings/gix/internal/app'
import { MessageCard } from '../components/MessageCard'

export function HistoryView({ onClose }: { onClose: () => void }) {
  const [convs, setConvs] = useState<any[]>([])
  const [detail, setDetail] = useState<any[]>([])

  const reload = () => HistoryService.List().then((c) => setConvs(c ?? []))
  useEffect(() => { reload() }, [])

  return (
    <div className="flex h-full bg-bg text-fg font-mono">
      <div className="w-2/5 border-r border-surface overflow-y-auto">
        <button className="m-2 text-muted" onClick={onClose}>← voltar</button>
        {convs.length === 0 && <div className="p-3 text-muted">Nenhuma conversa salva.</div>}
        {convs.map((c) => (
          <div key={c.ID} className="flex items-center justify-between px-3 py-2 hover:bg-surface cursor-pointer">
            <span className="truncate" onClick={() => HistoryService.Messages(c.ID).then((m) => setDetail(m ?? []))}>{c.Title}</span>
            <button className="text-red-500" onClick={() => HistoryService.Delete(c.ID).then(reload)}>✕</button>
          </div>
        ))}
      </div>
      <div className="flex-1 overflow-y-auto p-3 space-y-3">
        {detail.map((m, i) => (
          <MessageCard key={i} role={m.Role === 'user' ? 'user' : 'assistant'} content={m.Content}
            label={m.Role === 'user' ? 'Você' : 'IA'} />
        ))}
      </div>
    </div>
  )
}
```

> Nota: campos `ID`, `Title`, `Role`, `Content` seguem os structs `db.Conversation`/`db.Message`. Confirmar a capitalização exportada pelos bindings.

- [ ] **Step 2: Typecheck**

Run:
```bash
cd frontend && npx tsc --noEmit && cd ..
```
Expected: sem erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/HistoryView.tsx
git commit -m "feat(frontend): view de historico"
```

---

### Task 12: `App.tsx` — navegação, esc-hide e embed de assets

**Files:**
- Modify: `frontend/src/App.tsx`
- Create: `internal/app/assets.go` (embed do `frontend/dist`)
- Create: `internal/app/icon.go` (embed do ícone do tray)
- Create: `internal/app/icon.png`

**Interfaces:**
- Consumes: `ChatView`, `SettingsView`, `HistoryView`, `ConfigService.Get`, `Window` do runtime.
- Produces: `App` raiz com troca de view e atalho de fechar; `assets embed.FS` e `trayIcon []byte` usados em `shell.go` (Task 6).

- [ ] **Step 1: Implementar `frontend/src/App.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { Window } from '@wailsio/runtime'
import { ChatView } from './views/ChatView'
import { SettingsView } from './views/SettingsView'
import { HistoryView } from './views/HistoryView'
import { ChatService, ConfigService } from '../bindings/gix/internal/app'

type View = 'chat' | 'settings' | 'history'

export default function App() {
  const [view, setView] = useState<View>('chat')
  const [lang, setLang] = useState('pt')
  const [theme, setTheme] = useState('light')

  const loadCfg = () => ConfigService.Get().then((c: any) => { setLang(c.language); setTheme(c.theme) })
  useEffect(() => { loadCfg() }, [])
  useEffect(() => { document.documentElement.dataset.theme = theme }, [theme])

  // Duplo-Esc esconde a janela e cancela streaming.
  useEffect(() => {
    let last = 0
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        const now = Date.now()
        if (now - last < 500) { ChatService.Cancel(); Window.Hide() }
        last = now
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="h-screen w-screen bg-bg text-fg">
      <div className="flex gap-2 px-2 py-1 bg-surface text-muted text-sm">
        <button onClick={() => setView('chat')}>Chat</button>
        <button onClick={() => setView('history')}>Histórico</button>
        <div className="flex-1" />
        <button onClick={() => setView('settings')}>⚙</button>
      </div>
      <div className="h-[calc(100%-2rem)]">
        {view === 'chat' && <ChatView lang={lang} />}
        {view === 'settings' && <SettingsView onClose={() => { loadCfg(); setView('chat') }} />}
        {view === 'history' && <HistoryView onClose={() => setView('chat')} />}
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Adicionar o ícone do tray**

Copiar um PNG 256x256 para `internal/app/icon.png` (pode reaproveitar `build/appicon.png` do scaffold):
```bash
cp build/appicon.png internal/app/icon.png
```

- [ ] **Step 3: Criar `internal/app/assets.go` e `internal/app/icon.go`**

`internal/app/assets.go`:
```go
package app

import "embed"

//go:embed all:../../frontend/dist
var assets embed.FS
```
`internal/app/icon.go`:
```go
package app

import _ "embed"

//go:embed icon.png
var trayIcon []byte
```

> Nota: o caminho do embed de `dist` deve ser relativo ao arquivo Go. Se `//go:embed ../../frontend/dist` for rejeitado (embed não permite `..`), mover o embed para um pacote na raiz do frontend ou ajustar o layout para `frontend/dist` ser irmão do `assets.go`. Alternativa canônica do template: manter `assets.go` na raiz do módulo. Validar e ajustar o local do arquivo de embed conforme a restrição do `go:embed`.

- [ ] **Step 4: Build do frontend e do binário**

Run:
```bash
cd frontend && npm run build && cd ..
go build ./...
```
Expected: `frontend/dist` gerado; `go build ./...` sem erros (agora `assets` e `trayIcon` existem).

- [ ] **Step 5: Rodar o app completo**

Run:
```bash
wails3 dev
```
Expected: janela frameless abre; navegação Chat/Histórico/Settings funciona; tema aplicado via `data-theme`.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/App.tsx internal/app/assets.go internal/app/icon.go internal/app/icon.png
git commit -m "feat(app): App raiz com navegacao, esc-hide e embed de assets"
```

---

## FASE 4 — Verificação ponta a ponta e limpeza

### Task 13: Verificação manual do comportamento completo

**Files:** nenhum (roteiro de verificação).

- [ ] **Step 1: Build de produção**

Run:
```bash
wails3 build
```
Expected: binário em `build/bin` (ou conforme `Taskfile.yml`).

- [ ] **Step 2: Roteiro manual (marcar cada item)**

Rodar o binário e validar:
- App inicia escondido, ícone aparece no system tray.
- Duplo-Espaço global abre a janela centralizada e com foco no input.
- Enviar mensagem: tokens aparecem; resposta faz streaming com markdown renderizado.
- `Shift+Enter` quebra linha; `Enter` envia.
- `/new` limpa a conversa e zera tokens.
- Duplo-Esc esconde a janela e cancela um streaming em curso.
- Settings: trocar tema reflete no visual após salvar; trocar idioma muda rótulos; salvar API key/modelo/prompt persiste.
- Histórico: lista conversas, abre detalhe, exclui.
- Tray → Exibir mostra; Tray → Sair encerra.

Sem commit (verificação).

---

### Task 14: Remover a UI Fyne e dependências órfãs

**Files:**
- Delete: `internal/ui/**`
- Modify: `go.mod`/`go.sum` (remover Fyne)

- [ ] **Step 1: Apagar o pacote Fyne**

Run:
```bash
rm -rf internal/ui
```

- [ ] **Step 2: Remover dependências órfãs**

Run:
```bash
go mod tidy
```
Expected: entradas do `fyne.io/fyne/v2` somem do `go.mod`.

- [ ] **Step 3: Build e testes completos**

Run:
```bash
go build ./... && go test ./...
```
Expected: build ok; testes de `ai`, `config`, `db`, `app` passam.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore(ui): remover UI Fyne apos migracao para Wails"
```

---

### Task 15: Atualizar build/release (goreleaser + lefthook)

**Files:**
- Modify: `.goreleaser.yaml`
- Modify: `.lefthook.yml`
- Modify: `Taskfile.yml` (se necessário)

**Interfaces:**
- Produces: pipeline que builda o frontend antes do binário.

- [ ] **Step 1: Ajustar `.lefthook.yml`**

Garantir que o pre-commit não quebre por falta do `frontend/dist`. Como `go test ./...` agora depende do embed de `assets`, adicionar um passo que builda o frontend (ou usar uma tag de build/`dist` placeholder commitado). Estratégia recomendada: criar `frontend/dist/.gitkeep` versionado para o embed sempre resolver, e deixar o build real do dist fora do pre-commit.

Adicionar `frontend/dist/.gitkeep`:
```bash
mkdir -p frontend/dist && touch frontend/dist/.gitkeep
git add -f frontend/dist/.gitkeep
```
E remover `frontend/dist` do `.gitignore` apenas para o `.gitkeep` (usar `frontend/dist/*` + `!frontend/dist/.gitkeep`).

- [ ] **Step 2: Ajustar `.goreleaser.yaml`**

Adicionar hook `before` que builda o frontend:
```yaml
before:
  hooks:
    - sh -c "cd frontend && npm ci && npm run build"
    - go mod tidy
```
E confirmar que o `main` aponta para `./cmd/gix`.

- [ ] **Step 3: Validar**

Run:
```bash
go build ./... && go test ./...
```
Expected: passa com o `dist` (real ou placeholder) presente.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yaml .lefthook.yml Taskfile.yml frontend/dist/.gitkeep .gitignore
git commit -m "build: pipeline de release com build do frontend (Wails)"
```

---

## Self-review (cobertura do spec)

- **Capacidades além do Fyne** → markdown (Task 9), tokens/temas (Task 7), animações destravadas via CSS (base pronta). ✓
- **Mais controle no código (tokens)** → Task 7, regra "sem literais". ✓
- **Janela única com views** → Tasks 11/12. ✓
- **Core Go intacto** → `ai`/`config`/`db` não tocados; só movido `hotkey` (Task 2). ✓
- **ChatService streaming via eventos** → Task 5 + Task 6 wiring. ✓
- **Tray + frameless + hotkey + center + esc-hide** → Tasks 6 e 12. ✓
- **`/new`, pt/en, duplo-Enter/Esc** → Tasks 9 e 12. ✓
- **Limpeza Fyne + build** → Tasks 14 e 15. ✓

Riscos sinalizados inline onde a API alpha do Wails v3 pode divergir (nomes de opções de janela, embed com `..`, caminho dos bindings).
