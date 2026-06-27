package app

import (
	"context"
	"strings"
	"testing"

	"gix/internal/ai"
	"gix/internal/db"
	"gix/internal/embed"
)

// fakeCompleter returns queued responses (one per call) and records the last
// messages it received.
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

// fakeEmbedder maps text to a 3-dim theme vector by keyword, so tests can assert
// semantic ranking deterministically (no model needed).
type fakeEmbedder struct{}

var themeKeywords = [3][]string{
	{"carro", "motor", "oficina", "veículo", "ruído", "barulho"}, // theme 0
	{"mercado", "pão", "leite", "compra", "compras"},             // theme 1
	{"reunião", "equipe", "time", "encontro"},                    // theme 2
}

func (fakeEmbedder) Dim() int { return 3 }

func (fakeEmbedder) embed(text string) ([]float32, error) {
	text = strings.ToLower(text)
	v := make([]float32, 3)
	for i, kws := range themeKeywords {
		for _, kw := range kws {
			if strings.Contains(text, kw) {
				v[i] = 1
				break
			}
		}
	}
	return v, nil
}

func (e fakeEmbedder) EmbedDoc(text string) ([]float32, error)   { return e.embed(text) }
func (e fakeEmbedder) EmbedQuery(text string) ([]float32, error) { return e.embed(text) }

func notesTestDB(t *testing.T) *db.Database {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/notes.db")
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
	svc := NewNotesService(NewConfigService(), d, func(string) Completer { return fake })
	svc.setEmbedder(fakeEmbedder{})
	return svc
}

// addNote inserts a note with a real fake-embedding vector, mimicking Capture's
// storage without going through the AI.
func addNote(t *testing.T, d *db.Database, title, content string, tags ...string) int64 {
	t.Helper()
	v, _ := fakeEmbedder{}.EmbedDoc(title + "\n" + content)
	id, err := d.CreateNote(title, content, tags, embed.EncodeVector(v), len(v))
	if err != nil {
		t.Fatalf("addNote: %v", err)
	}
	return id
}

func TestCaptureCreatesNote(t *testing.T) {
	d := notesTestDB(t)
	fake := &fakeCompleter{responses: []string{
		`{"title":"Carro","content":"barulho no motor do carro","tags":["carro","#Manutenção"]}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Capture("o carro tá com barulho")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if res.Status != "created" || res.NoteTitle != "Carro" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(res.Tags) != 2 || res.Tags[0] != "carro" || res.Tags[1] != "manutenção" {
		t.Fatalf("tags not normalized: %+v", res.Tags)
	}
	if vecs, _ := d.AllVectors(); len(vecs) != 1 {
		t.Fatalf("expected one stored vector, got %d", len(vecs))
	}
}

func TestCaptureFallbackTitle(t *testing.T) {
	d := notesTestDB(t)
	fake := &fakeCompleter{responses: []string{`{"content":"primeira linha\nsegunda","tags":[]}`}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Capture("algo")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if res.NoteTitle != "primeira linha" {
		t.Fatalf("expected title from content, got %q", res.NoteTitle)
	}
}

func TestCaptureNoAPIKey(t *testing.T) {
	d := notesTestDB(t)
	t.Setenv("AppData", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "")
	svc := NewNotesService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} })

	res, err := svc.Capture("algo")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if res.Status != "no_api_key" {
		t.Fatalf("expected no_api_key, got %+v", res)
	}
}

func TestFindHybridRanksAndIncludesSemanticOnly(t *testing.T) {
	d := notesTestDB(t)
	carExact := addNote(t, d, "Carro", "o motor do carro está com barulho", "carro")
	carSemantic := addNote(t, d, "Oficina", "levei o veículo na oficina pelo ruído", "manutenção")
	addNote(t, d, "Mercado", "comprei pão e leite", "compras")
	svc := newNotesSvc(t, d, &fakeCompleter{})

	results, err := svc.Find("carro")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d: %+v", len(results), results)
	}
	// carExact matches both full-text and vector → ranks first via RRF.
	if results[0].NoteID != carExact {
		t.Fatalf("expected exact car note first, got %+v", results)
	}
	// carSemantic shares no query term but is found via the vector (semantic).
	var foundSemantic, foundMarket bool
	for _, r := range results {
		if r.NoteID == carSemantic {
			foundSemantic = true
		}
		if r.Title == "Mercado" {
			foundMarket = true
		}
	}
	if !foundSemantic {
		t.Fatalf("semantic-only car note missing from results: %+v", results)
	}
	if foundMarket {
		t.Fatalf("unrelated market note should not appear for query 'carro': %+v", results)
	}
}

func TestFindFullTextOnlyWithoutEmbedder(t *testing.T) {
	d := notesTestDB(t)
	addNote(t, d, "Carro", "motor do carro com barulho", "carro")
	// Service without an embedder: must still work via full-text alone.
	svc := NewNotesService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} })

	results, err := svc.Find("carro")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 full-text result, got %+v", results)
	}
}

func TestAskSummarizesTopNotes(t *testing.T) {
	d := notesTestDB(t)
	addNote(t, d, "Carro", "o motor do carro está com barulho", "carro")
	fake := &fakeCompleter{responses: []string{"resumo: o carro está com barulho no motor"}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Ask("o que tem sobre o carro?")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res.Status != "ok" || !strings.Contains(res.Summary, "resumo") {
		t.Fatalf("unexpected ask result: %+v", res)
	}
	if len(res.Sources) == 0 {
		t.Fatalf("expected source notes, got none")
	}
	if fake.calls != 1 {
		t.Fatalf("Ask should call the AI exactly once, called %d", fake.calls)
	}
}

func TestAskEmptyWhenNoNotes(t *testing.T) {
	d := notesTestDB(t)
	fake := &fakeCompleter{responses: []string{"should not be called"}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Ask("qualquer coisa")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res.Status != "empty" {
		t.Fatalf("expected empty, got %+v", res)
	}
	if fake.calls != 0 {
		t.Fatalf("Ask must not call the AI when there are no notes")
	}
}

func TestFindDoesNotCallAI(t *testing.T) {
	d := notesTestDB(t)
	addNote(t, d, "Carro", "motor do carro", "carro")
	fake := &fakeCompleter{responses: []string{"nope"}}
	svc := newNotesSvc(t, d, fake)

	if _, err := svc.Find("carro"); err != nil {
		t.Fatalf("Find: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("Find must not call the AI, called %d", fake.calls)
	}
}

func TestRRFRewardsAppearingInBothLists(t *testing.T) {
	// id 2 is rank-1 in list A and rank-0 in list B; id 1 is rank-0 in A only.
	fused := rrf([][]int64{{1, 2}, {2, 3}}, 60)
	if len(fused) != 3 || fused[0].id != 2 {
		t.Fatalf("expected id 2 fused first, got %+v", fused)
	}
}

func TestNormalizeTags(t *testing.T) {
	got := normalizeTags([]string{"#Carro", "carro", " Manutenção ", "", "a", "b", "c", "d"})
	want := []string{"carro", "manutenção", "a", "b", "c"} // de-duped, lowercased, capped at 5
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("normalizeTags = %v, want %v", got, want)
	}
}

func TestNormalizeTagsUncappedKeepsMoreThanFive(t *testing.T) {
	got := normalizeTagsUncapped([]string{"#A", "a", " B ", "c", "d", "e", "f", "g"})
	want := []string{"a", "b", "c", "d", "e", "f", "g"} // de-duped, lowercased, no cap
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("normalizeTagsUncapped = %v, want %v", got, want)
	}
}

func TestUpdateReindexesAndDoesNotCallAI(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Carro", "motor do carro com barulho", "carro")
	fake := &fakeCompleter{responses: []string{"should not be called"}}
	svc := newNotesSvc(t, d, fake)

	// Update to a market-themed note: title+content change, tags change.
	n, err := svc.Update(id, "Mercado", "comprei pão e leite no mercado", []string{"compras", "casa"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("Update must not call the AI, called %d", fake.calls)
	}
	if n.ID != id || n.Title != "Mercado" || n.Content != "comprei pão e leite no mercado" {
		t.Fatalf("unexpected updated note: %+v", n)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "compras" || n.Tags[1] != "casa" {
		t.Fatalf("tags not updated/normalized: %v", n.Tags)
	}

	// Re-embedded: the note now ranks for a market query and not for the old car term.
	results, _ := svc.Find("mercado")
	if len(results) != 1 || results[0].NoteID != id {
		t.Fatalf("expected updated note found by new term: %+v", results)
	}
}

func TestUpdateTagsUncapped(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "A", "x", "old")
	svc := newNotesSvc(t, d, &fakeCompleter{})

	n, err := svc.Update(id, "A", "x", []string{"a", "b", "c", "d", "e", "f", "g"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(n.Tags) != 7 {
		t.Fatalf("manual edit should not cap tags, got %v", n.Tags)
	}
}

func TestUpdateFallbackTitleWhenEmpty(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Original", "conteúdo")
	svc := newNotesSvc(t, d, &fakeCompleter{})

	n, err := svc.Update(id, "   ", "primeira linha\nsegunda", nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if n.Title != "primeira linha" {
		t.Fatalf("expected title derived from content, got %q", n.Title)
	}
}

func TestUpdateWithoutEmbedderClearsVector(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Carro", "motor", "carro") // addNote stores a vector
	// Service without an embedder: an update can't re-embed, so it stores none.
	svc := NewNotesService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} })

	if _, err := svc.Update(id, "Carro", "motor novo", []string{"carro"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if vecs, _ := d.AllVectors(); len(vecs) != 0 {
		t.Fatalf("expected no vector without embedder, got %+v", vecs)
	}
}

func TestDeleteRemovesNote(t *testing.T) {
	d := notesTestDB(t)
	id := addNote(t, d, "Carro", "motor barulho", "carro")
	svc := newNotesSvc(t, d, &fakeCompleter{})

	if err := svc.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	notes, _ := svc.List()
	if len(notes) != 0 {
		t.Fatalf("expected no notes after delete, got %+v", notes)
	}
	if vecs, _ := d.AllVectors(); len(vecs) != 0 {
		t.Fatalf("vector survived delete: %+v", vecs)
	}
}

func TestCaptureDetectsAlertWhenTimeBound(t *testing.T) {
	d := notesTestDB(t)
	fake := &fakeCompleter{responses: []string{
		`{"title":"Médico","content":"consulta","tags":["saude"],"alert":{"message":"ligar pro médico","fire_at":"2099-04-01T09:00:00-03:00","recurrence":null}}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, err := svc.Capture("ligar pro médico amanhã 9h")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if res.Status != "created" {
		t.Fatalf("nota deveria ser criada, veio %+v", res)
	}
	if res.Alert == nil || res.Alert.Message != "ligar pro médico" {
		t.Fatalf("esperava proposta de alerta, veio %+v", res.Alert)
	}
	if res.Alert.FireAt != "2099-04-01T09:00:00-03:00" {
		t.Fatalf("fire_at da proposta = %q", res.Alert.FireAt)
	}
}

func TestCaptureNoAlertWhenNotTimeBound(t *testing.T) {
	d := notesTestDB(t)
	fake := &fakeCompleter{responses: []string{
		`{"title":"Ideia","content":"comprar leite","tags":["mercado"],"alert":null}`,
	}}
	svc := newNotesSvc(t, d, fake)

	res, _ := svc.Capture("ideia: comprar leite")
	if res.Status != "created" {
		t.Fatalf("nota deveria ser criada, veio %+v", res)
	}
	if res.Alert != nil {
		t.Fatalf("não deveria propor alerta, veio %+v", res.Alert)
	}
}

func TestSnippetSingleLineAndTruncates(t *testing.T) {
	if s := snippet("uma\nnota\ncurta"); s != "uma nota curta" {
		t.Fatalf("snippet flatten failed: %q", s)
	}
	long := strings.Repeat("palavra ", 60)
	if s := snippet(long); !strings.HasSuffix(s, "…") {
		t.Fatalf("expected ellipsis on long snippet: %q", s)
	}
}

func TestParseCaptureJSONStripsFences(t *testing.T) {
	dec, err := parseCaptureJSON("```json\n{\"title\":\"X\",\"content\":\"- y\",\"tags\":[\"t\"]}\n```")
	if err != nil {
		t.Fatalf("parseCaptureJSON: %v", err)
	}
	if dec.Title != "X" || dec.Content != "- y" || len(dec.Tags) != 1 {
		t.Fatalf("unexpected decode: %+v", dec)
	}
}

func TestListReturnsNotesNewestFirst(t *testing.T) {
	d := notesTestDB(t)
	addNote(t, d, "Primeira", "a")
	addNote(t, d, "Segunda", "b")
	svc := newNotesSvc(t, d, &fakeCompleter{})

	notes, err := svc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 2 || notes[0].Title != "Segunda" || notes[1].Title != "Primeira" {
		t.Fatalf("unexpected order: %+v", notes)
	}
}
