package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gix/internal/ai"
	"gix/internal/config"
	"gix/internal/db"
	"gix/internal/embed"
)

// Completer is the slice of the AI client NotesService needs: one non-streaming
// call returning the whole response (JSON). Injected for tests.
type Completer interface {
	Complete(ctx context.Context, model string, msgs []ai.Message) (string, *ai.Usage, error)
}

// Embedder produces semantic vectors for notes (passages) and searches
// (queries). Implemented by *embed.Embedder; injected so tests can fake it and
// so the service still works (full-text only) before the model finishes loading.
type Embedder interface {
	EmbedQuery(text string) ([]float32, error)
	EmbedDoc(text string) ([]float32, error)
	Dim() int
}

// Tunables for hybrid search.
const (
	candidateLimit = 30 // per-source (FTS / vector) candidates before fusion
	rrfK           = 60 // Reciprocal Rank Fusion constant
	askTopK        = 6  // notes fed to the AI for /ask
	snippetRunes   = 180
)

type NotesService struct {
	cfg       *ConfigService
	db        *db.Database
	newClient func(apiKey string) Completer
	embedder  Embedder
}

func NewNotesService(cfg *ConfigService, database *db.Database, newClient func(apiKey string) Completer) *NotesService {
	return &NotesService{cfg: cfg, db: database, newClient: newClient}
}

// setEmbedder installs the embedder once the model has loaded (see shell.go's
// background warm-up). Unexported so Wails doesn't expose it to the frontend;
// callers live in the same package. Until then semantic search is skipped and
// captures store no vector.
func (s *NotesService) setEmbedder(e Embedder) { s.embedder = e }

// List returns every note, newest first (used by the notes browser).
func (s *NotesService) List() ([]db.Note, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.db.ListNotes()
}

// --- capture ---

// CaptureResult is what the frontend gets after a /note.
//
//	"created"    note stored
//	"no_api_key" the API key is missing
//	"error"      failure (see Message)
type CaptureResult struct {
	Status    string   `json:"status"`
	NoteID    int64    `json:"noteId"`
	NoteTitle string   `json:"noteTitle"`
	Tags      []string `json:"tags"`
	Message   string   `json:"message"`
	Tokens    int      `json:"tokens"`
	Cost      float64  `json:"cost"`
}

// captureDecision is the JSON the model returns when formatting a capture.
type captureDecision struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

// Capture formats a quick note with the AI (title + Markdown body + tags),
// stores it as one atomic note, and indexes it for full-text and (if the model
// is loaded) semantic search.
func (s *NotesService) Capture(text string) (CaptureResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return CaptureResult{Status: "error", Message: "empty"}, nil
	}
	if s.db == nil {
		return CaptureResult{Status: "error", Message: "no_db"}, nil
	}

	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return CaptureResult{Status: "no_api_key"}, nil
	}

	client := s.newClient(apiKey)
	raw, usage, err := client.Complete(context.Background(), cfg.Model, buildCapturePrompt(text, time.Now()))
	if err != nil {
		return CaptureResult{Status: "error", Message: err.Error()}, nil
	}
	dec, err := parseCaptureJSON(raw)
	if err != nil {
		return CaptureResult{Status: "error", Message: err.Error()}, nil
	}

	content := strings.TrimSpace(dec.Content)
	if content == "" {
		content = text
	}
	title := strings.TrimSpace(dec.Title)
	if title == "" {
		title = db.ExtractTitle(content)
	}
	tags := normalizeTags(dec.Tags)

	var vec []byte
	dim := 0
	if s.embedder != nil {
		if v, eerr := s.embedder.EmbedDoc(title + "\n" + content); eerr == nil {
			vec = embed.EncodeVector(v)
			dim = len(v)
		}
	}

	id, err := s.db.CreateNote(title, content, tags, vec, dim)
	if err != nil {
		return CaptureResult{}, err
	}
	tokens, cost := usageCost(usage, cfg.Model)
	return CaptureResult{Status: "created", NoteID: id, NoteTitle: title, Tags: tags, Tokens: tokens, Cost: cost}, nil
}

// --- search ---

// SearchResult is one ranked note for /find and /ask. Content is included so the
// detail pane can render it without another round-trip.
type SearchResult struct {
	NoteID  int64    `json:"noteId"`
	Title   string   `json:"title"`
	Snippet string   `json:"snippet"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	Score   float64  `json:"score"`
}

// Find runs hybrid search (full-text + semantic, fused via RRF). No AI, no cost.
func (s *NotesService) Find(query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || s.db == nil {
		return nil, nil
	}

	ftsHits, err := s.db.SearchFTS(query, candidateLimit)
	if err != nil {
		return nil, err
	}
	ftsOrder := make([]int64, len(ftsHits))
	for i, h := range ftsHits {
		ftsOrder[i] = h.NoteID
	}

	vecOrder, err := s.vectorSearch(query)
	if err != nil {
		return nil, err
	}

	fused := rrf([][]int64{ftsOrder, vecOrder}, rrfK)
	if len(fused) == 0 {
		return nil, nil
	}

	ids := make([]int64, len(fused))
	for i, f := range fused {
		ids[i] = f.id
	}
	notes, err := s.db.NotesByIDs(ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]db.Note, len(notes))
	for _, n := range notes {
		byID[n.ID] = n
	}

	results := make([]SearchResult, 0, len(fused))
	for _, f := range fused {
		n, ok := byID[f.id]
		if !ok {
			continue
		}
		results = append(results, SearchResult{
			NoteID:  n.ID,
			Title:   n.Title,
			Snippet: snippet(n.Content),
			Content: n.Content,
			Tags:    n.Tags,
			Score:   f.score,
		})
	}
	return results, nil
}

// vectorSearch embeds the query and ranks all stored note vectors by cosine.
// Returns note ids best-first, or nil when the embedder isn't ready yet.
func (s *NotesService) vectorSearch(query string) ([]int64, error) {
	if s.embedder == nil {
		return nil, nil
	}
	qv, err := s.embedder.EmbedQuery(query)
	if err != nil {
		return nil, nil // degrade to FTS-only rather than failing the search
	}
	vecs, err := s.db.AllVectors()
	if err != nil {
		return nil, err
	}
	type scored struct {
		id  int64
		sim float64
	}
	ranked := make([]scored, 0, len(vecs))
	for _, v := range vecs {
		sim := embed.Cosine(qv, embed.DecodeVector(v.Vec))
		if sim <= 0 { // orthogonal/opposite carries no semantic signal
			continue
		}
		ranked = append(ranked, scored{id: v.NoteID, sim: sim})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].sim > ranked[j].sim })
	if len(ranked) > candidateLimit {
		ranked = ranked[:candidateLimit]
	}
	order := make([]int64, len(ranked))
	for i, r := range ranked {
		order[i] = r.id
	}
	return order, nil
}

// --- ask ---

// AskResult is /find plus an AI summary of the top notes.
type AskResult struct {
	Status  string         `json:"status"`
	Summary string         `json:"summary"`
	Sources []SearchResult `json:"sources"`
	Message string         `json:"message"`
	Tokens  int            `json:"tokens"`
	Cost    float64        `json:"cost"`
}

// Ask searches, then asks the AI to summarize the top notes in answer to the
// query. Returns the summary plus the source notes it drew from.
func (s *NotesService) Ask(query string) (AskResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return AskResult{Status: "error", Message: "empty"}, nil
	}

	results, err := s.Find(query)
	if err != nil {
		return AskResult{}, err
	}
	if len(results) == 0 {
		return AskResult{Status: "empty", Sources: nil}, nil
	}
	if len(results) > askTopK {
		results = results[:askTopK]
	}

	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return AskResult{Status: "no_api_key", Sources: results}, nil
	}

	client := s.newClient(apiKey)
	raw, usage, err := client.Complete(context.Background(), cfg.Model, buildSummarizePrompt(query, results))
	if err != nil {
		return AskResult{Status: "error", Message: err.Error(), Sources: results}, nil
	}
	tokens, cost := usageCost(usage, cfg.Model)
	return AskResult{
		Status:  "ok",
		Summary: strings.TrimSpace(stripFences(raw)),
		Sources: results,
		Tokens:  tokens,
		Cost:    cost,
	}, nil
}

// --- fusion ---

type fusedItem struct {
	id    int64
	score float64
}

// rrf fuses several ranked id lists with Reciprocal Rank Fusion: an item's score
// is the sum over lists of 1/(k + rank), rank being 0-based. Higher is better.
func rrf(rankings [][]int64, k float64) []fusedItem {
	scores := map[int64]float64{}
	for _, ranking := range rankings {
		for rank, id := range ranking {
			scores[id] += 1 / (k + float64(rank))
		}
	}
	out := make([]fusedItem, 0, len(scores))
	for id, sc := range scores {
		out = append(out, fusedItem{id: id, score: sc})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].id > out[j].id // stable, newest-id first on ties
	})
	return out
}

// --- helpers ---

// snippet returns a short single-line preview of a note's content.
func snippet(content string) string {
	flat := strings.Join(strings.Fields(strings.ReplaceAll(content, "\n", " ")), " ")
	r := []rune(flat)
	if len(r) > snippetRunes {
		return strings.TrimSpace(string(r[:snippetRunes])) + "…"
	}
	return flat
}

// normalizeTags trims, lowercases, drops empties and de-dupes, capping at 5.
func normalizeTags(tags []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(t, "#")))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == 5 {
			break
		}
	}
	return out
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i != -1 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

func parseCaptureJSON(s string) (captureDecision, error) {
	var dec captureDecision
	err := json.Unmarshal([]byte(stripFences(s)), &dec)
	return dec, err
}

func usageCost(usage *ai.Usage, model string) (int, float64) {
	if usage == nil {
		return 0, 0
	}
	cost := 0.0
	if p, ok := config.ModelPrices[model]; ok {
		cost = p.CalculateCost(usage.PromptTokens, usage.CompletionTokens)
	}
	return usage.TotalTokens, cost
}

// --- prompts ---

func buildCapturePrompt(text string, now time.Time) []ai.Message {
	system := fmt.Sprintf(`Você organiza anotações rápidas do usuário em uma nota atômica e bem formatada.
A data e hora atuais são: %s.
Resolva qualquer data relativa ("amanhã", "sexta") para uma data absoluta no texto.
Formate "content" como Markdown bem estruturado (parágrafo, lista, tarefa "- [ ]", ou pequena seção) — preserve a informação do usuário, sem inventar nem remover.
Gere um "title" curto (sem marcadores Markdown) e de 1 a 5 "tags" temáticas, minúsculas, sem "#".
Responda APENAS com JSON, sem cercas:
{"title":"<título curto>","content":"<Markdown da nota>","tags":["tag1","tag2"]}`,
		now.Format("2006-01-02 15:04 (Monday)"))
	return []ai.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: text},
	}
}

func buildSummarizePrompt(query string, results []SearchResult) []ai.Message {
	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "[%d] %s\n%s\n---\n", i+1, r.Title, r.Content)
	}
	system := `Você responde à pergunta do usuário usando APENAS as anotações fornecidas abaixo.
Resuma de forma direta em Markdown. Não invente informação que não esteja nas notas.
Se as notas não responderem à pergunta, diga isso claramente.`
	user := fmt.Sprintf("Pergunta:\n%s\n\nAnotações:\n%s", query, b.String())
	return []ai.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
}
