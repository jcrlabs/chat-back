package ws

import (
	"time"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/domain"
)

// ClientMessage — messages sent from client to server
type ClientMessage struct {
	Type    string    `json:"type"`                // join_room | leave_room | chat_message | typing
	RoomID  uuid.UUID `json:"room_id"`
	Content string    `json:"content,omitempty"`   // chat_message only
	Typing  *bool     `json:"is_typing,omitempty"` // typing only
}

// ServerMessage — messages sent from server to client
type ServerMessage struct {
	Type      string           `json:"type"`                // chat_message | typing | presence | room_joined | error
	RoomID    uuid.UUID        `json:"room_id,omitempty"`
	UserID    uuid.UUID        `json:"user_id,omitempty"`
	Username  string           `json:"username,omitempty"`
	Content   string           `json:"content,omitempty"`
	Timestamp time.Time        `json:"timestamp,omitempty"`
	Status    string           `json:"status,omitempty"` // online | offline
	Members   []domain.Member  `json:"members,omitempty"`
	Error     *ServerError     `json:"error,omitempty"`
}

type ServerError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	TypeJoinRoom    = "join_room"
	TypeLeaveRoom   = "leave_room"
	TypeChatMessage = "chat_message"
	TypeTyping      = "typing"
	TypePresence    = "presence"
	TypeRoomJoined  = "room_joined"
	TypeError       = "error"
)
