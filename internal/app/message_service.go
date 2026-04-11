package app

import (
	"context"
	"strings"
	"time"

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

func (s *MessageService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MessageService) Edit(ctx context.Context, id, userID uuid.UUID, content string) (*domain.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > domain.MaxMessageLength {
		return nil, domain.ErrBadRequest
	}
	msg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if msg.UserID != userID {
		return nil, domain.ErrForbidden
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateContent(ctx, id, content, now); err != nil {
		return nil, err
	}
	msg.Content = content
	msg.EditedAt = &now
	return msg, nil
}

func (s *MessageService) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
