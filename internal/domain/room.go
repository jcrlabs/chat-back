package domain

import (
	"time"

	"github.com/google/uuid"
)

type RoomType string

const (
	RoomTypePublic  RoomType = "public"
	RoomTypePrivate RoomType = "private"
	RoomTypeDM      RoomType = "dm"
)

const MaxRoomMembers = 100

type Room struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Type      RoomType  `json:"type"`
	OwnerID   uuid.UUID `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Member struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
}
