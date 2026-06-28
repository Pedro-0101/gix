package app

import (
	"sort"
	"strings"

	"gix/internal/db"
	"gix/internal/embed"
)

// Tunables for hybrid search.
const (
	candidateLimit = 30 // per-source (FTS / vector) candidates before fusion
	rrfK           = 60 // Reciprocal Rank Fusion constant
	snippetRunes   = 180
)

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

// snippet returns a short single-line preview of a note's content.
func snippet(content string) string {
	flat := strings.Join(strings.Fields(strings.ReplaceAll(content, "\n", " ")), " ")
	r := []rune(flat)
	if len(r) > snippetRunes {
		return strings.TrimSpace(string(r[:snippetRunes])) + "…"
	}
	return flat
}
