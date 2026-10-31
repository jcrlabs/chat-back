package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/chat-back/internal/domain"
)

type InviteRepo struct {
	pool *pgxpool.Pool
}

func NewInviteRepo(pool *pgxpool.Pool) *InviteRepo {
	return &InviteRepo{pool: pool}
}

func (r *InviteRepo) Create(ctx context.Context, invite *domain.RoomInvite) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO room_invites (id, room_id, inviter_id, invitee_id, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		invite.ID, invite.RoomID, invite.InviterID, invite.InviteeID, invite.Status, invite.CreatedAt,
	)
	return err
}

func (r *InviteRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.RoomInvite, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT ri.id, ri.room_id, ri.inviter_id, ri.invitee_id, ri.status, ri.created_at,
		        rm.name, u1.username, u2.username
		 FROM room_invites ri
		 JOIN rooms rm ON rm.id = ri.room_id
		 JOIN users u1 ON u1.id = ri.inviter_id
		 JOIN users u2 ON u2.id = ri.invitee_id
		 WHERE ri.id = $1`, id,
	)
	inv := &domain.RoomInvite{}
	if err := row.Scan(&inv.ID, &inv.RoomID, &inv.InviterID, &inv.InviteeID, &inv.Status, &inv.CreatedAt,
		&inv.RoomName, &inv.InviterUsername, &inv.InviteeUsername); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

func (r *InviteRepo) Accept(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE room_invites SET status = 'accepted' WHERE id = $1 AND status = 'pending'`, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *InviteRepo) Decline(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE room_invites SET status = 'declined' WHERE id = $1 AND status = 'pending'`, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *InviteRepo) ListPending(ctx context.Context, userID uuid.UUID) ([]*domain.RoomInvite, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT ri.id, ri.room_id, ri.inviter_id, ri.invitee_id, ri.status, ri.created_at,
		        rm.name, u.username, ''
		 FROM room_invites ri
		 JOIN rooms rm ON rm.id = ri.room_id
		 JOIN users u ON u.id = ri.inviter_id
		 WHERE ri.invitee_id = $1 AND ri.status = 'pending'
		 ORDER BY ri.created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []*domain.RoomInvite
	for rows.Next() {
		inv := &domain.RoomInvite{}
		if err := rows.Scan(&inv.ID, &inv.RoomID, &inv.InviterID, &inv.InviteeID, &inv.Status, &inv.CreatedAt,
			&inv.RoomName, &inv.InviterUsername, &inv.InviteeUsername); err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}

func (r *InviteRepo) ListForRoom(ctx context.Context, roomID uuid.UUID) ([]*domain.RoomInvite, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT ri.id, ri.room_id, ri.inviter_id, ri.invitee_id, ri.status, ri.created_at,
		        '', '', u.username
		 FROM room_invites ri
		 JOIN users u ON u.id = ri.invitee_id
		 WHERE ri.room_id = $1
		 ORDER BY ri.created_at DESC`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []*domain.RoomInvite
	for rows.Next() {
		inv := &domain.RoomInvite{}
		if err := rows.Scan(&inv.ID, &inv.RoomID, &inv.InviterID, &inv.InviteeID, &inv.Status, &inv.CreatedAt,
			&inv.RoomName, &inv.InviterUsername, &inv.InviteeUsername); err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}
