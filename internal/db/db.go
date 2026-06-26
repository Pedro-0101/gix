package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

type Conversation struct {
	ID        int64
	Title     string
	Model     string
	CreatedAt string
}

type Message struct {
	ID      int64
	Role    string
	Content string
}

// Note is one atomic captured note. Title/Content are AI-formatted at capture;
// Tags are AI-extracted. The semantic vector lives in note_vectors and the
// searchable text in the notes_fts virtual table (kept in sync by CreateNote).
type Note struct {
	ID        int64
	Title     string
	Content   string
	Tags      []string
	CreatedAt string
}

// NoteVector is a stored embedding: the raw little-endian float32 blob plus its
// note id. Callers decode via embed.DecodeVector.
type NoteVector struct {
	NoteID int64
	Vec    []byte
}

// FTSHit is a full-text match: a note id and its bm25 score (lower is better).
type FTSHit struct {
	NoteID int64
	Score  float64
}

type Database struct {
	db *sql.DB
}

func New() (*Database, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return Open(filepath.Join(dir, "gix", "notes.db"))
}

// Open opens (or creates) the database at path and ensures the schema.
func Open(path string) (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS note_tags (
			note_id INTEGER NOT NULL,
			tag TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_note_tags_tag ON note_tags(tag)`,
		`CREATE INDEX IF NOT EXISTS idx_note_tags_note ON note_tags(note_id)`,
		`CREATE TABLE IF NOT EXISTS note_vectors (
			note_id INTEGER PRIMARY KEY,
			dim INTEGER NOT NULL,
			vec BLOB NOT NULL
		)`,
		// Regular (not contentless) FTS5 so snippet()/highlight work; rowid = note id.
		// remove_diacritics 2 makes "ruido" match "ruído" — important for Portuguese.
		`CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
			title, content, tags,
			tokenize = 'unicode61 remove_diacritics 2'
		)`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			model TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return nil, err
		}
	}

	return &Database{db: db}, nil
}

// --- notes ---

// CreateNote inserts an atomic note with its tags and embedding, and indexes it
// for full-text search, all in one transaction. vec is the embed-serialized
// vector blob; dim its length in float32s. Returns the new note id.
func (d *Database) CreateNote(title, content string, tags []string, vec []byte, dim int) (int64, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec("INSERT INTO notes (title, content) VALUES (?, ?)", title, content)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, tag := range tags {
		if _, err := tx.Exec("INSERT INTO note_tags (note_id, tag) VALUES (?, ?)", id, tag); err != nil {
			return 0, err
		}
	}

	if len(vec) > 0 {
		if _, err := tx.Exec("INSERT INTO note_vectors (note_id, dim, vec) VALUES (?, ?, ?)", id, dim, vec); err != nil {
			return 0, err
		}
	}

	if _, err := tx.Exec(
		"INSERT INTO notes_fts (rowid, title, content, tags) VALUES (?, ?, ?, ?)",
		id, title, content, strings.Join(tags, " ")); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateNote replaces a note's title, content, tags and embedding in one
// transaction, keeping note_tags, note_vectors and the FTS index in sync. The
// note's id and created_at are preserved. vec is the embed-serialized vector
// blob; dim its length in float32s. An empty vec leaves the note without one.
func (d *Database) UpdateNote(id int64, title, content string, tags []string, vec []byte, dim int) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE notes SET title = ?, content = ? WHERE id = ?", title, content, id); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM note_tags WHERE note_id = ?", id); err != nil {
		return err
	}
	for _, tag := range tags {
		if _, err := tx.Exec("INSERT INTO note_tags (note_id, tag) VALUES (?, ?)", id, tag); err != nil {
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM note_vectors WHERE note_id = ?", id); err != nil {
		return err
	}
	if len(vec) > 0 {
		if _, err := tx.Exec("INSERT INTO note_vectors (note_id, dim, vec) VALUES (?, ?, ?)", id, dim, vec); err != nil {
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM notes_fts WHERE rowid = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec(
		"INSERT INTO notes_fts (rowid, title, content, tags) VALUES (?, ?, ?, ?)",
		id, title, content, strings.Join(tags, " ")); err != nil {
		return err
	}

	return tx.Commit()
}

// GetNote returns one note with its tags.
func (d *Database) GetNote(id int64) (Note, error) {
	var n Note
	err := d.db.QueryRow(
		"SELECT id, title, content, created_at FROM notes WHERE id = ?", id).
		Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt)
	if err != nil {
		return Note{}, err
	}
	n.Tags, err = d.tagsFor(id)
	if err != nil {
		return Note{}, err
	}
	return n, nil
}

// ListNotes returns every note, newest first, each with its tags.
func (d *Database) ListNotes() ([]Note, error) {
	rows, err := d.db.Query(
		"SELECT id, title, content, created_at FROM notes ORDER BY created_at DESC, id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return d.attachTags(out)
}

// NotesByIDs returns the notes for the given ids (with tags), in arbitrary order.
func (d *Database) NotesByIDs(ids []int64) ([]Note, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := d.db.Query(
		"SELECT id, title, content, created_at FROM notes WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return d.attachTags(out)
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

// DeleteNote removes a note and all of its derived rows.
func (d *Database) DeleteNote(id int64) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		"DELETE FROM notes WHERE id = ?",
		"DELETE FROM note_tags WHERE note_id = ?",
		"DELETE FROM note_vectors WHERE note_id = ?",
		"DELETE FROM notes_fts WHERE rowid = ?",
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *Database) tagsFor(noteID int64) ([]string, error) {
	rows, err := d.db.Query("SELECT tag FROM note_tags WHERE note_id = ? ORDER BY rowid", noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// attachTags fills the Tags field of each note with one extra query.
func (d *Database) attachTags(notes []Note) ([]Note, error) {
	for i := range notes {
		tags, err := d.tagsFor(notes[i].ID)
		if err != nil {
			return nil, err
		}
		notes[i].Tags = tags
	}
	return notes, nil
}

// --- conversations / messages (chat history) ---

func (d *Database) CreateConversation(title, model string) (int64, error) {
	res, err := d.db.Exec("INSERT INTO conversations (title, model) VALUES (?, ?)", title, model)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Database) AddMessage(convID int64, role, content string) error {
	_, err := d.db.Exec(
		"INSERT INTO messages (conversation_id, role, content) VALUES (?, ?, ?)",
		convID, role, content)
	return err
}

func (d *Database) ListConversations() ([]Conversation, error) {
	rows, err := d.db.Query(
		"SELECT id, title, model, created_at FROM conversations ORDER BY created_at DESC, id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.Model, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *Database) GetMessages(convID int64) ([]Message, error) {
	rows, err := d.db.Query(
		"SELECT id, role, content FROM messages WHERE conversation_id = ? ORDER BY id ASC", convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Role, &m.Content); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *Database) DeleteConversation(id int64) error {
	// SQLite foreign keys aren't enforced here, so delete messages first.
	if _, err := d.db.Exec("DELETE FROM messages WHERE conversation_id = ?", id); err != nil {
		return err
	}
	_, err := d.db.Exec("DELETE FROM conversations WHERE id = ?", id)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}

// ExtractTitle derives a title from the first line of content, stripped of
// leading Markdown markers and truncated to 40 runes. Used as a fallback when
// the AI doesn't supply a title.
func ExtractTitle(content string) string {
	content = strings.TrimSpace(content)
	if i := strings.IndexByte(content, '\n'); i != -1 {
		content = content[:i]
	}
	content = stripMarkdownMarkers(content)
	if content == "" {
		return "Nota"
	}
	r := []rune(content)
	const max = 40
	if len(r) > max {
		return strings.TrimSpace(string(r[:max])) + "…"
	}
	return content
}

// stripMarkdownMarkers removes common leading Markdown markers (heading, bullet,
// task, quote) from a line, applied repeatedly for cases like "- [ ] task".
func stripMarkdownMarkers(s string) string {
	for {
		t := strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(t, "#"):
			t = strings.TrimLeft(t, "#")
		case strings.HasPrefix(t, ">"):
			t = strings.TrimLeft(t, ">")
		case strings.HasPrefix(t, "- [ ] "), strings.HasPrefix(t, "- [x] "), strings.HasPrefix(t, "- [X] "):
			t = t[6:]
		case strings.HasPrefix(t, "- "), strings.HasPrefix(t, "* "), strings.HasPrefix(t, "+ "):
			t = t[2:]
		default:
			return strings.TrimSpace(t)
		}
		s = t
	}
}
