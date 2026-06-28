// Package db is the SQLite persistence layer for gix: notes (with tags,
// embeddings and a full-text index), alerts and chat history. Every domain
// shares one *sql.DB behind the Database type and lives in its own file
// (notes.go, search.go, alerts.go, conversations.go); the schema is in
// schema.go.
package db

import (
	"database/sql"
	"os"
	"path/filepath"
)

// Database wraps the single SQLite connection shared by every domain.
type Database struct {
	db *sql.DB
}

// New opens the database at the default per-user location (UserConfigDir/gix).
func New() (*Database, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return Open(filepath.Join(dir, "gix", "notes.db"))
}

// Close closes the underlying connection.
func (d *Database) Close() error {
	return d.db.Close()
}
