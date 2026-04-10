package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxMessageLength = 4000

type Message struct {
	ID          uuid.UUID `json:"id"`
	RoomID      uuid.UUID `json:"room_id"`
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}

func (m *Message) Validate() error {
	content := strings.TrimSpace(m.Content)
	if content == "" {
		return ErrBadRequest
	}
	if len(content) > MaxMessageLength {
		return ErrBadRequest
	}
	return nil
}
