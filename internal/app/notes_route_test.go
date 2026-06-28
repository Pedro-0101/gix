package app

import (
	"strings"
	"testing"

	"gix/internal/db"
)

func TestValidAttach(t *testing.T) {
	cands := []db.Note{{ID: 3}, {ID: 7}}
	id7 := int64(7)
	id9 := int64(9)

	if _, ok := validAttach(nil, cands); ok {
		t.Fatalf("nil attach_to should not match")
	}
	if n, ok := validAttach(&id7, cands); !ok || n.ID != 7 {
		t.Fatalf("expected match on id 7, got %+v ok=%v", n, ok)
	}
	if _, ok := validAttach(&id9, cands); ok {
		t.Fatalf("id absent from candidates must not match (hallucination guard)")
	}
}

func TestCaptureProposesAttachToSimilarNote(t *testing.T) {
	d := notesTestDB(t)
	target := addNote(t, d, "Carro", "o motor do carro está com barulho", "carro") // id 1
	fake := &fakeCompleter{responses: []string{
		`{"title":"Carro - oficina","content":"levar na oficina sexta","tags":["carro"],"attach_to":1}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Capture("o carro também está fazendo um ruído no motor")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if res.Status != "attach_proposed" {
		t.Fatalf("expected attach_proposed, got %+v", res)
	}
	if res.Attach == nil || res.Attach.TargetID != target {
		t.Fatalf("expected attach to target %d, got %+v", target, res.Attach)
	}
	if res.Content == "" {
		t.Fatalf("attach proposal must carry the formatted content for create-instead")
	}
	// Nothing written yet: still exactly one note, and its content is unchanged.
	notes, _ := svc.List()
	if len(notes) != 1 {
		t.Fatalf("attach proposal must not write; got %d notes", len(notes))
	}
	if strings.Contains(notes[0].Content, "oficina") {
		t.Fatalf("target note should be untouched until confirmed: %q", notes[0].Content)
	}
}

func TestCaptureIgnoresHallucinatedAttachId(t *testing.T) {
	d := notesTestDB(t)
	addNote(t, d, "Carro", "o motor do carro está com barulho", "carro")
	fake := &fakeCompleter{responses: []string{
		`{"title":"Carro","content":"barulho novo","tags":["carro"],"attach_to":999}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Capture("o carro está com um barulho diferente no motor")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if res.Status != "created" {
		t.Fatalf("an attach_to absent from candidates must fall back to create, got %+v", res)
	}
	if notes, _ := svc.List(); len(notes) != 2 {
		t.Fatalf("expected a new note created, got %d", len(notes))
	}
}

func TestCaptureCreatesWhenNoCandidates(t *testing.T) {
	d := notesTestDB(t)
	// Empty db → no candidates → model can't attach even if it tried.
	fake := &fakeCompleter{responses: []string{
		`{"title":"Ideia","content":"uma ideia nova","tags":["ideia"],"attach_to":1}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Capture("uma ideia nova qualquer")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if res.Status != "created" {
		t.Fatalf("expected created with no candidates, got %+v", res)
	}
}

func TestAppendToMergesContentAndTags(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Carro", "barulho no motor", "carro")
	svc := newNotesSvc(t, d, &fakeCompleter{})

	res, err := svc.AppendTo(id, "também vazando óleo", []string{"manutenção", "carro"})
	if err != nil {
		t.Fatalf("AppendTo: %v", err)
	}
	if res.Status != "attached" || res.NoteID != id || res.NoteTitle != "Carro" {
		t.Fatalf("unexpected append result: %+v", res)
	}
	// Tags are the union, de-duped (carro appears in both).
	if strings.Join(res.Tags, ",") != "carro,manutenção" {
		t.Fatalf("expected unioned tags, got %v", res.Tags)
	}
	n, _ := d.GetNote(id)
	if !strings.Contains(n.Content, "barulho no motor") || !strings.Contains(n.Content, "vazando óleo") {
		t.Fatalf("merged content missing a part: %q", n.Content)
	}
	// Re-embedded so search stays in sync.
	if vecs, _ := d.AllVectors(); len(vecs) != 1 {
		t.Fatalf("expected the note's vector refreshed, got %d", len(vecs))
	}
}

func TestAppendToMissingNote(t *testing.T) {
	d := notesTestDB(t)
	svc := newNotesSvc(t, d, &fakeCompleter{})

	res, err := svc.AppendTo(999, "algo", nil)
	if err != nil {
		t.Fatalf("AppendTo: %v", err)
	}
	if res.Status != "error" {
		t.Fatalf("expected error for missing note, got %+v", res)
	}
}
