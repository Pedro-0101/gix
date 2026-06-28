package app

import (
	"strings"
	"testing"

	"gix/internal/db"
)

func TestNextPartTitle(t *testing.T) {
	cases := map[string]string{
		"Carro":            "Carro · parte 2",
		"Carro · parte 2":  "Carro · parte 3",
		"Carro · parte 10": "Carro · parte 11",
	}
	for in, want := range cases {
		if got := nextPartTitle(in); got != want {
			t.Errorf("nextPartTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEffectiveLimit(t *testing.T) {
	svc := newNotesSvc(t, notesTestDB(t), &fakeCompleter{}) // global default 8000
	if got := svc.effectiveLimit(db.Note{CharLimit: 0}); got != 8000 {
		t.Fatalf("no override should fall back to global 8000, got %d", got)
	}
	if got := svc.effectiveLimit(db.Note{CharLimit: 120}); got != 120 {
		t.Fatalf("override should win, got %d", got)
	}
}

func TestAppendToProposesOverflow(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Carro", "barulho no motor", "carro")
	if err := d.SetNoteCharLimit(id, 20); err != nil {
		t.Fatalf("SetNoteCharLimit: %v", err)
	}
	svc := newNotesSvc(t, d, &fakeCompleter{})

	res, err := svc.AppendTo(id, "também está vazando óleo por toda parte", []string{"manutenção"})
	if err != nil {
		t.Fatalf("AppendTo: %v", err)
	}
	if res.Status != "overflow_proposed" {
		t.Fatalf("expected overflow_proposed, got %+v", res)
	}
	if res.Overflow == nil || res.Overflow.TargetID != id || res.Overflow.Limit != 20 {
		t.Fatalf("overflow proposal not populated: %+v", res.Overflow)
	}
	if res.Overflow.Length <= res.Overflow.Limit {
		t.Fatalf("projected length %d should exceed limit %d", res.Overflow.Length, res.Overflow.Limit)
	}
	// The capture content/tags ride along so the frontend can resolve without re-asking the AI.
	if res.Content == "" {
		t.Fatalf("overflow proposal must carry the pending content")
	}
	// Nothing written: the note is untouched.
	n, _ := d.GetNote(id)
	if strings.Contains(n.Content, "óleo") {
		t.Fatalf("overflow proposal must not write to the note: %q", n.Content)
	}
}

func TestResolveOverflowPart2(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Carro", "barulho no motor", "carro")
	svc := newNotesSvc(t, d, &fakeCompleter{})

	res, err := svc.ResolveOverflow(id, "vazando óleo", []string{"manutenção"}, "part2")
	if err != nil {
		t.Fatalf("ResolveOverflow: %v", err)
	}
	if res.Status != "created" || res.NoteTitle != "Carro · parte 2" {
		t.Fatalf("expected a 'parte 2' sibling, got %+v", res)
	}
	// Original untouched, sibling added → two notes.
	notes, _ := svc.List()
	if len(notes) != 2 {
		t.Fatalf("expected original + sibling, got %d notes", len(notes))
	}
	orig, _ := d.GetNote(id)
	if strings.Contains(orig.Content, "óleo") {
		t.Fatalf("part2 must leave the original note unchanged: %q", orig.Content)
	}
}

func TestResolveOverflowSummarize(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Carro", "barulho no motor do carro", "carro")
	fake := &fakeCompleter{responses: []string{"resumo enxuto do carro"}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.ResolveOverflow(id, "também vazando óleo", []string{"manutenção"}, "summarize")
	if err != nil {
		t.Fatalf("ResolveOverflow: %v", err)
	}
	if res.Status != "attached" || res.NoteID != id {
		t.Fatalf("expected the note condensed in place, got %+v", res)
	}
	n, _ := d.GetNote(id)
	if n.Content != "resumo enxuto do carro" {
		t.Fatalf("note body should be the summary, got %q", n.Content)
	}
	if strings.Join(n.Tags, ",") != "carro,manutenção" {
		t.Fatalf("tags should be the union, got %v", n.Tags)
	}
}

func TestResolveOverflowSplit(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Tudo junto", "barulho no motor", "geral")
	fake := &fakeCompleter{responses: []string{
		`{"notes":[{"title":"Motor","content":"barulho no motor","tags":["carro"]},{"title":"Compras","content":"comprar leite","tags":["mercado"]}]}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.ResolveOverflow(id, "comprar leite", nil, "split")
	if err != nil {
		t.Fatalf("ResolveOverflow: %v", err)
	}
	if res.Status != "split" || res.Count != 2 {
		t.Fatalf("expected split into 2, got %+v", res)
	}
	// Original replaced by the two themed notes.
	notes, _ := svc.List()
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes after split, got %d", len(notes))
	}
	if _, err := d.GetNote(id); err == nil {
		t.Fatalf("the original note should be gone after a split")
	}
}

func TestResolveOverflowSplitFallsBackOnUnusableJSON(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Carro", "barulho no motor", "carro")
	fake := &fakeCompleter{responses: []string{"not json at all"}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.ResolveOverflow(id, "vazando óleo", nil, "split")
	if err != nil {
		t.Fatalf("ResolveOverflow: %v", err)
	}
	if res.Status != "attached" || res.NoteID != id {
		t.Fatalf("unusable split must fall back to keeping one note, got %+v", res)
	}
	n, _ := d.GetNote(id)
	if !strings.Contains(n.Content, "barulho no motor") || !strings.Contains(n.Content, "óleo") {
		t.Fatalf("fallback must preserve all content: %q", n.Content)
	}
}

func TestResolveOverflowUnknownMode(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Carro", "barulho", "carro")
	svc := newNotesSvc(t, d, &fakeCompleter{})

	res, _ := svc.ResolveOverflow(id, "x", nil, "bogus")
	if res.Status != "error" {
		t.Fatalf("unknown mode should error, got %+v", res)
	}
}
