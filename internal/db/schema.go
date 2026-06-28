package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

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
			char_limit INTEGER NOT NULL DEFAULT 0,
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

	// Migrations for databases created before a column existed: CREATE TABLE IF
	// NOT EXISTS above won't add columns to an already-present table.
	if err := ensureColumn(db, "notes", "char_limit", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// ensureColumn adds column to table with the given type/default if it isn't
// already there, making the additive migration idempotent. SQLite has no
// "ADD COLUMN IF NOT EXISTS", so a duplicate-column error is treated as success.
func ensureColumn(db *sql.DB, table, column, decl string) error {
	_, err := db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + decl)
	if err != nil && strings.Contains(err.Error(), "duplicate column name") {
		return nil
	}
	return err
}
