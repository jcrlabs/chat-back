package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/domain"
)

type RoomService struct {
	repo RoomRepository
}

func NewRoomService(repo RoomRepository) *RoomService {
	return &RoomService{repo: repo}
}

func (s *RoomService) Create(ctx context.Context, name string, roomType domain.RoomType, ownerID uuid.UUID) (*domain.Room, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, domain.ErrBadRequest
	}
	room := &domain.Room{
		ID:      uuid.New(),
		Name:    name,
		Type:    roomType,
		OwnerID: ownerID,
	}
	if err := s.repo.Create(ctx, room); err != nil {
		return nil, err
	}
	return room, nil
}

func (s *RoomService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Room, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RoomService) List(ctx context.Context) ([]*domain.Room, error) {
	return s.repo.List(ctx)
}

func (s *RoomService) Delete(ctx context.Context, id, requesterID uuid.UUID) error {
	return s.repo.Delete(ctx, id, requesterID)
}

func (s *RoomService) GetMembers(ctx context.Context, roomID uuid.UUID) ([]domain.Member, error) {
	return s.repo.GetMembers(ctx, roomID)
}
