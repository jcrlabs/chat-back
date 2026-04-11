package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/chat-back/internal/domain"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u := &domain.User{}
	var displayName *string
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, COALESCE(display_name,''), email,
		        (avatar_data IS NOT NULL) AS has_avatar, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.HasAvatar, &u.CreatedAt, &u.UpdatedAt)
	_ = displayName
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return u, err
}

func (r *UserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, displayName string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET display_name = $1, updated_at = now() WHERE id = $2`,
		displayName, id,
	)
	return err
}

func (r *UserRepo) SaveAvatar(ctx context.Context, id uuid.UUID, data []byte, mime string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET avatar_data = $1, avatar_mime = $2, updated_at = now() WHERE id = $3`,
		data, mime, id,
	)
	return err
}

func (r *UserRepo) GetAvatar(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	var data []byte
	var mime string
	err := r.pool.QueryRow(ctx,
		`SELECT avatar_data, COALESCE(avatar_mime,'image/jpeg') FROM users WHERE id = $1`, id,
	).Scan(&data, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", domain.ErrNotFound
	}
	return data, mime, err
}

func (r *UserRepo) Search(ctx context.Context, query string, excludeID uuid.UUID) ([]*domain.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, username, COALESCE(display_name,''), email,
		        (avatar_data IS NOT NULL), created_at, updated_at
		 FROM users
		 WHERE id != $1
		   AND (username ILIKE $2 OR display_name ILIKE $2)
		 ORDER BY username LIMIT 20`,
		excludeID, "%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.User
	for rows.Next() {
		u := &domain.User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email,
			&u.HasAvatar, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
