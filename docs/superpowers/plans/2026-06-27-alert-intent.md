# Alertas em linguagem natural Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detectar intenção de lembrete em `/note` e em mensagens comuns do chat e, após confirmação do usuário, criar o alerta — sem precisar do `/alerta`.

**Architecture:** Os dois pontos de entrada produzem uma "proposta" `{message, fireAt, recurrence, noteId?}` que passa por um único caminho de confirmar-e-criar (`AlertsService.CreateProposed`, sem nova chamada de IA). O `/note` estende a chamada de captura já existente para também emitir um `alert`; o chat ganha *tool calling* (`create_alert`) sobre o stream do OpenRouter.

**Tech Stack:** Go (pacote `internal/app`, `internal/ai`), Wails v3, React + TypeScript + Vitest no `frontend/`.

## Global Constraints

- Pacote Go dos serviços: `package app` em `internal/app`; cliente HTTP em `package ai`.
- Pre-commit (lefthook) roda `go test ./...` e `go vet`; commit-msg roda commitlint (Conventional Commits — ex.: `feat(alerts): ...`, `test(...)`, `refactor(...)`).
- Strings de UI são em português; i18n vive em `frontend/src/i18n.ts` com dicionários `pt` e `en` (toda chave nova vai nos dois).
- Confirmação SEMPRE antes de criar: nada é gravado sem o usuário escolher "Sim".
- `/note` sempre cria a nota; o alerta só é proposto quando há horário/data concreto.
- Não há segundo round-trip de resultado de ferramenta ao modelo no chat — a confirmação é local.
- `fire_at` trafega como ISO 8601 com offset (RFC3339); recorrência trafega como JSON marshalado (string) ou `""`.

---

### Task 1: Tool calling no cliente de IA

**Files:**
- Modify: `internal/ai/client.go`
- Test: `internal/ai/client_test.go`

**Interfaces:**
- Consumes: nada novo.
- Produces:
  - `type Tool struct { Type string; Function ToolFunction }`
  - `type ToolFunction struct { Name string; Description string; Parameters json.RawMessage }`
  - `type ToolCall struct { Name string; Arguments string }`
  - `func (c *Client) StreamTools(ctx context.Context, model string, messages []Message, tools []Tool, onDelta func(string)) (*Usage, []ToolCall, error)`
  - `func (c *Client) Stream(...)` mantém a assinatura atual (vira wrapper de `StreamTools`).

- [ ] **Step 1: Write the failing test**

Adicione a `internal/ai/client_test.go`:

```go
func TestStreamToolsAccumulatesToolCallDeltas(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"create_alert","arguments":"{\"message\":\"x\","}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"fire_at\":\"2099-01-01T09:00:00-03:00\"}"}}]}}]}`,
		`data: {"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
		"data: [DONE]",
		"",
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	c := New("k")
	c.baseURL = srv.URL

	var text strings.Builder
	usage, calls, err := c.StreamTools(context.Background(), "m",
		[]Message{{Role: "user", Content: "oi"}}, []Tool{{Type: "function"}},
		func(s string) { text.WriteString(s) })
	if err != nil {
		t.Fatalf("StreamTools: %v", err)
	}
	if text.String() != "" {
		t.Fatalf("não deveria haver texto, veio %q", text.String())
	}
	if len(calls) != 1 || calls[0].Name != "create_alert" {
		t.Fatalf("esperava 1 tool call create_alert, veio %+v", calls)
	}
	if calls[0].Arguments != `{"message":"x","fire_at":"2099-01-01T09:00:00-03:00"}` {
		t.Fatalf("arguments concatenados errados: %q", calls[0].Arguments)
	}
	if usage == nil || usage.TotalTokens != 3 {
		t.Fatalf("usage = %+v", usage)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestStreamToolsAccumulatesToolCallDeltas -v`
Expected: FAIL — `c.StreamTools undefined`.

- [ ] **Step 3: Write minimal implementation**

Em `internal/ai/client.go`, adicione os tipos (após `type Message struct`):

```go
// Tool descreve uma função que o modelo pode chamar (tool calling).
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall é uma chamada de função completa, remontada a partir do stream.
type ToolCall struct {
	Name      string
	Arguments string
}

type toolCallAcc struct {
	name string
	args strings.Builder
}
```

Adicione `Tools` ao request:

```go
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Tools    []Tool    `json:"tools,omitempty"`
}
```

Estenda `streamChunk` para tool calls:

```go
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int `json:"index"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
}
```

Substitua o método `Stream` por um wrapper + `StreamTools`:

```go
// Stream mantém a interface antiga: stream de texto, sem ferramentas.
func (c *Client) Stream(ctx context.Context, model string, messages []Message, onDelta func(string)) (*Usage, error) {
	u, _, err := c.StreamTools(ctx, model, messages, nil, onDelta)
	return u, err
}

// StreamTools faz a chamada com stream:true, repassa texto via onDelta e remonta
// quaisquer tool_calls (argumentos chegam em pedaços por índice). Retorna o Usage
// e as tool calls completas. ctx cancelado aborta; status != 2xx vira erro.
func (c *Client) StreamTools(ctx context.Context, model string, messages []Message, tools []Tool, onDelta func(string)) (*Usage, []ToolCall, error) {
	body, err := json.Marshal(chatRequest{Model: model, Messages: messages, Stream: true, Tools: tools})
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "gix")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("openrouter: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var usage *Usage
	calls := map[int]*toolCallAcc{}
	var order []int

	reader := bufio.NewReader(resp.Body)
	for {
		line, readErr := reader.ReadString('\n')
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if data == "[DONE]" {
				return usage, collectToolCalls(calls, order), nil
			}
			var chunk streamChunk
			if json.Unmarshal([]byte(data), &chunk) == nil {
				if chunk.Usage != nil {
					usage = chunk.Usage
				}
				for _, ch := range chunk.Choices {
					if ch.Delta.Content != "" {
						onDelta(ch.Delta.Content)
					}
					for _, tc := range ch.Delta.ToolCalls {
						a, ok := calls[tc.Index]
						if !ok {
							a = &toolCallAcc{}
							calls[tc.Index] = a
							order = append(order, tc.Index)
						}
						if tc.Function.Name != "" {
							a.name = tc.Function.Name
						}
						a.args.WriteString(tc.Function.Arguments)
					}
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return usage, collectToolCalls(calls, order), nil
			}
			return usage, collectToolCalls(calls, order), readErr
		}
	}
}

func collectToolCalls(calls map[int]*toolCallAcc, order []int) []ToolCall {
	if len(order) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(order))
	for _, idx := range order {
		a := calls[idx]
		out = append(out, ToolCall{Name: a.name, Arguments: a.args.String()})
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ai/ -v`
Expected: PASS (novo teste + todos os existentes de `Stream`/`Complete` continuam verdes).

- [ ] **Step 5: Commit**

```bash
git add internal/ai/client.go internal/ai/client_test.go
git commit -m "feat(ai): tool calling no stream (StreamTools) com acumulação de deltas"
```

---

### Task 2: `AlertsService.CreateProposed` e refator do caminho de gravação

**Files:**
- Modify: `internal/app/alerts.go`
- Test: `internal/app/alerts_test.go`

**Interfaces:**
- Consumes: `marshalRecurrence` (recurrence.go), `db.CreateAlert`, `s.loc`, `s.db`.
- Produces:
  - `func (s *AlertsService) store(message, fireAtISO, recurrence, defaultMessage string, noteID *int64) CreateAlertResult`
  - `func (s *AlertsService) CreateProposed(message, fireAtISO, recurrence string, noteID *int64) (CreateAlertResult, error)`
  - `func futureOrRecurring(fireAtISO, recurrence string, now time.Time) bool`

- [ ] **Step 1: Write the failing test**

Adicione a `internal/app/alerts_test.go`:

```go
func TestCreateProposedStoresFutureAndRejectsPast(t *testing.T) {
	d := alertsTestDB(t)
	svc := newAlertsSvc(t, d, &fakeCompleter{})

	res, err := svc.CreateProposed("dentista", "2099-05-05T09:00:00-03:00", "", nil)
	if err != nil {
		t.Fatalf("CreateProposed: %v", err)
	}
	if res.Status != "created" || res.Message != "dentista" || res.AlertID == 0 {
		t.Fatalf("esperava created, veio %+v", res)
	}

	past, _ := svc.CreateProposed("velho", "2000-01-01T09:00:00-03:00", "", nil)
	if past.Status != "past" {
		t.Fatalf("esperava past, veio %+v", past)
	}
	if all, _ := d.ListAlerts(); len(all) != 1 {
		t.Fatalf("só o futuro deveria estar gravado, veio %d", len(all))
	}
}

func TestCreateProposedLinksNoteAndKeepsRecurrence(t *testing.T) {
	d := alertsTestDB(t)
	noteID, _ := d.CreateNote("Pagar conta", "boleto", nil, nil, 0)
	svc := newAlertsSvc(t, d, &fakeCompleter{})

	res, _ := svc.CreateProposed("Pagar conta", "2000-01-01T08:00:00-03:00",
		`{"freq":"monthly","interval":1}`, &noteID)
	if res.Status != "created" {
		t.Fatalf("recorrente no passado deveria gravar, veio %+v", res)
	}
	stored, _ := d.GetAlert(res.AlertID)
	if stored.NoteID == nil || *stored.NoteID != noteID {
		t.Fatalf("esperava vínculo com a nota, veio %+v", stored)
	}
}

func TestFutureOrRecurring(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	if futureOrRecurring("2020-01-01T09:00:00-03:00", "", now) {
		t.Fatal("passado sem recorrência não deveria valer")
	}
	if !futureOrRecurring("2020-01-01T09:00:00-03:00", `{"freq":"daily","interval":1}`, now) {
		t.Fatal("recorrente deveria valer mesmo no passado")
	}
	if !futureOrRecurring("2099-01-01T09:00:00-03:00", "", now) {
		t.Fatal("futuro deveria valer")
	}
	if futureOrRecurring("data ruim", "", now) {
		t.Fatal("data inválida não deveria valer")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestCreateProposed|TestFutureOrRecurring' -v`
Expected: FAIL — `svc.CreateProposed undefined` / `futureOrRecurring undefined`.

- [ ] **Step 3: Write minimal implementation**

Em `internal/app/alerts.go`, substitua o corpo de `createFromWhen` (linhas que validam/gravam) extraindo o método `store`. O `createFromWhen` fica:

```go
func (s *AlertsService) createFromWhen(text, defaultMessage string, noteID *int64) (CreateAlertResult, error) {
	if s.db == nil {
		return CreateAlertResult{Status: "error"}, nil
	}
	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return CreateAlertResult{Status: "no_api_key"}, nil
	}

	dec, err := s.parseWhen(text, defaultMessage, time.Now())
	if err != nil {
		return CreateAlertResult{Status: "unparseable"}, nil
	}
	return s.store(dec.Message, dec.FireAt, marshalRecurrence(dec.Recurrence), defaultMessage, noteID), nil
}

// store valida campos já parseados e grava o alerta (sem IA). defaultMessage
// (ex.: título da nota) é usado quando message vem vazio.
func (s *AlertsService) store(message, fireAtISO, recurrence, defaultMessage string, noteID *int64) CreateAlertResult {
	if s.db == nil {
		return CreateAlertResult{Status: "error"}
	}
	fireAt, err := time.Parse(time.RFC3339, strings.TrimSpace(fireAtISO))
	if err != nil {
		return CreateAlertResult{Status: "unparseable"}
	}
	fireAt = fireAt.UTC()

	message = strings.TrimSpace(message)
	if message == "" {
		message = strings.TrimSpace(defaultMessage)
	}
	if message == "" {
		return CreateAlertResult{Status: "unparseable"}
	}
	if recurrence == "" && !fireAt.After(time.Now()) {
		return CreateAlertResult{Status: "past"}
	}

	id, err := s.db.CreateAlert(db.Alert{Message: message, NoteID: noteID, FireAt: fireAt, Recurrence: recurrence})
	if err != nil {
		return CreateAlertResult{Status: "error"}
	}
	return CreateAlertResult{
		Status:      "created",
		AlertID:     id,
		Message:     message,
		FireAtLocal: fireAt.In(s.loc).Format(time.RFC3339),
		Recurrence:  recurrence,
	}
}

// CreateProposed grava um alerta a partir de campos já parseados (sem chamar a
// IA). Usado quando o chat (tool call) ou um /note já produziu o horário.
func (s *AlertsService) CreateProposed(message, fireAtISO, recurrence string, noteID *int64) (CreateAlertResult, error) {
	return s.store(message, fireAtISO, recurrence, "", noteID), nil
}

// futureOrRecurring diz se um alerta parseado vale a pena propor: fire time
// válido que seja recorrente ou ainda no futuro.
func futureOrRecurring(fireAtISO, recurrence string, now time.Time) bool {
	fireAt, err := time.Parse(time.RFC3339, strings.TrimSpace(fireAtISO))
	if err != nil {
		return false
	}
	return recurrence != "" || fireAt.After(now)
}
```

Remova do antigo `createFromWhen` o bloco que fazia `time.Parse`/checagem de message/past/`CreateAlert` (agora vive em `store`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestCreate|TestFutureOrRecurring' -v`
Expected: PASS (inclui os testes antigos `TestCreateOneShotAlert`, `TestCreatePastOneShotRejected`, `TestCreateForNoteDefaultsMessageToNoteTitle`, etc.).

- [ ] **Step 5: Commit**

```bash
git add internal/app/alerts.go internal/app/alerts_test.go
git commit -m "feat(alerts): CreateProposed e refator do caminho de gravação (store)"
```

---

### Task 3: `/note` detecta o alerta na captura

**Files:**
- Modify: `internal/app/notes.go`
- Test: `internal/app/notes_test.go`

**Interfaces:**
- Consumes: `marshalRecurrence`, `futureOrRecurring` (Task 2), `alertDecision` (alerts.go), `recurrenceRule` (recurrence.go).
- Produces:
  - `type AlertProposal struct { Message string; FireAt string; Recurrence string }` (tags JSON: `message`, `fireAt`, `recurrence`)
  - Campo novo em `CaptureResult`: `Alert *AlertProposal \`json:"alert"\``

- [ ] **Step 1: Write the failing test**

Adicione a `internal/app/notes_test.go`:

```go
func TestCaptureDetectsAlertWhenTimeBound(t *testing.T) {
	d := notesTestDB(t)
	fake := &fakeCompleter{responses: []string{
		`{"title":"Médico","content":"consulta","tags":["saude"],"alert":{"message":"ligar pro médico","fire_at":"2099-04-01T09:00:00-03:00","recurrence":null}}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Capture("ligar pro médico amanhã 9h")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if res.Status != "created" {
		t.Fatalf("nota deveria ser criada, veio %+v", res)
	}
	if res.Alert == nil || res.Alert.Message != "ligar pro médico" {
		t.Fatalf("esperava proposta de alerta, veio %+v", res.Alert)
	}
	if res.Alert.FireAt != "2099-04-01T09:00:00-03:00" {
		t.Fatalf("fire_at da proposta = %q", res.Alert.FireAt)
	}
}

func TestCaptureNoAlertWhenNotTimeBound(t *testing.T) {
	d := notesTestDB(t)
	fake := &fakeCompleter{responses: []string{
		`{"title":"Ideia","content":"comprar leite","tags":["mercado"],"alert":null}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, _ := svc.Capture("ideia: comprar leite")
	if res.Status != "created" {
		t.Fatalf("nota deveria ser criada, veio %+v", res)
	}
	if res.Alert != nil {
		t.Fatalf("não deveria propor alerta, veio %+v", res.Alert)
	}
}
```

> Nota: use os mesmos helpers que os outros testes de notes usam para montar o serviço e o DB. Se `newNotesSvc`/`notesTestDB` não existirem com esses nomes, reutilize o helper de construção já presente em `notes_test.go` (procure por `NewNotesService(` no arquivo) e o `alertsTestDB`/equivalente para o DB temporário.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestCaptureDetectsAlert -v`
Expected: FAIL — `res.Alert undefined`.

- [ ] **Step 3: Write minimal implementation**

Em `internal/app/notes.go`:

Adicione o tipo e o campo em `CaptureResult`:

```go
// AlertProposal é um lembrete que o modelo de captura detectou dentro da nota.
// FireAt é ISO 8601 com offset (como o modelo retornou); Recurrence é JSON
// marshalado ou "". Presente só quando a nota tem horário/data concretos.
type AlertProposal struct {
	Message    string `json:"message"`
	FireAt     string `json:"fireAt"`
	Recurrence string `json:"recurrence"`
}
```

No struct `CaptureResult`, acrescente:

```go
	Alert     *AlertProposal `json:"alert"`
```

Em `captureDecision`, acrescente:

```go
	Alert   *alertDecision `json:"alert"`
```

Em `Capture`, depois de calcular `title` e `tags` e antes do `CreateNote`, monte a proposta; e inclua no resultado:

```go
	var proposal *AlertProposal
	if dec.Alert != nil {
		rec := marshalRecurrence(dec.Alert.Recurrence)
		if futureOrRecurring(dec.Alert.FireAt, rec, time.Now()) {
			msg := strings.TrimSpace(dec.Alert.Message)
			if msg == "" {
				msg = title
			}
			proposal = &AlertProposal{Message: msg, FireAt: strings.TrimSpace(dec.Alert.FireAt), Recurrence: rec}
		}
	}
```

E no `return` de sucesso, adicione `Alert: proposal`:

```go
	return CaptureResult{Status: "created", NoteID: id, NoteTitle: title, Tags: tags, Tokens: tokens, Cost: cost, Alert: proposal}, nil
```

Estenda `buildCapturePrompt` para pedir o `alert` (substitua o system prompt atual):

```go
	system := fmt.Sprintf(`Você organiza anotações rápidas do usuário em uma nota atômica e bem formatada.
A data e hora atuais são: %s.
Resolva qualquer data relativa ("amanhã", "sexta") para uma data absoluta no texto.
Formate "content" como Markdown bem estruturado (parágrafo, lista, tarefa "- [ ]", ou pequena seção) — preserve a informação do usuário, sem inventar nem remover.
Gere um "title" curto (sem marcadores Markdown) e de 1 a 5 "tags" temáticas, minúsculas, sem "#".
Se — e somente se — a nota descrever um lembrete com horário/data concretos, inclua "alert" com o horário resolvido; caso contrário use "alert": null.
Responda APENAS com JSON, sem cercas:
{"title":"<título curto>","content":"<Markdown da nota>","tags":["tag1","tag2"],"alert":null ou {"message":"<lembrete curto>","fire_at":"<ISO 8601 com offset>","recurrence":null ou {"freq":"daily|weekly|monthly|yearly","interval":1,"weekday":"mon","time":"09:00"}}}`,
		now.Format("2006-01-02 15:04 (Monday)"))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run TestCapture -v`
Expected: PASS (novos + os testes de captura existentes).

- [ ] **Step 5: Commit**

```bash
git add internal/app/notes.go internal/app/notes_test.go
git commit -m "feat(notes): captura detecta lembrete e expõe AlertProposal"
```

---

### Task 4: Chat oferece a ferramenta `create_alert` e emite `alert:proposed`

**Files:**
- Modify: `internal/app/chat.go`
- Test: `internal/app/chat_test.go`

**Interfaces:**
- Consumes: `ai.Tool`, `ai.ToolCall`, `Client.StreamTools` (Task 1); `alertDecision`, `marshalRecurrence`.
- Produces:
  - Interface `Streamer` agora exige `StreamTools(ctx, model, msgs, tools, onDelta) (*ai.Usage, []ai.ToolCall, error)`.
  - Evento `alert:proposed` com payload `{message string; fireAt string; recurrence string}` (tags JSON `message`/`fireAt`/`recurrence`).

- [ ] **Step 1: Write the failing test**

Em `internal/app/chat_test.go`, atualize os dois fakes para a nova assinatura e adicione um campo de tool calls ao `fakeStreamer`:

```go
type fakeStreamer struct {
	deltas    []string
	usage     *ai.Usage
	toolCalls []ai.ToolCall
}

func (f *fakeStreamer) StreamTools(ctx context.Context, model string, msgs []ai.Message, tools []ai.Tool, onDelta func(string)) (*ai.Usage, []ai.ToolCall, error) {
	for _, d := range f.deltas {
		onDelta(d)
	}
	return f.usage, f.toolCalls, nil
}
```

```go
func (b *blockingStreamer) StreamTools(ctx context.Context, model string, msgs []ai.Message, tools []ai.Tool, onDelta func(string)) (*ai.Usage, []ai.ToolCall, error) {
	close(b.entered)
	<-b.release
	return &ai.Usage{}, nil, nil
}
```

Adicione o teste novo:

```go
func TestChatServiceEmitsAlertProposedOnToolCall(t *testing.T) {
	t.Setenv("AppData", t.TempDir())
	d, err := db.Open(filepath.Join(t.TempDir(), "c3.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	var mu sync.Mutex
	events := map[string]int{}
	var proposed any
	emit := func(name string, data any) {
		mu.Lock()
		defer mu.Unlock()
		events[name]++
		if name == "alert:proposed" {
			proposed = data
		}
	}

	cfgSvc := NewConfigService()
	cur := cfgSvc.Current()
	cur.APIKey = "k"
	_ = cfgSvc.Save(*cur)

	fake := &fakeStreamer{
		usage:     &ai.Usage{TotalTokens: 3},
		toolCalls: []ai.ToolCall{{Name: "create_alert", Arguments: `{"message":"remédio","fire_at":"2099-01-01T09:00:00-03:00","recurrence":null}`}},
	}
	s := NewChatService(cfgSvc, d, emit, func(string) Streamer { return fake })

	s.Send("me lembra do remédio amanhã 9h")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := events["alert:proposed"]
		mu.Unlock()
		if got > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if events["alert:proposed"] != 1 {
		t.Fatalf("esperava 1 alert:proposed, veio %d", events["alert:proposed"])
	}
	if events["chat:done"] != 0 {
		t.Fatalf("tool call sem texto não deveria emitir chat:done, veio %d", events["chat:done"])
	}
	p, ok := proposed.(alertProposedPayload)
	if !ok || p.Message != "remédio" || p.FireAt != "2099-01-01T09:00:00-03:00" {
		t.Fatalf("payload inesperado: %+v", proposed)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestChatService -v`
Expected: FAIL — compilação (`alertProposedPayload undefined`, fakes não satisfazem a interface até atualizar `Send`).

- [ ] **Step 3: Write minimal implementation**

Em `internal/app/chat.go`:

Atualize os imports para incluir `encoding/json`, `fmt`, `time` (além dos atuais `context`, `strings`, `sync`, `ai`, `config`, `db`).

Troque a interface `Streamer`:

```go
type Streamer interface {
	StreamTools(ctx context.Context, model string, msgs []ai.Message, tools []ai.Tool, onDelta func(string)) (*ai.Usage, []ai.ToolCall, error)
}
```

Adicione, no nível do pacote (perto do topo do arquivo):

```go
// createAlertTool deixa o modelo do chat agendar um lembrete em vez de só
// responder em prosa. O app intercepta a chamada e pede confirmação ao usuário.
var createAlertTool = ai.Tool{
	Type: "function",
	Function: ai.ToolFunction{
		Name:        "create_alert",
		Description: "Agenda um lembrete/alarme quando o usuário pede para ser lembrado de algo num horário ou data. Resolva datas relativas a partir do horário local atual informado no system prompt.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"},"fire_at":{"type":"string","description":"ISO 8601 com offset"},"recurrence":{"type":["object","null"]}},"required":["message","fire_at"]}`),
	},
}

type alertProposedPayload struct {
	Message    string `json:"message"`
	FireAt     string `json:"fireAt"`
	Recurrence string `json:"recurrence"`
}

// chatToolSystem injeta o horário local atual para o modelo resolver datas
// relativas ao chamar create_alert.
func chatToolSystem(now time.Time, language string) ai.Message {
	zoneName, offsetSec := now.Zone()
	return ai.Message{Role: "system", Content: fmt.Sprintf(
		`Data e hora locais atuais: %s. Fuso: %s (UTC%+d). Idioma: %s. Se o usuário pedir um lembrete/alarme com horário ou data, chame a ferramenta create_alert (resolvendo datas relativas a ESTE momento) em vez de só responder.`,
		now.Format("2006-01-02 15:04 (Monday)"), zoneName, offsetSec/3600, language)}
}

func findToolCall(calls []ai.ToolCall, name string) (ai.ToolCall, bool) {
	for _, c := range calls {
		if c.Name == name {
			return c, true
		}
	}
	return ai.ToolCall{}, false
}

func parseAlertCall(c ai.ToolCall) (alertProposedPayload, error) {
	var dec alertDecision
	if err := json.Unmarshal([]byte(c.Arguments), &dec); err != nil {
		return alertProposedPayload{}, err
	}
	if strings.TrimSpace(dec.FireAt) == "" {
		return alertProposedPayload{}, fmt.Errorf("empty fire_at")
	}
	return alertProposedPayload{
		Message:    strings.TrimSpace(dec.Message),
		FireAt:     strings.TrimSpace(dec.FireAt),
		Recurrence: marshalRecurrence(dec.Recurrence),
	}, nil
}
```

No `Send`, ao montar `msgs`, adicione a system message da ferramenta logo após o `cfg.SystemPrompt`:

```go
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		msgs = append(msgs, ai.Message{Role: "system", Content: cfg.SystemPrompt})
	}
	msgs = append(msgs, chatToolSystem(time.Now(), cfg.Language))
	msgs = append(msgs, s.history...)
```

Troque a chamada de stream dentro da goroutine:

```go
		usage, toolCalls, streamErr := client.StreamTools(ctx, cfg.Model, msgs, []ai.Tool{createAlertTool}, func(delta string) {
			sb.WriteString(delta)
			s.emit("chat:delta", delta)
		})
```

Substitua o `default:` do switch para tratar a tool call:

```go
		default:
			if call, ok := findToolCall(toolCalls, "create_alert"); ok {
				if p, perr := parseAlertCall(call); perr == nil {
					s.emit("alert:proposed", p)
					if full != "" {
						s.persist(cid, gen, full)
						s.emit("chat:done", DonePayload{Content: full})
					} else {
						s.persist(cid, gen, "(propôs um alerta)")
					}
					return
				}
			}
			if full == "" {
				full = "(sem resposta)"
			}
			s.persist(cid, gen, full)
			s.emit("chat:done", DonePayload{Content: full})
```

> `cfg.Language` já existe no config (usado por `buildAlertPrompt`). Se o nome do campo divergir, use o mesmo acessor que `buildAlertPrompt` usa para idioma.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -v`
Expected: PASS (novo teste + `TestChatServiceSendEmitsSequence` e `TestChatServiceSecondSendWhileStreamingIsNoop` continuam verdes).

- [ ] **Step 5: Commit**

```bash
git add internal/app/chat.go internal/app/chat_test.go
git commit -m "feat(chat): ferramenta create_alert emite alert:proposed para confirmação"
```

---

### Task 5: Bindings, eventos e tipos no frontend

**Files:**
- Regenerate: `frontend/bindings/...` (gerado pelo Wails)
- Modify: `frontend/src/lib/events.ts`
- Modify: `frontend/src/commands/types.ts`

**Interfaces:**
- Consumes: `AlertsService.CreateProposed` e `NotesService` (capture com `alert`) das bindings.
- Produces:
  - `onAlertProposed(cb)` + `AlertProposedPayload` em `events.ts`.
  - `CaptureResult.alert?` e `CommandContext.alerts.createProposed` em `types.ts`.

- [ ] **Step 1: Regenerar as bindings do Wails**

Run (na raiz do projeto): `wails3 generate bindings -clean`
Expected: `frontend/bindings/gix/internal/app/alertsservice.ts` passa a expor `CreateProposed(message, fireAt, recurrence, noteId)`, e os models de `CaptureResult` ganham `alert`.

Verify: `grep -n "CreateProposed" frontend/bindings/gix/internal/app/alertsservice.ts`
Expected: uma linha de função.

> Se o comando de geração do projeto for outro (cheque `Taskfile.yml`/`Makefile`/`wails.json` na raiz), use o equivalente. Não edite bindings à mão.

- [ ] **Step 2: Adicionar o evento `alert:proposed`**

Em `frontend/src/lib/events.ts`, espelhando os handlers existentes (`onAlertFired`/`onAlertOpen`):

```ts
export interface AlertProposedPayload {
  message: string
  fireAt: string
  recurrence: string
}

export function onAlertProposed(cb: (p: AlertProposedPayload) => void) {
  return Events.On('alert:proposed', (e) => cb(e.data as AlertProposedPayload))
}
```

- [ ] **Step 3: Estender os tipos de comando**

Em `frontend/src/commands/types.ts`:

No `interface CaptureResult`, adicione:

```ts
  alert?: { message: string; fireAt: string; recurrence: string }
```

No `CommandContext.alerts`, adicione `createProposed`:

```ts
  alerts: {
    create(text: string): Promise<CreateAlertResult>
    createProposed(p: { message: string; fireAt: string; recurrence: string; noteId?: number }): Promise<CreateAlertResult>
  }
```

- [ ] **Step 4: Verificar tipos**

Run: `cd frontend && npx tsc --noEmit`
Expected: pode falhar SÓ em `App.tsx` (ainda não implementa `createProposed`) — isso é resolvido na Task 6. Não deve haver erro em `events.ts`/`types.ts`.

- [ ] **Step 5: Commit**

```bash
git add frontend/bindings frontend/src/lib/events.ts frontend/src/commands/types.ts
git commit -m "feat(frontend): bindings + evento alert:proposed + tipos de proposta"
```

---

### Task 6: Confirmação no chat e no `/note` (App.tsx, note.ts, i18n)

**Files:**
- Modify: `frontend/src/i18n.ts`
- Modify: `frontend/src/commands/builtins/note.ts`
- Test: `frontend/src/commands/builtins/note.test.ts`
- Modify: `frontend/src/App.tsx`

**Interfaces:**
- Consumes: `onAlertProposed` (Task 5), `ctx.alerts.createProposed`, `ctx.choose`, `formatFireAt`/`recurrenceLabel` (`src/lib/alerts.ts`), `tr` (`src/i18n.ts`).
- Produces: comportamento de UI; nenhuma API nova para outras tasks.

- [ ] **Step 1: Adicionar chaves de i18n**

Em `frontend/src/i18n.ts`, no dicionário `pt` (perto das outras chaves `alert_*`):

```ts
  alert_confirm: 'Criar alerta para',
  alert_yes: 'Sim',
  alert_no: 'Não',
```

No dicionário `en` (perto das `alert_*`):

```ts
  alert_confirm: 'Create an alert for',
  alert_yes: 'Yes',
  alert_no: 'No',
```

- [ ] **Step 2: Escrever o teste do `/note` (proposta de alerta)**

Em `frontend/src/commands/builtins/note.test.ts`, estenda o `mockCtx` para incluir `choose` e `alerts.createProposed`, e adicione os casos:

```ts
function mockCtx(capture?: Partial<CaptureResult>, chooseValue: string | null = 'yes') {
  const emitted: string[] = []
  const ctx = {
    lang: 'pt',
    setView: vi.fn(),
    emitSystemMessage: (m: string) => emitted.push(m),
    choose: vi.fn(async () => chooseValue),
    notes: {
      capture: vi.fn(async () => ({
        status: 'created', noteId: 1, noteTitle: 'X', tags: [], message: '', ...capture,
      } as CaptureResult)),
    },
    alerts: {
      create: vi.fn(),
      createProposed: vi.fn(async () => ({
        status: 'created', alertId: 7, message: 'ligar', fireAtLocal: '2099-04-01T09:00:00-03:00', recurrence: '',
      })),
    },
  } as unknown as CommandContext
  return { ctx, emitted, ctxAny: ctx as any }
}

it('proposes an alert when capture returns one and the user confirms', async () => {
  const { ctx, ctxAny } = mockCtx(
    { alert: { message: 'ligar', fireAt: '2099-04-01T09:00:00-03:00', recurrence: '' } },
    'yes',
  )
  await noteCommand.run(ctx, 'ligar pro médico amanhã 9h')
  expect(ctxAny.choose).toHaveBeenCalled()
  expect(ctxAny.alerts.createProposed).toHaveBeenCalledWith(
    expect.objectContaining({ message: 'ligar', noteId: 1 }),
  )
})

it('does not create the alert when the user declines', async () => {
  const { ctx, ctxAny } = mockCtx(
    { alert: { message: 'ligar', fireAt: '2099-04-01T09:00:00-03:00', recurrence: '' } },
    'no',
  )
  await noteCommand.run(ctx, 'x')
  expect(ctxAny.choose).toHaveBeenCalled()
  expect(ctxAny.alerts.createProposed).not.toHaveBeenCalled()
})

it('does not propose when capture returns no alert', async () => {
  const { ctx, ctxAny } = mockCtx({}, 'yes')
  await noteCommand.run(ctx, 'só uma nota')
  expect(ctxAny.choose).not.toHaveBeenCalled()
})
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/commands/builtins/note.test.ts`
Expected: FAIL — `note.ts` ainda não chama `choose`/`createProposed`.

- [ ] **Step 4: Implementar o `/note`**

Em `frontend/src/commands/builtins/note.ts`, adicione o import e estenda o caso `created`:

```ts
import { tr } from '../../i18n'
import { recurrenceLabel, formatFireAt } from '../../lib/alerts'
import type { Command } from '../types'
```

```ts
      case 'created': {
        const tags = res.tags?.length ? `  _${res.tags.map((t) => '#' + t).join(' ')}_` : ''
        ctx.emitSystemMessage(`${tr(ctx.lang, 'note_created')} **${res.noteTitle}**${tags}`)
        if (res.alert) {
          const when = formatFireAt(res.alert.fireAt, ctx.lang)
          const rec = recurrenceLabel(ctx.lang, res.alert.recurrence)
          const suffix = [when, rec].filter(Boolean).join(' · ')
          const ok = await ctx.choose({
            title: `${tr(ctx.lang, 'alert_confirm')} ${suffix}`,
            choices: [
              { label: tr(ctx.lang, 'alert_yes'), value: 'yes' },
              { label: tr(ctx.lang, 'alert_no'), value: 'no' },
            ],
          })
          if (ok === 'yes') {
            const ar = await ctx.alerts.createProposed({
              message: res.alert.message, fireAt: res.alert.fireAt, recurrence: res.alert.recurrence, noteId: res.noteId,
            })
            if (ar.status === 'created') {
              const w = formatFireAt(ar.fireAtLocal, ctx.lang)
              const r = recurrenceLabel(ctx.lang, ar.recurrence)
              const sfx = [w, r].filter(Boolean).join(' · ')
              ctx.emitSystemMessage(`${tr(ctx.lang, 'alert_created')} **${ar.message}**${sfx ? `  _${sfx}_` : ''}`)
            }
          }
        }
        return
      }
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/commands/builtins/note.test.ts`
Expected: PASS.

- [ ] **Step 6: Implementar o caminho do chat em App.tsx**

Em `frontend/src/App.tsx`:

Adicione os imports (junto dos demais de `./lib/...` e `./commands/...`):

```ts
import { onAlertProposed } from './lib/events'
import { recurrenceLabel, formatFireAt } from './lib/alerts'
```

> Se `App.tsx` já importa de `./lib/events` ou `./lib/alerts`, acrescente os nomes ao import existente em vez de duplicar a linha.

Logo após a definição de `const commandContext: CommandContext = { ... }`, adicione `createProposed` ao bloco `alerts` e um ref do contexto:

```ts
    alerts: {
      create: (text) => AlertsService.Create(text) as any,
      createProposed: (p) =>
        AlertsService.CreateProposed(p.message, p.fireAt, p.recurrence, p.noteId ?? null) as any,
    },
```

```ts
  // Ref sempre-atual do contexto, para efeitos (eventos) lerem a versão viva.
  const commandContextRef = useRef(commandContext)
  commandContextRef.current = commandContext
```

Adicione o efeito de proposta de alerta (perto do efeito `onAlertFired`):

```tsx
  useEffect(() => {
    const off = onAlertProposed(async (p) => {
      setStreaming(false)
      // a resposta foi uma tool call: remove a bolha vazia do assistente
      setMsgs((m) => {
        const last = m[m.length - 1]
        return last && last.role === 'assistant' && !last.content ? m.slice(0, -1) : m
      })
      const ctx = commandContextRef.current
      const when = formatFireAt(p.fireAt, ctx.lang)
      const rec = recurrenceLabel(ctx.lang, p.recurrence)
      const suffix = [when, rec].filter(Boolean).join(' · ')
      const ok = await ctx.choose({
        title: `${tr(ctx.lang, 'alert_confirm')} ${suffix}`,
        choices: [
          { label: tr(ctx.lang, 'alert_yes'), value: 'yes' },
          { label: tr(ctx.lang, 'alert_no'), value: 'no' },
        ],
      })
      if (ok !== 'yes') return
      const res = await ctx.alerts.createProposed({ message: p.message, fireAt: p.fireAt, recurrence: p.recurrence })
      if (res.status === 'created') {
        const w = formatFireAt(res.fireAtLocal, ctx.lang)
        const r = recurrenceLabel(ctx.lang, res.recurrence)
        const sfx = [w, r].filter(Boolean).join(' · ')
        ctx.emitSystemMessage(`${tr(ctx.lang, 'alert_created')} **${res.message}**${sfx ? `  _${sfx}_` : ''}`)
      }
    })
    return () => { off() }
  }, [])
```

- [ ] **Step 7: Verificar tipos, lint de tipos e testes do frontend**

Run: `cd frontend && npx tsc --noEmit && npx vitest run`
Expected: PASS (sem erros de tipo; suíte de testes verde).

- [ ] **Step 8: Commit**

```bash
git add frontend/src/i18n.ts frontend/src/commands/builtins/note.ts frontend/src/commands/builtins/note.test.ts frontend/src/App.tsx
git commit -m "feat(alerts): confirmação de alerta proposto no chat e no /note"
```

---

### Task 7: Verificação manual ponta a ponta

**Files:** nenhum (validação).

- [ ] **Step 1: Rodar a suíte completa**

Run: `go test ./... && cd frontend && npx vitest run`
Expected: tudo verde.

- [ ] **Step 2: Subir o app**

Run (raiz): `wails3 dev`
(Garanta uma chave do OpenRouter configurada em Configurações.)

- [ ] **Step 3: Cenários manuais**

Verifique cada um:

1. `/note comprar leite amanhã 9h` → nota criada **e** chip "Criar alerta para …" → "Sim" → mensagem "⏰ alerta criado"; o alerta aparece em `/alerta` (sem argumento) vinculado à nota.
2. `/note ideia: comprar leite` → só a nota; **nenhum** chip.
3. `me lembra de ligar pro médico amanhã às 9h` (sem comando) → chip de confirmação → "Sim" → alerta criado; "Não" → nada criado.
4. `qual a capital da França?` (sem comando) → resposta de chat normal, sem chip.
5. `/alerta jantar sexta 20h` → continua funcionando como antes.

- [ ] **Step 4: Finalizar**

Se algum cenário falhar, use `superpowers:systematic-debugging` antes de corrigir. Caso contrário, o trabalho está completo.

---

## Self-Review

**Cobertura do spec:**
- Conceito "alert proposal" + caminho único → Task 2 (`store`/`CreateProposed`). ✓
- Tool calling no client → Task 1. ✓
- `/note` sempre cria nota; alerta só com tempo → Task 3 (`futureOrRecurring`). ✓
- Chat decide via tool e emite `alert:proposed` → Task 4. ✓
- Confirmação antes de criar (chip `choose`) → Task 6 (chat + note). ✓
- Eventos/tipos/bindings → Task 5. ✓
- Casos de borda (past, no_api_key, sem tool call, cancelar) → cobertos em Tasks 2/4/6 e verificação Task 7. ✓
- `/alerta` inalterado → garantido por reuso de `store` (Task 2) e verificado na Task 7. ✓

**Consistência de tipos:** `AlertProposal{Message,FireAt,Recurrence}` (Go) ↔ `alert?:{message,fireAt,recurrence}` (TS); `alertProposedPayload{Message,FireAt,Recurrence}` (json `message/fireAt/recurrence`) ↔ `AlertProposedPayload` (TS). `CreateProposed(message, fireAtISO, recurrence, noteID *int64)` ↔ binding `CreateProposed(message, fireAt, recurrence, noteId)`. `StreamTools` assinatura idêntica na interface `Streamer`, nos fakes e no `*ai.Client`. ✓

**Placeholders:** nenhum — todo passo traz código/comando concretos.
