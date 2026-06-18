package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type Note struct {
	ID      int64
	Title   string
	Content string
}

type Database struct {
	db *sql.DB
}

func New() (*Database, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "gix", "notes.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func ExtractTitle(content string) string {
	content = strings.TrimSpace(content)
	idx := strings.Index(content, " ")
	if idx == -1 {
		return content
	}
	return content[:idx]
}

func (d *Database) Create(title, content string) (int64, error) {
	res, err := d.db.Exec("INSERT INTO notes (title, content) VALUES (?, ?)", title, content)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Database) List() ([]Note, error) {
	rows, err := d.db.Query("SELECT id, title, content FROM notes ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, nil
}

func (d *Database) Delete(id int64) error {
	_, err := d.db.Exec("DELETE FROM notes WHERE id = ?", id)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}
