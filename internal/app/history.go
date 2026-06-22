package app

import "gix/internal/db"

type HistoryService struct {
	db *db.Database
}

func NewHistoryService(database *db.Database) *HistoryService {
	return &HistoryService{db: database}
}

func (s *HistoryService) List() ([]db.Conversation, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.db.ListConversations()
}

func (s *HistoryService) Messages(id int64) ([]db.Message, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.db.GetMessages(id)
}

func (s *HistoryService) Delete(id int64) error {
	if s.db == nil {
		return nil
	}
	return s.db.DeleteConversation(id)
}
