package redis

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// PubSub implements ws.Broadcaster and app.MessageBroadcaster using Redis Cluster pub/sub.
type PubSub struct {
	client *redis.ClusterClient

	mu   sync.Mutex
	subs map[string]*redis.PubSub // channel → subscription
}

func NewPubSub(client *redis.ClusterClient) *PubSub {
	return &PubSub{
		client: client,
		subs:   make(map[string]*redis.PubSub),
	}
}

func (ps *PubSub) Publish(ctx context.Context, roomID uuid.UUID, msg []byte) error {
	return ps.client.Publish(ctx, roomChannel(roomID), msg).Err()
}

func (ps *PubSub) Subscribe(ctx context.Context, roomID uuid.UUID, onMessage func([]byte)) error {
	ch := roomChannel(roomID)

	sub := ps.client.Subscribe(ctx, ch)
	ps.mu.Lock()
	ps.subs[ch] = sub
	ps.mu.Unlock()

	msgCh := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return sub.Close()
		case m, ok := <-msgCh:
			if !ok {
				return nil
			}
			onMessage([]byte(m.Payload))
		}
	}
}

func (ps *PubSub) Unsubscribe(ctx context.Context, roomID uuid.UUID) error {
	ch := roomChannel(roomID)
	ps.mu.Lock()
	sub, ok := ps.subs[ch]
	if ok {
		delete(ps.subs, ch)
	}
	ps.mu.Unlock()

	if ok {
		return sub.Close()
	}
	return nil
}

func roomChannel(roomID uuid.UUID) string {
	return "room:" + roomID.String()
}
