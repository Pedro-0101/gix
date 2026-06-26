package db

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *Database {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open (does modernc.org/sqlite have FTS5?): %v", err)
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

	if err := d.DeleteConversation(id); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	if convs, _ := d.ListConversations(); len(convs) != 0 {
		t.Fatalf("esperava 0 conversas, veio %d", len(convs))
	}
}

func TestCreateAndGetNote(t *testing.T) {
	d := openTestDB(t)
	id, err := d.CreateNote("Carro", "barulho no motor do carro", []string{"carro", "manutenção"}, []byte{1, 2, 3, 4}, 1)
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	n, err := d.GetNote(id)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if n.Title != "Carro" || n.Content != "barulho no motor do carro" {
		t.Fatalf("unexpected note: %+v", n)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "carro" || n.Tags[1] != "manutenção" {
		t.Fatalf("tags = %v, want [carro manutenção]", n.Tags)
	}
	if n.CreatedAt == "" {
		t.Fatalf("CreatedAt vazio: %+v", n)
	}
}

func TestSearchFTSMatchesAndRanks(t *testing.T) {
	d := openTestDB(t)
	carID, _ := d.CreateNote("Carro", "o motor do carro está com barulho", []string{"carro"}, nil, 0)
	d.CreateNote("Mercado", "comprei pão e leite", []string{"compras"}, nil, 0)

	hits, err := d.SearchFTS("motor carro", 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(hits) != 1 || hits[0].NoteID != carID {
		t.Fatalf("expected only the car note, got %+v", hits)
	}
}

func TestSearchFTSRemovesDiacritics(t *testing.T) {
	d := openTestDB(t)
	id, _ := d.CreateNote("Ruído", "barulho e ruído no motor", []string{"carro"}, nil, 0)

	// Query without the accent must still match the accented content.
	hits, err := d.SearchFTS("ruido", 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(hits) != 1 || hits[0].NoteID != id {
		t.Fatalf("diacritic-insensitive match failed: %+v", hits)
	}
}

func TestSearchFTSEmptyQuery(t *testing.T) {
	d := openTestDB(t)
	d.CreateNote("A", "algo", nil, nil, 0)
	hits, err := d.SearchFTS("  !?  ", 10)
	if err != nil {
		t.Fatalf("SearchFTS empty: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no hits for empty query, got %+v", hits)
	}
}

func TestListAndNotesByIDs(t *testing.T) {
	d := openTestDB(t)
	a, _ := d.CreateNote("A", "primeira", []string{"x"}, nil, 0)
	b, _ := d.CreateNote("B", "segunda", nil, nil, 0)

	notes, err := d.ListNotes()
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 2 || notes[0].ID != b { // newest first
		t.Fatalf("ListNotes order/len wrong: %+v", notes)
	}

	got, err := d.NotesByIDs([]int64{a})
	if err != nil {
		t.Fatalf("NotesByIDs: %v", err)
	}
	if len(got) != 1 || got[0].ID != a || len(got[0].Tags) != 1 {
		t.Fatalf("NotesByIDs wrong: %+v", got)
	}
}

func TestAllVectors(t *testing.T) {
	d := openTestDB(t)
	id, _ := d.CreateNote("A", "x", nil, []byte{9, 8, 7, 6}, 1)
	d.CreateNote("B", "y", nil, nil, 0) // no vector

	vecs, err := d.AllVectors()
	if err != nil {
		t.Fatalf("AllVectors: %v", err)
	}
	if len(vecs) != 1 || vecs[0].NoteID != id || string(vecs[0].Vec) != string([]byte{9, 8, 7, 6}) {
		t.Fatalf("unexpected vectors: %+v", vecs)
	}
}

func TestDeleteNoteRemovesEverything(t *testing.T) {
	d := openTestDB(t)
	id, _ := d.CreateNote("A", "motor barulho", []string{"carro"}, []byte{1, 2, 3, 4}, 1)
	if err := d.DeleteNote(id); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}

	if _, err := d.GetNote(id); err == nil {
		t.Fatal("note still present after delete")
	}
	if hits, _ := d.SearchFTS("motor", 10); len(hits) != 0 {
		t.Fatalf("fts row survived delete: %+v", hits)
	}
	if vecs, _ := d.AllVectors(); len(vecs) != 0 {
		t.Fatalf("vector survived delete: %+v", vecs)
	}
}

func TestUpdateNotePreservesIdentityAndReindexes(t *testing.T) {
	d := openTestDB(t)
	id, _ := d.CreateNote("Carro", "motor com barulho", []string{"carro"}, []byte{1, 2, 3, 4}, 1)
	before, _ := d.GetNote(id)

	if err := d.UpdateNote(id, "Bicicleta", "pneu furado da bicicleta", []string{"bike", "lazer"}, []byte{5, 6, 7, 8}, 1); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	n, err := d.GetNote(id)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if n.ID != id {
		t.Fatalf("id changed: %d -> %d", id, n.ID)
	}
	if n.CreatedAt != before.CreatedAt {
		t.Fatalf("created_at changed: %q -> %q", before.CreatedAt, n.CreatedAt)
	}
	if n.Title != "Bicicleta" || n.Content != "pneu furado da bicicleta" {
		t.Fatalf("title/content not updated: %+v", n)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "bike" || n.Tags[1] != "lazer" {
		t.Fatalf("tags not replaced: %v", n.Tags)
	}

	// FTS reflects the new text: old term gone, new term found.
	if hits, _ := d.SearchFTS("motor", 10); len(hits) != 0 {
		t.Fatalf("old fts term still matches: %+v", hits)
	}
	if hits, _ := d.SearchFTS("bicicleta", 10); len(hits) != 1 || hits[0].NoteID != id {
		t.Fatalf("new fts term not found: %+v", hits)
	}

	// Vector replaced.
	vecs, _ := d.AllVectors()
	if len(vecs) != 1 || string(vecs[0].Vec) != string([]byte{5, 6, 7, 8}) {
		t.Fatalf("vector not replaced: %+v", vecs)
	}
}

func TestUpdateNoteClearsVectorWhenEmpty(t *testing.T) {
	d := openTestDB(t)
	id, _ := d.CreateNote("A", "x", nil, []byte{1, 2, 3, 4}, 1)

	if err := d.UpdateNote(id, "A", "x editado", nil, nil, 0); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}
	if vecs, _ := d.AllVectors(); len(vecs) != 0 {
		t.Fatalf("expected no vector after empty update, got %+v", vecs)
	}
}

func ptr(i int64) *int64 { return &i }

func TestAlertCRUDAndDue(t *testing.T) {
	d := openTestDB(t)
	base := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	past := Alert{Message: "ligar pro médico", FireAt: base.Add(-time.Hour)}
	pastID, err := d.CreateAlert(past)
	if err != nil {
		t.Fatalf("CreateAlert past: %v", err)
	}
	future := Alert{Message: "academia", NoteID: ptr(7), FireAt: base.Add(time.Hour), Recurrence: `{"freq":"weekly","interval":1}`}
	futureID, err := d.CreateAlert(future)
	if err != nil {
		t.Fatalf("CreateAlert future: %v", err)
	}

	// GetAlert round-trips fields (fire_at compared at second precision).
	got, err := d.GetAlert(futureID)
	if err != nil {
		t.Fatalf("GetAlert: %v", err)
	}
	if got.Message != "academia" || got.NoteID == nil || *got.NoteID != 7 || got.Status != "pending" {
		t.Fatalf("unexpected alert: %+v", got)
	}
	if !got.FireAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("fire_at not preserved: %v want %v", got.FireAt, base.Add(time.Hour))
	}

	// DueAlerts returns only the past pending alert.
	due, err := d.DueAlerts(base)
	if err != nil {
		t.Fatalf("DueAlerts: %v", err)
	}
	if len(due) != 1 || due[0].ID != pastID {
		t.Fatalf("expected only the past alert due, got %+v", due)
	}

	// ListAlerts(pending) returns both, fire_at asc (past first).
	pend, _ := d.ListAlerts("pending")
	if len(pend) != 2 || pend[0].ID != pastID {
		t.Fatalf("ListAlerts order wrong: %+v", pend)
	}

	// Reschedule the past one into the future; it leaves the due set.
	if err := d.UpdateAlertFireAt(pastID, base.Add(48*time.Hour)); err != nil {
		t.Fatalf("UpdateAlertFireAt: %v", err)
	}
	if due, _ := d.DueAlerts(base); len(due) != 0 {
		t.Fatalf("expected nothing due after reschedule, got %+v", due)
	}

	// SetAlertStatus moves it out of pending; DeleteAlert removes it.
	if err := d.SetAlertStatus(futureID, "done"); err != nil {
		t.Fatalf("SetAlertStatus: %v", err)
	}
	if pend, _ := d.ListAlerts("pending"); len(pend) != 1 {
		t.Fatalf("expected 1 pending after done, got %+v", pend)
	}
	if err := d.DeleteAlert(pastID); err != nil {
		t.Fatalf("DeleteAlert: %v", err)
	}
	if all, _ := d.ListAlerts(); len(all) != 1 {
		t.Fatalf("expected 1 alert after delete, got %+v", all)
	}
}

func TestExtractTitle(t *testing.T) {
	cases := map[string]string{
		"Olá mundo":                 "Olá mundo",
		"  primeira\nsegunda linha": "primeira",
		"":                          "Nota",
		"## Cabeçalho":              "Cabeçalho",
		"- [ ] tarefa":              "tarefa",
	}
	for in, want := range cases {
		if got := ExtractTitle(in); got != want {
			t.Errorf("ExtractTitle(%q) = %q, want %q", in, got, want)
		}
	}
	long := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz" // 50 chars
	if got := ExtractTitle(long); []rune(got)[len([]rune(got))-1] != '…' {
		t.Errorf("ExtractTitle(longo) deveria terminar com reticências: %q", got)
	}
}
