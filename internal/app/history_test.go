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
