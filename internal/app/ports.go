package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/domain"
)

// RoomRepository is the port for room persistence.
type RoomRepository interface {
	Create(ctx context.Context, room *domain.Room) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Room, error)
	List(ctx context.Context) ([]*domain.Room, error)
	Delete(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error
	AddMember(ctx context.Context, roomID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, roomID, userID uuid.UUID) error
	GetMembers(ctx context.Context, roomID uuid.UUID) ([]domain.Member, error)
	MemberCount(ctx context.Context, roomID uuid.UUID) (int, error)
}

// MessageRepository is the port for message persistence.
type MessageRepository interface {
	Save(ctx context.Context, msg *domain.Message) error
	// ListBefore returns up to `limit` messages before `cursor` (UUID, time-sorted).
	// Pass uuid.Nil for first page.
	ListBefore(ctx context.Context, roomID uuid.UUID, cursor uuid.UUID, limit int) ([]*domain.Message, error)
}

// PresenceStore is the port for tracking online presence.
type PresenceStore interface {
	SetOnline(ctx context.Context, userID uuid.UUID) error
	SetOffline(ctx context.Context, userID uuid.UUID) error
	Heartbeat(ctx context.Context, userID uuid.UUID) error
	IsOnline(ctx context.Context, userID uuid.UUID) (bool, error)
}
