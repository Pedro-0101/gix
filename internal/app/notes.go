package app

import (
	"context"
	"fmt"
	"strings"

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

// Update replaces a note's title, content and tags exactly as the user typed
// them — no AI, no cost. The text is re-embedded locally (free) when the model
// is loaded so semantic search stays in sync; otherwise the note keeps no vector
// and degrades to full-text. Tags are normalized but uncapped (manual edit).
// Returns the updated note for the UI to re-render.
func (s *NotesService) Update(id int64, title, content string, tags []string) (db.Note, error) {
	if s.db == nil {
		return db.Note{}, fmt.Errorf("no_db")
	}
	content = strings.TrimSpace(content)
	title = strings.TrimSpace(title)
	if title == "" {
		title = db.ExtractTitle(content)
	}
	normTags := normalizeTagsUncapped(tags)

	if err := s.storeNoteBody(id, title, content, normTags); err != nil {
		return db.Note{}, err
	}
	return s.db.GetNote(id)
}

// SetCharLimit sets a per-note size override in characters; 0 clears it so the
// note falls back to the global default (config.NoteCharLimit). Negative values
// are clamped to 0.
func (s *NotesService) SetCharLimit(id int64, limit int) error {
	if s.db == nil {
		return fmt.Errorf("no_db")
	}
	if limit < 0 {
		limit = 0
	}
	return s.db.SetNoteCharLimit(id, limit)
}

// Delete removes a note and all of its derived rows (tags, vector, FTS).
func (s *NotesService) Delete(id int64) error {
	if s.db == nil {
		return fmt.Errorf("no_db")
	}
	return s.db.DeleteNote(id)
}

// --- shared helpers ---

// embedFor returns the serialized embedding vector and its dimension for a note's
// title+content, or (nil, 0) when the embedder isn't loaded yet or fails. Callers
// pass the result straight to db.CreateNote/UpdateNote, which store no vector for
// an empty blob (semantic search degrades to full-text until the model warms up).
func (s *NotesService) embedFor(title, content string) ([]byte, int) {
	if s.embedder == nil {
		return nil, 0
	}
	v, err := s.embedder.EmbedDoc(title + "\n" + content)
	if err != nil {
		return nil, 0
	}
	return embed.EncodeVector(v), len(v)
}

// storeNoteBody re-embeds and writes title/content/tags over an existing note,
// keeping FTS/vector in sync. Shared by the overflow strategies that replace a
// note's body.
func (s *NotesService) storeNoteBody(id int64, title, content string, tags []string) error {
	vec, dim := s.embedFor(title, content)
	return s.db.UpdateNote(id, title, content, tags, vec, dim)
}

// normalizeTagsUncapped trims, lowercases, drops empties and de-dupes, with no
// limit on count. Used for manual edits, where the user is in control.
func normalizeTagsUncapped(tags []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(t, "#")))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// normalizeTags is normalizeTagsUncapped capped at 5. Used for AI capture, where
// the model can over-tag.
func normalizeTags(tags []string) []string {
	out := normalizeTagsUncapped(tags)
	if len(out) > 5 {
		out = out[:5]
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
