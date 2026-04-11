package app

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/domain"
)

type RoomService struct {
	repo       RoomRepository
	inviteRepo RoomInviteRepository
}

func NewRoomService(repo RoomRepository, inviteRepo RoomInviteRepository) *RoomService {
	return &RoomService{repo: repo, inviteRepo: inviteRepo}
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
	// Add owner as member with owner role
	_ = s.repo.AddMember(ctx, room.ID, ownerID, domain.RoleOwner)
	return room, nil
}

func (s *RoomService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Room, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RoomService) List(ctx context.Context, userID uuid.UUID) ([]*domain.Room, error) {
	return s.repo.ListForUser(ctx, userID)
}

func (s *RoomService) Delete(ctx context.Context, id, requesterID uuid.UUID) error {
	return s.repo.Delete(ctx, id, requesterID)
}

func (s *RoomService) GetMembers(ctx context.Context, roomID uuid.UUID) ([]domain.Member, error) {
	return s.repo.GetMembers(ctx, roomID)
}

func (s *RoomService) InviteUser(ctx context.Context, roomID, inviterID, inviteeID uuid.UUID) (*domain.RoomInvite, error) {
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room.Type != domain.RoomTypePrivate {
		return nil, domain.ErrBadRequest
	}
	if room.OwnerID != inviterID {
		isMember, err := s.repo.IsMember(ctx, roomID, inviterID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, domain.ErrForbidden
		}
	}
	invite := &domain.RoomInvite{
		ID:        uuid.New(),
		RoomID:    roomID,
		InviterID: inviterID,
		InviteeID: inviteeID,
		Status:    domain.InviteStatusPending,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.inviteRepo.Create(ctx, invite); err != nil {
		return nil, err
	}
	return invite, nil
}

func (s *RoomService) AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	invite, err := s.inviteRepo.GetByID(ctx, inviteID)
	if err != nil {
		return err
	}
	if invite.InviteeID != userID {
		return domain.ErrForbidden
	}
	if err := s.inviteRepo.Accept(ctx, inviteID); err != nil {
		return err
	}
	return s.repo.AddMember(ctx, invite.RoomID, userID, domain.RoleMember)
}

func (s *RoomService) DeclineInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	invite, err := s.inviteRepo.GetByID(ctx, inviteID)
	if err != nil {
		return err
	}
	if invite.InviteeID != userID {
		return domain.ErrForbidden
	}
	return s.inviteRepo.Decline(ctx, inviteID)
}

func (s *RoomService) ListMyInvites(ctx context.Context, userID uuid.UUID) ([]*domain.RoomInvite, error) {
	return s.inviteRepo.ListPending(ctx, userID)
}

func (s *RoomService) ListRoomInvites(ctx context.Context, roomID, requesterID uuid.UUID) ([]*domain.RoomInvite, error) {
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room.OwnerID != requesterID {
		return nil, domain.ErrForbidden
	}
	return s.inviteRepo.ListForRoom(ctx, roomID)
}

func (s *RoomService) IsMember(ctx context.Context, roomID, userID uuid.UUID) (bool, error) {
	return s.repo.IsMember(ctx, roomID, userID)
}

func (s *RoomService) GetMemberRole(ctx context.Context, roomID, userID uuid.UUID) (domain.MemberRole, error) {
	return s.repo.GetMemberRole(ctx, roomID, userID)
}

func (s *RoomService) SetRole(ctx context.Context, roomID, requesterID, targetID uuid.UUID, role domain.MemberRole) error {
	if role == domain.RoleOwner {
		return domain.ErrForbidden // can't promote to owner
	}
	requesterRole, err := s.repo.GetMemberRole(ctx, roomID, requesterID)
	if err != nil {
		return domain.ErrForbidden
	}
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		return err
	}
	// only owner can set roles
	if requesterRole != domain.RoleOwner && room.OwnerID != requesterID {
		return domain.ErrForbidden
	}
	return s.repo.SetMemberRole(ctx, roomID, targetID, role)
}

func (s *RoomService) KickMember(ctx context.Context, roomID, requesterID, targetID uuid.UUID) error {
	if requesterID == targetID {
		return domain.ErrBadRequest
	}
	requesterRole, err := s.repo.GetMemberRole(ctx, roomID, requesterID)
	if err != nil {
		return domain.ErrForbidden
	}
	targetRole, err := s.repo.GetMemberRole(ctx, roomID, targetID)
	if err != nil {
		return domain.ErrNotFound
	}
	// owner can kick anyone; admin can kick members only
	switch requesterRole {
	case domain.RoleOwner:
		// ok
	case domain.RoleAdmin:
		if targetRole != domain.RoleMember {
			return domain.ErrForbidden
		}
	default:
		return domain.ErrForbidden
	}
	return s.repo.RemoveMember(ctx, roomID, targetID)
}

func (s *RoomService) GetRoom(ctx context.Context, id uuid.UUID) (*domain.Room, error) {
	return s.repo.GetByID(ctx, id)
}
