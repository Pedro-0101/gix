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

func TestChatServiceSendEmitsSequence(t *testing.T) {
	t.Setenv("AppData", t.TempDir())

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

type blockingStreamer struct {
	entered chan struct{}
	release chan struct{}
}

func (b *blockingStreamer) StreamTools(ctx context.Context, model string, msgs []ai.Message, tools []ai.Tool, onDelta func(string)) (*ai.Usage, []ai.ToolCall, error) {
	close(b.entered)
	<-b.release
	return &ai.Usage{}, nil, nil
}

func TestChatServiceSecondSendWhileStreamingIsNoop(t *testing.T) {
	t.Setenv("AppData", t.TempDir())
	d, err := db.Open(filepath.Join(t.TempDir(), "c2.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	emit := func(name string, data any) {}

	cfgSvc := NewConfigService()
	cur := cfgSvc.Current()
	cur.APIKey = "k"
	_ = cfgSvc.Save(*cur)

	bs := &blockingStreamer{entered: make(chan struct{}), release: make(chan struct{})}
	s := NewChatService(cfgSvc, d, emit, func(string) Streamer { return bs })

	s.Send("primeira")
	<-bs.entered // first stream is now in-flight; streaming == true

	s.Send("segunda") // must be a no-op

	convs, _ := d.ListConversations()
	if len(convs) != 1 {
		t.Fatalf("esperava 1 conversa enquanto streaming, veio %d", len(convs))
	}

	close(bs.release) // let the first stream finish and persist

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msgs, _ := d.GetMessages(convs[0].ID)
		if len(msgs) >= 2 { // user + assistant persisted => goroutine done
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	msgs, _ := d.GetMessages(convs[0].ID)
	for _, m := range msgs {
		if m.Content == "segunda" {
			t.Fatal("a segunda mensagem nao deveria ter sido persistida")
		}
	}
}

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
