package db

import (
	"database/sql"
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

func TestNoteLifecycle(t *testing.T) {
	d := openTestDB(t)

	id, err := d.CreateNote("Lembretes", "- comprar shampoo", 10, "append")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	n, err := d.GetNote(id)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if n.Title != "Lembretes" || n.Content != "- comprar shampoo" || n.LineLimit != 10 || n.IntegrationMode != "append" {
		t.Fatalf("nota inesperada: %+v", n)
	}
	if n.CreatedAt == "" {
		t.Fatalf("CreatedAt vazio: %+v", n)
	}

	if err := d.UpdateNoteContent(id, "- comprar shampoo\n- pagar conta"); err != nil {
		t.Fatalf("UpdateNoteContent: %v", err)
	}
	n, _ = d.GetNote(id)
	if n.Content != "- comprar shampoo\n- pagar conta" {
		t.Fatalf("conteúdo não atualizado: %q", n.Content)
	}
	if n.UpdatedAt == "" {
		t.Fatalf("UpdatedAt deveria ser preenchido após update: %+v", n)
	}

	// Uma segunda nota para validar a listagem.
	if _, err := d.CreateNote("Ideias", "- app de notas", 0, ""); err != nil {
		t.Fatalf("CreateNote 2: %v", err)
	}
	notes, err := d.ListNotes()
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("esperava 2 notas, veio %d", len(notes))
	}

	if err := d.DeleteNote(id); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}
	notes, _ = d.ListNotes()
	if len(notes) != 1 || notes[0].Title != "Ideias" {
		t.Fatalf("após delete esperava só 'Ideias', veio %+v", notes)
	}
}

// Abrir um banco legado (tabela notes sem as colunas novas) e reabrir deve
// migrar sem erro e preservar os dados — a migração é idempotente.
func TestNotesMigrationIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	// Simula um banco antigo: tabela notes só com as colunas originais.
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	_, err = legacy.Exec(`CREATE TABLE notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("create legacy notes: %v", err)
	}
	if _, err := legacy.Exec(`INSERT INTO notes (title, content) VALUES ('Velha', 'conteudo')`); err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	legacy.Close()

	// Primeira abertura migra; segunda abertura não deve falhar (idempotente).
	for i := range 2 {
		d, err := Open(path)
		if err != nil {
			t.Fatalf("Open #%d migrou com erro: %v", i, err)
		}
		notes, err := d.ListNotes()
		if err != nil {
			t.Fatalf("ListNotes #%d: %v", i, err)
		}
		if len(notes) != 1 || notes[0].Title != "Velha" || notes[0].Content != "conteudo" {
			t.Fatalf("dados legados perdidos na migração: %+v", notes)
		}
		// As colunas novas existem e caem no default ("usar global").
		if notes[0].LineLimit != 0 || notes[0].IntegrationMode != "" {
			t.Fatalf("defaults de migração inesperados: %+v", notes[0])
		}
		d.Close()
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
