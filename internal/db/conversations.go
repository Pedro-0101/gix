package db

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
