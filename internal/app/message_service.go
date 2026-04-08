package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/domain"
)

const defaultPageSize = 50

type MessageService struct {
	repo MessageRepository
}

func NewMessageService(repo MessageRepository) *MessageService {
	return &MessageService{repo: repo}
}

func (s *MessageService) Save(ctx context.Context, msg *domain.Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}
	return s.repo.Save(ctx, msg)
}

func (s *MessageService) History(ctx context.Context, roomID uuid.UUID, cursor uuid.UUID, limit int) ([]*domain.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = defaultPageSize
	}
	return s.repo.ListBefore(ctx, roomID, cursor, limit)
}
