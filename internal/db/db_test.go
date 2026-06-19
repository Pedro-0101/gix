package db

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *Database {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestConversationLifecycle(t *testing.T) {
	d := openTestDB(t)

	id, err := d.CreateConversation("Primeira pergunta", "modelo:free")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if err := d.AddMessage(id, "user", "oi"); err != nil {
		t.Fatalf("AddMessage user: %v", err)
	}
	if err := d.AddMessage(id, "assistant", "olá"); err != nil {
		t.Fatalf("AddMessage assistant: %v", err)
	}

	msgs, err := d.GetMessages(id)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Content != "olá" {
		t.Fatalf("mensagens inesperadas: %+v", msgs)
	}

	convs, err := d.ListConversations()
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 1 || convs[0].Title != "Primeira pergunta" || convs[0].Model != "modelo:free" {
		t.Fatalf("conversas inesperadas: %+v", convs)
	}

	if err := d.DeleteConversation(id); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	convs, _ = d.ListConversations()
	if len(convs) != 0 {
		t.Fatalf("esperava 0 conversas, veio %d", len(convs))
	}
	msgs, _ = d.GetMessages(id)
	if len(msgs) != 0 {
		t.Fatalf("esperava 0 mensagens apos delete, veio %d", len(msgs))
	}
}

func TestExtractTitle(t *testing.T) {
	cases := map[string]string{
		"Olá mundo":                 "Olá mundo",
		"  primeira\nsegunda linha": "primeira",
		"":                          "Conversa",
	}
	for in, want := range cases {
		if got := ExtractTitle(in); got != want {
			t.Errorf("ExtractTitle(%q) = %q, want %q", in, got, want)
		}
	}
	long := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz" // 50 chars
	got := ExtractTitle(long)
	if []rune(got)[len([]rune(got))-1] != '…' {
		t.Errorf("ExtractTitle(longo) deveria terminar com reticências: %q", got)
	}
}
