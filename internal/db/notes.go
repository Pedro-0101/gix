package db

import "strings"

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
