package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

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
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message TEXT NOT NULL,
			note_id INTEGER,
			fire_at DATETIME NOT NULL,
			recurrence TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_due ON alerts(status, fire_at)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return nil, err
		}
	}

	return &Database{db: db}, nil
}
