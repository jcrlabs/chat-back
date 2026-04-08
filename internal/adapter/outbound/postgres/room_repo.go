package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/chat-back/internal/domain"
)

type RoomRepo struct {
	pool *pgxpool.Pool
}

func NewRoomRepo(pool *pgxpool.Pool) *RoomRepo {
	return &RoomRepo{pool: pool}
}

func (r *RoomRepo) Create(ctx context.Context, room *domain.Room) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO rooms (id, name, type, owner_id, created_at) VALUES ($1, $2, $3, $4, $5)`,
		room.ID, room.Name, room.Type, room.OwnerID, room.CreatedAt,
	)
	return err
}

func (r *RoomRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Room, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, type, owner_id, created_at FROM rooms WHERE id = $1`, id,
	)
	room := &domain.Room{}
	if err := row.Scan(&room.ID, &room.Name, &room.Type, &room.OwnerID, &room.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return room, nil
}

func (r *RoomRepo) List(ctx context.Context) ([]*domain.Room, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, type, owner_id, created_at FROM rooms WHERE type = 'public' ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*domain.Room
	for rows.Next() {
		room := &domain.Room{}
		if err := rows.Scan(&room.ID, &room.Name, &room.Type, &room.OwnerID, &room.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (r *RoomRepo) Delete(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM rooms WHERE id = $1 AND owner_id = $2`, id, ownerID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrForbidden
	}
	return nil
}

func (r *RoomRepo) AddMember(ctx context.Context, roomID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO room_members (room_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		roomID, userID,
	)
	return err
}

func (r *RoomRepo) RemoveMember(ctx context.Context, roomID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM room_members WHERE room_id = $1 AND user_id = $2`, roomID, userID,
	)
	return err
}

func (r *RoomRepo) GetMembers(ctx context.Context, roomID uuid.UUID) ([]domain.Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT u.id, u.username FROM users u
		 JOIN room_members rm ON rm.user_id = u.id
		 WHERE rm.room_id = $1`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []domain.Member
	for rows.Next() {
		var m domain.Member
		if err := rows.Scan(&m.UserID, &m.Username); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *RoomRepo) MemberCount(ctx context.Context, roomID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM room_members WHERE room_id = $1`, roomID,
	).Scan(&count)
	return count, err
}
