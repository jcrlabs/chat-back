package app

import (
	"context"

	"github.com/google/uuid"
)

type PresenceService struct {
	store PresenceStore
}

func NewPresenceService(store PresenceStore) *PresenceService {
	return &PresenceService{store: store}
}

func (s *PresenceService) SetOnline(ctx context.Context, userID uuid.UUID) error {
	return s.store.SetOnline(ctx, userID)
}

func (s *PresenceService) SetOffline(ctx context.Context, userID uuid.UUID) error {
	return s.store.SetOffline(ctx, userID)
}

func (s *PresenceService) Heartbeat(ctx context.Context, userID uuid.UUID) error {
	return s.store.Heartbeat(ctx, userID)
}

func (s *PresenceService) IsOnline(ctx context.Context, userID uuid.UUID) (bool, error) {
	return s.store.IsOnline(ctx, userID)
}
