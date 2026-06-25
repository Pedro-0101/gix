package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"gix/internal/ai"
	"gix/internal/db"
)

// fakeCompleter devolve respostas enfileiradas (uma por chamada) e guarda a
// última lista de mensagens recebida, para inspeção nos testes.
type fakeCompleter struct {
	responses []string
	calls     int
	lastMsgs  []ai.Message
}

func (f *fakeCompleter) Complete(ctx context.Context, model string, msgs []ai.Message) (string, *ai.Usage, error) {
	f.lastMsgs = msgs
	r := ""
	if len(f.responses) > 0 {
		r = f.responses[f.calls%len(f.responses)]
	}
	f.calls++
	return r, &ai.Usage{TotalTokens: 5}, nil
}

func notesTestDB(t *testing.T) *db.Database {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "notes.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func newNotesSvc(t *testing.T, d *db.Database, fake Completer) *NotesService {
	t.Helper()
	t.Setenv("AppData", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "k")
	return NewNotesService(NewConfigService(), d, func(string) Completer { return fake })
}

func TestRouteCreatesNewNote(t *testing.T) {
	d := notesTestDB(t)
	fake := &fakeCompleter{responses: []string{
		`{"action":"create","title":"Compras","formatted_item":"- comprar shampoo (26/06 manhã)"}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Route("comprar shampoo amanhã de manhã")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Status != "created" || res.NoteTitle != "Compras" {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	notes, _ := d.ListNotes()
	if len(notes) != 1 || !strings.Contains(notes[0].Content, "shampoo") {
		t.Fatalf("nota não criada corretamente: %+v", notes)
	}
}

func TestRouteAppendsToExistingNote(t *testing.T) {
	d := notesTestDB(t)
	id, _ := d.CreateNote("Lembretes", "- pagar conta", 0, "append")
	fake := &fakeCompleter{responses: []string{
		fmt.Sprintf(`{"action":"append","note_id":%d,"formatted_item":"- comprar shampoo"}`, id),
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Route("comprar shampoo")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Status != "appended" || res.NoteID != id {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	n, _ := d.GetNote(id)
	if !strings.Contains(n.Content, "shampoo") || !strings.Contains(n.Content, "pagar conta") {
		t.Fatalf("anexo não preservou o conteúdo: %q", n.Content)
	}
}

func TestRouteReturnsFullWhenLimitExceeded(t *testing.T) {
	d := notesTestDB(t)
	// Nota com limite próprio de 2 linhas, já com 2 linhas.
	id, _ := d.CreateNote("Cheia", "l1\nl2", 2, "append")
	fake := &fakeCompleter{responses: []string{
		fmt.Sprintf(`{"action":"append","note_id":%d,"formatted_item":"l3"}`, id),
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Route("l3")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Status != "full" || res.NoteID != id {
		t.Fatalf("esperava status full: %+v", res)
	}
	n, _ := d.GetNote(id)
	if n.Content != "l1\nl2" {
		t.Fatalf("nota cheia não deveria ter sido gravada: %q", n.Content)
	}
}

func TestRouteNoAPIKey(t *testing.T) {
	d := notesTestDB(t)
	t.Setenv("AppData", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "")
	svc := NewNotesService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} })

	res, err := svc.Route("algo")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Status != "no_api_key" {
		t.Fatalf("esperava no_api_key, veio %+v", res)
	}
}

func TestResolveOverflowPart2(t *testing.T) {
	d := notesTestDB(t)
	id, _ := d.CreateNote("Cheia", "l1\nl2", 2, "append")
	svc := newNotesSvc(t, d, &fakeCompleter{})

	res, err := svc.ResolveOverflow(id, "novo item", "part2")
	if err != nil {
		t.Fatalf("ResolveOverflow: %v", err)
	}
	if res.Status != "created" {
		t.Fatalf("esperava created: %+v", res)
	}
	notes, _ := d.ListNotes()
	var found bool
	for _, n := range notes {
		if n.Title == "Cheia 2" && strings.Contains(n.Content, "novo item") {
			found = true
		}
	}
	if len(notes) != 2 || !found {
		t.Fatalf("parte 2 não criada: %+v", notes)
	}
}

func TestResolveOverflowSummarize(t *testing.T) {
	d := notesTestDB(t)
	id, _ := d.CreateNote("Cheia", "l1\nl2\nl3", 2, "append")
	fake := &fakeCompleter{responses: []string{"resumo conciso\n- novo item"}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.ResolveOverflow(id, "novo item", "summarize")
	if err != nil {
		t.Fatalf("ResolveOverflow: %v", err)
	}
	if res.Status != "appended" {
		t.Fatalf("esperava appended: %+v", res)
	}
	n, _ := d.GetNote(id)
	if n.Content != "resumo conciso\n- novo item" {
		t.Fatalf("resumo não gravado: %q", n.Content)
	}
}

func TestParseRouteJSONStripsFences(t *testing.T) {
	dec, err := parseRouteJSON("```json\n{\"action\":\"create\",\"title\":\"X\",\"formatted_item\":\"- y\"}\n```")
	if err != nil {
		t.Fatalf("parseRouteJSON: %v", err)
	}
	if dec.Action != "create" || dec.Title != "X" || dec.FormattedItem != "- y" {
		t.Fatalf("decodificação inesperada: %+v", dec)
	}
}

func TestExceedsLimit(t *testing.T) {
	if !exceedsLimit("a\nb\nc", 2) {
		t.Error("3 linhas deveriam exceder limite 2")
	}
	if exceedsLimit("a\nb", 2) {
		t.Error("2 linhas não excedem limite 2")
	}
	if exceedsLimit("a\nb\nc\nd", 0) {
		t.Error("limite 0 significa ilimitado")
	}
}
