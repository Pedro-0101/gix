package db

import (
	"path/filepath"
	"testing"
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
