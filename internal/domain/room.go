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

type MemberRole string

const (
	RoleOwner  MemberRole = "owner"
	RoleAdmin  MemberRole = "admin"
	RoleMember MemberRole = "member"
)

type Member struct {
	UserID   uuid.UUID  `json:"user_id"`
	Username string     `json:"username"`
	Role     MemberRole `json:"role"`
}

type InviteStatus string

const (
	InviteStatusPending  InviteStatus = "pending"
	InviteStatusAccepted InviteStatus = "accepted"
	InviteStatusDeclined InviteStatus = "declined"
)

type RoomInvite struct {
	ID        uuid.UUID    `json:"id"`
	RoomID    uuid.UUID    `json:"room_id"`
	InviterID uuid.UUID    `json:"inviter_id"`
	InviteeID uuid.UUID    `json:"invitee_id"`
	Status    InviteStatus `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
	// Joined fields for display
	RoomName        string `json:"room_name,omitempty"`
	InviterUsername string `json:"inviter_username,omitempty"`
	InviteeUsername string `json:"invitee_username,omitempty"`
}
