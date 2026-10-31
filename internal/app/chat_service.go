package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/domain"
)

// MessageBroadcaster is the port for pub/sub across pods.
type MessageBroadcaster interface {
	Publish(ctx context.Context, roomID uuid.UUID, msg []byte) error
}

type ChatService struct {
	messages    *MessageService
	broadcaster MessageBroadcaster
}

func NewChatService(messages *MessageService, broadcaster MessageBroadcaster) *ChatService {
	return &ChatService{messages: messages, broadcaster: broadcaster}
}

// SendMessage persists the message and returns it. The hub publishes to Redis.
func (s *ChatService) SendMessage(ctx context.Context, userID uuid.UUID, username string, roomID uuid.UUID, content string) (*domain.Message, error) {
	msg := &domain.Message{
		ID:        uuid.New(),
		RoomID:    roomID,
		UserID:    userID,
		Username:  username,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.messages.Save(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
