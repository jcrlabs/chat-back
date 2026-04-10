package domain

import (
	"time"

	"github.com/google/uuid"
)

type FriendEntry struct {
	FriendshipID uuid.UUID `json:"friendship_id"`
	User         User      `json:"user"`
}

type FriendRequest struct {
	FriendshipID uuid.UUID `json:"friendship_id"`
	From         User      `json:"from"`
	CreatedAt    time.Time `json:"created_at"`
}

type DMRoom struct {
	ID        uuid.UUID `json:"id"`
	OtherUser struct {
		ID          uuid.UUID `json:"id"`
		Username    string    `json:"username"`
		DisplayName string    `json:"display_name,omitempty"`
		HasAvatar   bool      `json:"has_avatar"`
	} `json:"other_user"`
	CreatedAt time.Time `json:"created_at"`
}
