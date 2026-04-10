package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/chat-back/internal/domain"
)

type FriendRepo struct {
	pool *pgxpool.Pool
}

func NewFriendRepo(pool *pgxpool.Pool) *FriendRepo {
	return &FriendRepo{pool: pool}
}

func (r *FriendRepo) SendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO friendships (requester_id, addressee_id) VALUES ($1, $2)
		 ON CONFLICT (requester_id, addressee_id) DO NOTHING`,
		requesterID, addresseeID,
	)
	return err
}

func (r *FriendRepo) Accept(ctx context.Context, id uuid.UUID, addresseeID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE friendships SET status = 'accepted', updated_at = now()
		 WHERE id = $1 AND addressee_id = $2 AND status = 'pending'`,
		id, addresseeID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *FriendRepo) Remove(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM friendships WHERE id = $1 AND (requester_id = $2 OR addressee_id = $2)`,
		id, userID,
	)
	return err
}

func (r *FriendRepo) ListFriends(ctx context.Context, userID uuid.UUID) ([]*domain.FriendEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.id,
		       u.id, u.username, COALESCE(u.display_name,''), u.email,
		       (u.avatar_data IS NOT NULL), u.created_at, u.updated_at
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.requester_id = $1 THEN f.addressee_id ELSE f.requester_id END
		WHERE (f.requester_id = $1 OR f.addressee_id = $1) AND f.status = 'accepted'
		ORDER BY u.username`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.FriendEntry
	for rows.Next() {
		e := &domain.FriendEntry{}
		if err := rows.Scan(&e.FriendshipID,
			&e.User.ID, &e.User.Username, &e.User.DisplayName, &e.User.Email,
			&e.User.HasAvatar, &e.User.CreatedAt, &e.User.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *FriendRepo) ListPendingReceived(ctx context.Context, userID uuid.UUID) ([]*domain.FriendRequest, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.id,
		       u.id, u.username, COALESCE(u.display_name,''), u.email,
		       (u.avatar_data IS NOT NULL), u.created_at, u.updated_at,
		       f.created_at
		FROM friendships f
		JOIN users u ON u.id = f.requester_id
		WHERE f.addressee_id = $1 AND f.status = 'pending'
		ORDER BY f.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.FriendRequest
	for rows.Next() {
		req := &domain.FriendRequest{}
		var createdAt time.Time
		if err := rows.Scan(&req.FriendshipID,
			&req.From.ID, &req.From.Username, &req.From.DisplayName, &req.From.Email,
			&req.From.HasAvatar, &req.From.CreatedAt, &req.From.UpdatedAt,
			&createdAt); err != nil {
			return nil, err
		}
		req.CreatedAt = createdAt
		out = append(out, req)
	}
	return out, rows.Err()
}

func (r *FriendRepo) GetOrCreateDM(ctx context.Context, userID1, userID2 uuid.UUID) (*domain.Room, error) {
	room := &domain.Room{}
	err := r.pool.QueryRow(ctx, `
		SELECT r.id, r.name, r.type, r.owner_id, r.created_at FROM rooms r
		WHERE r.type = 'dm'
		  AND EXISTS (SELECT 1 FROM room_members WHERE room_id = r.id AND user_id = $1)
		  AND EXISTS (SELECT 1 FROM room_members WHERE room_id = r.id AND user_id = $2)
		LIMIT 1`, userID1, userID2,
	).Scan(&room.ID, &room.Name, &room.Type, &room.OwnerID, &room.CreatedAt)
	if err == nil {
		return room, nil
	}
	// Create new DM room
	room.ID = uuid.New()
	room.Name = ""
	room.Type = domain.RoomTypeDM
	room.OwnerID = userID1
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO rooms (id, name, type, owner_id) VALUES ($1, $2, $3, $4)`,
		room.ID, room.Name, room.Type, room.OwnerID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO room_members (room_id, user_id) VALUES ($1, $2), ($1, $3)`,
		room.ID, userID1, userID2); err != nil {
		return nil, err
	}
	return room, tx.Commit(ctx)
}

func (r *FriendRepo) ListDMs(ctx context.Context, userID uuid.UUID) ([]*domain.DMRoom, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, r.created_at,
		       u.id, u.username, COALESCE(u.display_name,''), (u.avatar_data IS NOT NULL)
		FROM rooms r
		JOIN room_members rm ON rm.room_id = r.id AND rm.user_id != $1
		JOIN users u ON u.id = rm.user_id
		WHERE r.type = 'dm'
		  AND EXISTS (SELECT 1 FROM room_members WHERE room_id = r.id AND user_id = $1)
		ORDER BY r.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.DMRoom
	for rows.Next() {
		dm := &domain.DMRoom{}
		if err := rows.Scan(&dm.ID, &dm.CreatedAt,
			&dm.OtherUser.ID, &dm.OtherUser.Username,
			&dm.OtherUser.DisplayName, &dm.OtherUser.HasAvatar); err != nil {
			return nil, err
		}
		out = append(out, dm)
	}
	return out, rows.Err()
}
