package app_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/domain"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockFriendRepo struct {
	sendRequestFn        func(ctx context.Context, requesterID, addresseeID uuid.UUID) error
	acceptFn             func(ctx context.Context, id uuid.UUID, addresseeID uuid.UUID) error
	removeFn             func(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	listFriendsFn        func(ctx context.Context, userID uuid.UUID) ([]*domain.FriendEntry, error)
	listPendingFn        func(ctx context.Context, userID uuid.UUID) ([]*domain.FriendRequest, error)
	getOrCreateDMFn      func(ctx context.Context, userID1, userID2 uuid.UUID) (*domain.Room, error)
	listDMsFn            func(ctx context.Context, userID uuid.UUID) ([]*domain.DMRoom, error)
}

func (m *mockFriendRepo) SendRequest(ctx context.Context, r, a uuid.UUID) error {
	return m.sendRequestFn(ctx, r, a)
}
func (m *mockFriendRepo) Accept(ctx context.Context, id, addresseeID uuid.UUID) error {
	return m.acceptFn(ctx, id, addresseeID)
}
func (m *mockFriendRepo) Remove(ctx context.Context, id, userID uuid.UUID) error {
	return m.removeFn(ctx, id, userID)
}
func (m *mockFriendRepo) ListFriends(ctx context.Context, userID uuid.UUID) ([]*domain.FriendEntry, error) {
	return m.listFriendsFn(ctx, userID)
}
func (m *mockFriendRepo) ListPendingReceived(ctx context.Context, userID uuid.UUID) ([]*domain.FriendRequest, error) {
	return m.listPendingFn(ctx, userID)
}
func (m *mockFriendRepo) GetOrCreateDM(ctx context.Context, u1, u2 uuid.UUID) (*domain.Room, error) {
	return m.getOrCreateDMFn(ctx, u1, u2)
}
func (m *mockFriendRepo) ListDMs(ctx context.Context, userID uuid.UUID) ([]*domain.DMRoom, error) {
	return m.listDMsFn(ctx, userID)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestFriendService_SendRequest_ToSelf(t *testing.T) {
	svc := app.NewFriendService(&mockFriendRepo{})
	id := uuid.New()
	if err := svc.SendRequest(context.Background(), id, id); err != domain.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestFriendService_SendRequest_OK(t *testing.T) {
	called := false
	repo := &mockFriendRepo{
		sendRequestFn: func(_ context.Context, r, a uuid.UUID) error {
			called = true
			return nil
		},
	}
	svc := app.NewFriendService(repo)
	if err := svc.SendRequest(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("repo.SendRequest was not called")
	}
}

func TestFriendService_Accept_NotFound(t *testing.T) {
	repo := &mockFriendRepo{
		acceptFn: func(_ context.Context, _, _ uuid.UUID) error { return domain.ErrNotFound },
	}
	svc := app.NewFriendService(repo)
	err := svc.Accept(context.Background(), uuid.New(), uuid.New())
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFriendService_ListFriends_Empty(t *testing.T) {
	repo := &mockFriendRepo{
		listFriendsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.FriendEntry, error) {
			return nil, nil
		},
	}
	svc := app.NewFriendService(repo)
	friends, err := svc.ListFriends(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(friends) != 0 {
		t.Fatalf("expected empty list, got %d", len(friends))
	}
}

func TestFriendService_ListFriends_ReturnsFriends(t *testing.T) {
	alice := &domain.FriendEntry{
		FriendshipID: uuid.New(),
		User:         domain.User{Username: "alice", Tag: "0001"},
	}
	repo := &mockFriendRepo{
		listFriendsFn: func(_ context.Context, _ uuid.UUID) ([]*domain.FriendEntry, error) {
			return []*domain.FriendEntry{alice}, nil
		},
	}
	svc := app.NewFriendService(repo)
	friends, err := svc.ListFriends(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(friends) != 1 || friends[0].User.Username != "alice" {
		t.Fatalf("unexpected friends: %+v", friends)
	}
}

func TestFriendService_ListPendingReceived(t *testing.T) {
	req := &domain.FriendRequest{
		FriendshipID: uuid.New(),
		From:         domain.User{Username: "bob", Tag: "0042"},
	}
	repo := &mockFriendRepo{
		listPendingFn: func(_ context.Context, _ uuid.UUID) ([]*domain.FriendRequest, error) {
			return []*domain.FriendRequest{req}, nil
		},
	}
	svc := app.NewFriendService(repo)
	reqs, err := svc.ListPendingReceived(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 || reqs[0].From.Tag != "0042" {
		t.Fatalf("unexpected requests: %+v", reqs)
	}
}

func TestFriendService_GetOrCreateDM(t *testing.T) {
	roomID := uuid.New()
	repo := &mockFriendRepo{
		getOrCreateDMFn: func(_ context.Context, u1, u2 uuid.UUID) (*domain.Room, error) {
			return &domain.Room{ID: roomID, Type: domain.RoomTypeDM}, nil
		},
	}
	svc := app.NewFriendService(repo)
	room, err := svc.GetOrCreateDM(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if room.ID != roomID {
		t.Fatalf("expected room %v, got %v", roomID, room.ID)
	}
	if room.Type != domain.RoomTypeDM {
		t.Fatalf("expected DM room type, got %v", room.Type)
	}
}

func TestFriendService_Remove(t *testing.T) {
	called := false
	targetID := uuid.New()
	callerID := uuid.New()
	repo := &mockFriendRepo{
		removeFn: func(_ context.Context, id, userID uuid.UUID) error {
			called = true
			if id != targetID || userID != callerID {
				t.Errorf("unexpected args: id=%v userID=%v", id, userID)
			}
			return nil
		},
	}
	svc := app.NewFriendService(repo)
	if err := svc.Remove(context.Background(), targetID, callerID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("repo.Remove was not called")
	}
}
