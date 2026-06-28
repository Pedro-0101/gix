package db

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// FTSHit is a full-text match: a note id and its bm25 score (lower is better).
type FTSHit struct {
	NoteID int64
	Score  float64
}

// AllVectors returns every stored note embedding for brute-force similarity.
func (d *Database) AllVectors() ([]NoteVector, error) {
	rows, err := d.db.Query("SELECT note_id, vec FROM note_vectors")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NoteVector
	for rows.Next() {
		var v NoteVector
		if err := rows.Scan(&v.NoteID, &v.Vec); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ftsMatch turns free-form user text into a safe FTS5 MATCH expression: each
// alphanumeric term is double-quoted (neutralizing FTS operators) and OR-joined
// for recall. Terms shorter than 3 runes are dropped — they're almost all
// Portuguese/English stopwords ("no", "de", "a", "of") that would match
// everything; the semantic vector covers any real short query. Returns "" when
// no usable terms remain.
func ftsMatch(text string) string {
	var terms []string
	for _, f := range strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if utf8.RuneCountInString(f) < 3 {
			continue
		}
		terms = append(terms, `"`+f+`"`)
	}
	return strings.Join(terms, " OR ")
}

// SearchFTS runs a full-text search over free-form user text and returns
// matching note ids ranked by bm25 (ascending, best first). Text with no usable
// terms yields no hits.
func (d *Database) SearchFTS(text string, limit int) ([]FTSHit, error) {
	match := ftsMatch(text)
	if match == "" {
		return nil, nil
	}
	rows, err := d.db.Query(
		"SELECT rowid, bm25(notes_fts) FROM notes_fts WHERE notes_fts MATCH ? ORDER BY bm25(notes_fts) LIMIT ?",
		match, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FTSHit
	for rows.Next() {
		var h FTSHit
		if err := rows.Scan(&h.NoteID, &h.Score); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
