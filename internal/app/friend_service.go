package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/domain"
)

type FriendService struct {
	repo FriendRepository
}

func NewFriendService(repo FriendRepository) *FriendService {
	return &FriendService{repo: repo}
}

func (s *FriendService) SendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) error {
	if requesterID == addresseeID {
		return domain.ErrBadRequest
	}
	return s.repo.SendRequest(ctx, requesterID, addresseeID)
}

func (s *FriendService) Accept(ctx context.Context, id uuid.UUID, addresseeID uuid.UUID) error {
	return s.repo.Accept(ctx, id, addresseeID)
}

func (s *FriendService) Remove(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return s.repo.Remove(ctx, id, userID)
}

func (s *FriendService) ListFriends(ctx context.Context, userID uuid.UUID) ([]*domain.FriendEntry, error) {
	return s.repo.ListFriends(ctx, userID)
}

func (s *FriendService) ListPendingReceived(ctx context.Context, userID uuid.UUID) ([]*domain.FriendRequest, error) {
	return s.repo.ListPendingReceived(ctx, userID)
}

func (s *FriendService) GetOrCreateDM(ctx context.Context, userID1, userID2 uuid.UUID) (*domain.Room, error) {
	return s.repo.GetOrCreateDM(ctx, userID1, userID2)
}

func (s *FriendService) ListDMs(ctx context.Context, userID uuid.UUID) ([]*domain.DMRoom, error) {
	return s.repo.ListDMs(ctx, userID)
}
