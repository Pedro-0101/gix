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
