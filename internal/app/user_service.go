package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/domain"
)

type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) UpdateProfile(ctx context.Context, id uuid.UUID, displayName string) error {
	return s.repo.UpdateProfile(ctx, id, displayName)
}

func (s *UserService) SaveAvatar(ctx context.Context, id uuid.UUID, data []byte, mime string) error {
	return s.repo.SaveAvatar(ctx, id, data, mime)
}

func (s *UserService) GetAvatar(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	return s.repo.GetAvatar(ctx, id)
}

func (s *UserService) Search(ctx context.Context, query string, excludeID uuid.UUID) ([]*domain.User, error) {
	return s.repo.Search(ctx, query, excludeID)
}
