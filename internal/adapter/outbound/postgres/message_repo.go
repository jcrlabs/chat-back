package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/chat-back/internal/domain"
)

type MessageRepo struct {
	pool *pgxpool.Pool
}

func NewMessageRepo(pool *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{pool: pool}
}

func (r *MessageRepo) Save(ctx context.Context, msg *domain.Message) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO messages (id, room_id, user_id, username, content, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		msg.ID, msg.RoomID, msg.UserID, msg.Username, msg.Content, msg.CreatedAt,
	)
	return err
}

// ListBefore returns messages before cursor (exclusive), newest first, up to limit.
// cursor = uuid.Nil → return latest messages.
func (r *MessageRepo) ListBefore(ctx context.Context, roomID uuid.UUID, cursor uuid.UUID, limit int) ([]*domain.Message, error) {
	var (
		query string
		args  []any
	)

	const sel = `
		SELECT m.id, m.room_id, m.user_id, m.username,
		       COALESCE(u.display_name, ''),
		       CASE WHEN u.avatar_data IS NOT NULL
		            THEN '/api/users/' || m.user_id::text || '/avatar'
		            ELSE '' END,
		       m.content, m.created_at
		FROM messages m
		LEFT JOIN users u ON u.id = m.user_id`

	if cursor == uuid.Nil {
		query = sel + ` WHERE m.room_id = $1 ORDER BY m.id DESC LIMIT $2`
		args = []any{roomID, limit}
	} else {
		query = sel + ` WHERE m.room_id = $1 AND m.id < $2 ORDER BY m.id DESC LIMIT $3`
		args = []any{roomID, cursor, limit}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	msgs := make([]*domain.Message, 0)
	for rows.Next() {
		m := &domain.Message{}
		if err := rows.Scan(&m.ID, &m.RoomID, &m.UserID, &m.Username,
			&m.DisplayName, &m.AvatarURL, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
