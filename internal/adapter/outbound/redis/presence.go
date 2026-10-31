package redis

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const presenceTTL = 45 * time.Second

type Presence struct {
	client *redis.ClusterClient
}

func NewPresence(client *redis.ClusterClient) *Presence {
	return &Presence{client: client}
}

func (p *Presence) SetOnline(ctx context.Context, userID uuid.UUID) error {
	return p.client.Set(ctx, presenceKey(userID), "online", presenceTTL).Err()
}

func (p *Presence) SetOffline(ctx context.Context, userID uuid.UUID) error {
	return p.client.Del(ctx, presenceKey(userID)).Err()
}

func (p *Presence) Heartbeat(ctx context.Context, userID uuid.UUID) error {
	return p.client.Expire(ctx, presenceKey(userID), presenceTTL).Err()
}

func (p *Presence) IsOnline(ctx context.Context, userID uuid.UUID) (bool, error) {
	n, err := p.client.Exists(ctx, presenceKey(userID)).Result()
	return n > 0, err
}

func presenceKey(userID uuid.UUID) string {
	return "presence:" + userID.String()
}
