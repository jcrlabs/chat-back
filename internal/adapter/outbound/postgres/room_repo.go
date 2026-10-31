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

func (r *RoomRepo) DeleteAny(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, id)
	return err
}

func (r *RoomRepo) AddMember(ctx context.Context, roomID, userID uuid.UUID, role domain.MemberRole) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO room_members (room_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (room_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		roomID, userID, role,
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
		`SELECT u.id, u.username, rm.role FROM users u
		 JOIN room_members rm ON rm.user_id = u.id
		 WHERE rm.room_id = $1
		 ORDER BY CASE rm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, u.username`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []domain.Member
	for rows.Next() {
		var m domain.Member
		if err := rows.Scan(&m.UserID, &m.Username, &m.Role); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *RoomRepo) GetMemberRole(ctx context.Context, roomID, userID uuid.UUID) (domain.MemberRole, error) {
	var role domain.MemberRole
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM room_members WHERE room_id = $1 AND user_id = $2`, roomID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return role, err
}

func (r *RoomRepo) SetMemberRole(ctx context.Context, roomID, userID uuid.UUID, role domain.MemberRole) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE room_members SET role = $3 WHERE room_id = $1 AND user_id = $2`, roomID, userID, role,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *RoomRepo) MemberCount(ctx context.Context, roomID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM room_members WHERE room_id = $1`, roomID,
	).Scan(&count)
	return count, err
}

func (r *RoomRepo) IsMember(ctx context.Context, roomID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2)`,
		roomID, userID,
	).Scan(&exists)
	return exists, err
}

func (r *RoomRepo) Rename(ctx context.Context, id uuid.UUID, name string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE rooms SET name = $2 WHERE id = $1`, id, name,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *RoomRepo) ListAll(ctx context.Context) ([]*domain.Room, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, type, owner_id, created_at FROM rooms ORDER BY created_at DESC`,
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

func (r *RoomRepo) GetUnreadCounts(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := r.pool.Query(ctx,
		`WITH user_rooms AS (
		     SELECT r.id AS room_id FROM rooms r
		     WHERE r.type IN ('public', 'voice')
		        OR (r.type IN ('private', 'dm') AND EXISTS (
		                SELECT 1 FROM room_members rm WHERE rm.room_id = r.id AND rm.user_id = $1
		            ))
		 )
		 SELECT m.room_id, COUNT(*) AS cnt
		 FROM messages m
		 JOIN user_rooms ur ON ur.room_id = m.room_id
		 LEFT JOIN room_reads rr ON rr.room_id = m.room_id AND rr.user_id = $1
		 WHERE m.user_id != $1 AND (rr.last_read_at IS NULL OR m.created_at > rr.last_read_at)
		 GROUP BY m.room_id
		 HAVING COUNT(*) > 0`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[uuid.UUID]int)
	for rows.Next() {
		var roomID uuid.UUID
		var cnt int
		if err := rows.Scan(&roomID, &cnt); err != nil {
			return nil, err
		}
		counts[roomID] = cnt
	}
	return counts, rows.Err()
}

func (r *RoomRepo) MarkRoomRead(ctx context.Context, userID, roomID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO room_reads (user_id, room_id, last_read_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (user_id, room_id) DO UPDATE SET last_read_at = now()`,
		userID, roomID,
	)
	return err
}

func (r *RoomRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]*domain.Room, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT r.id, r.name, r.type, r.owner_id, r.created_at
		 FROM rooms r
		 WHERE r.type IN ('public', 'voice')
		    OR (r.type = 'private' AND EXISTS (
		            SELECT 1 FROM room_members rm WHERE rm.room_id = r.id AND rm.user_id = $1
		        ))
		 ORDER BY r.created_at DESC`, userID,
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
