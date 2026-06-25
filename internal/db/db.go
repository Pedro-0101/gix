package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

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

// Note é uma anotação do usuário. LineLimit == 0 e IntegrationMode == ""
// significam "usar o default global" (resolvido no serviço, não aqui).
type Note struct {
	ID              int64
	Title           string
	Content         string
	LineLimit       int
	IntegrationMode string
	CreatedAt       string
	UpdatedAt       string
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

// Open abre (ou cria) o banco no caminho dado e garante o schema.
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
			line_limit INTEGER NOT NULL DEFAULT 0,
			integration_mode TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME
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

	// Migração de bancos legados: a tabela notes pode existir sem as colunas
	// novas. ALTER TABLE ... ADD COLUMN num banco já migrado falha com
	// "duplicate column name" — ignoramos esse caso para manter idempotência.
	migrations := []string{
		`ALTER TABLE notes ADD COLUMN line_limit INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE notes ADD COLUMN integration_mode TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE notes ADD COLUMN updated_at DATETIME`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return nil, err
		}
	}

	return &Database{db: db}, nil
}

// ExtractTitle gera um título a partir da primeira linha do conteúdo,
// truncando em 40 runas.
func ExtractTitle(content string) string {
	content = strings.TrimSpace(content)
	if i := strings.IndexByte(content, '\n'); i != -1 {
		content = content[:i]
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "Conversa"
	}
	r := []rune(content)
	const max = 40
	if len(r) > max {
		return strings.TrimSpace(string(r[:max])) + "…"
	}
	return content
}

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

func (d *Database) CreateNote(title, content string, lineLimit int, mode string) (int64, error) {
	res, err := d.db.Exec(
		"INSERT INTO notes (title, content, line_limit, integration_mode) VALUES (?, ?, ?, ?)",
		title, content, lineLimit, mode)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Database) GetNote(id int64) (Note, error) {
	var n Note
	var updated sql.NullString
	err := d.db.QueryRow(
		"SELECT id, title, content, line_limit, integration_mode, created_at, updated_at FROM notes WHERE id = ?", id).
		Scan(&n.ID, &n.Title, &n.Content, &n.LineLimit, &n.IntegrationMode, &n.CreatedAt, &updated)
	if err != nil {
		return Note{}, err
	}
	n.UpdatedAt = updated.String
	return n, nil
}

func (d *Database) ListNotes() ([]Note, error) {
	rows, err := d.db.Query(
		"SELECT id, title, content, line_limit, integration_mode, created_at, updated_at FROM notes ORDER BY created_at DESC, id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Note
	for rows.Next() {
		var n Note
		var updated sql.NullString
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.LineLimit, &n.IntegrationMode, &n.CreatedAt, &updated); err != nil {
			return nil, err
		}
		n.UpdatedAt = updated.String
		out = append(out, n)
	}
	return out, rows.Err()
}

func (d *Database) UpdateNoteContent(id int64, content string) error {
	_, err := d.db.Exec(
		"UPDATE notes SET content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", content, id)
	return err
}

func (d *Database) DeleteNote(id int64) error {
	_, err := d.db.Exec("DELETE FROM notes WHERE id = ?", id)
	return err
}

func (d *Database) DeleteConversation(id int64) error {
	// SQLite não força foreign keys por padrão e não habilitamos o PRAGMA,
	// então a remoção em cascata é manual: apagar as mensagens antes da
	// conversa. Manter esta ordem.
	if _, err := d.db.Exec("DELETE FROM messages WHERE conversation_id = ?", id); err != nil {
		return err
	}
	_, err := d.db.Exec("DELETE FROM conversations WHERE id = ?", id)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}

