# CLAUDE.md — Real-Time Chat Backend (chat.jcrlabs.net)

> Extiende: `SHARED-CLAUDE.md` (sección Go + Hexagonal)

## Project Overview

Chat en tiempo real con WebSockets. Demuestra concurrencia Go (goroutines/channels), Redis pub/sub para escalar entre pods, y protocol design. Salas, DMs, typing, presencia.

## Tech Stack

- **Language**: Go 1.26
- **WebSocket**: `github.com/coder/websocket` (stdlib-friendly, sucesor de nhooyr)
- **HTTP**: `net/http` stdlib
- **DB**: PostgreSQL 17 — `github.com/jackc/pgx/v5` (pool)
- **PubSub/Cache**: Redis — `github.com/redis/go-redis/v9`
- **Auth**: `github.com/golang-jwt/jwt/v5` (validate en WS handshake)
- **ID generation**: `github.com/google/uuid` v7 (time-sorted)

## Architecture (Hexagonal)

```
chat-back/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── user.go                      # User entity
│   │   ├── room.go                      # Room (public/private/dm) + membership rules
│   │   ├── message.go                   # Message entity + validation rules
│   │   └── errors.go                    # ErrNotFound, ErrForbidden, ErrRoomFull
│   │
│   ├── app/
│   │   ├── room_service.go              # Create room, join/leave, list, permissions
│   │   │   // port: RoomRepository interface
│   │   ├── message_service.go           # Persist + load history (cursor-based)
│   │   │   // port: MessageRepository interface
│   │   ├── presence_service.go          # Track online/offline
│   │   │   // port: PresenceStore interface
│   │   └── chat_service.go             # Orquesta: validate message → persist → publish
│   │       // port: MessageBroadcaster interface
│   │
│   ├── adapter/
│   │   ├── inbound/
│   │   │   ├── http/
│   │   │   │   ├── server.go
│   │   │   │   ├── ws_handler.go        # Upgrade → authenticate → register client
│   │   │   │   ├── room_handler.go      # REST: CRUD rooms
│   │   │   │   └── message_handler.go   # REST: GET history (paginado)
│   │   │   │
│   │   │   └── ws/                      # ── WebSocket engine ──
│   │   │       ├── hub.go               # Central broker: register/unregister/route
│   │   │       ├── client.go            # Per-connection: readPump + writePump goroutines
│   │   │       └── protocol.go          # Marshal/unmarshal JSON protocol messages
│   │   │
│   │   └── outbound/
│   │       ├── postgres/
│   │       │   ├── room_repo.go         # Implements RoomRepository
│   │       │   └── message_repo.go      # Implements MessageRepository
│   │       └── redis/
│   │           ├── presence.go          # Implements PresenceStore (SET EX + EXPIRE)
│   │           └── pubsub.go            # Implements MessageBroadcaster (pub/sub per room)
│   │
│   ├── middleware/
│   └── config/
│
├── migrations/
├── k8s/
├── .golangci.yml
├── Makefile
└── Dockerfile
```

## WebSocket Protocol (typed JSON)

```go
// internal/adapter/inbound/ws/protocol.go

// Client → Server
type ClientMessage struct {
    Type    string          `json:"type"`     // join_room | leave_room | chat_message | typing
    RoomID  uuid.UUID       `json:"room_id"`
    Content string          `json:"content,omitempty"`   // solo chat_message
    Typing  *bool           `json:"is_typing,omitempty"` // solo typing
}

// Server → Client
type ServerMessage struct {
    Type      string    `json:"type"`      // chat_message | typing | presence | room_joined | error
    RoomID    uuid.UUID `json:"room_id,omitempty"`
    UserID    uuid.UUID `json:"user_id,omitempty"`
    Username  string    `json:"username,omitempty"`
    Content   string    `json:"content,omitempty"`
    Timestamp time.Time `json:"timestamp,omitempty"`
    Status    string    `json:"status,omitempty"`    // online | offline (presence)
    Members   []Member  `json:"members,omitempty"`   // room_joined
    Error     *Error    `json:"error,omitempty"`     // error
}
```

## Hub + Client pattern (concurrencia correcta)

```go
// internal/adapter/inbound/ws/client.go
type Client struct {
    hub      *Hub
    conn     *websocket.Conn
    userID   uuid.UUID
    username string
    rooms    map[uuid.UUID]struct{}   // rooms joined
    send     chan []byte              // buffered: writePump lee de aquí
}

// readPump: 1 goroutine por client — lee del WS, envía al hub
func (c *Client) readPump(ctx context.Context) {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close(websocket.StatusNormalClosure, "")
    }()
    for {
        _, msg, err := c.conn.Read(ctx)
        if err != nil {
            return  // connection closed
        }
        c.hub.incoming <- &Envelope{client: c, data: msg}
    }
}

// writePump: 1 goroutine por client — lee del channel send, escribe al WS
func (c *Client) writePump(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second) // ping interval
    defer ticker.Stop()
    for {
        select {
        case msg, ok := <-c.send:
            if !ok {
                return
            }
            c.conn.Write(ctx, websocket.MessageText, msg)
        case <-ticker.C:
            c.conn.Ping(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

## Redis pub/sub para multi-pod

```go
// Publish: cuando un mensaje llega al hub de un pod
func (r *RedisPubSub) Publish(ctx context.Context, roomID uuid.UUID, msg []byte) error {
    return r.client.Publish(ctx, "room:"+roomID.String(), msg).Err()
}

// Subscribe: goroutine que escucha Redis y reenvía al hub local
func (r *RedisPubSub) Subscribe(ctx context.Context, roomID uuid.UUID, onMessage func([]byte)) error {
    sub := r.client.Subscribe(ctx, "room:"+roomID.String())
    ch := sub.Channel()
    for {
        select {
        case <-ctx.Done():
            return sub.Close()
        case msg := <-ch:
            onMessage([]byte(msg.Payload))
        }
    }
}
```

## Presencia con Redis

```go
// internal/adapter/outbound/redis/presence.go
func (p *RedisPresence) SetOnline(ctx context.Context, userID uuid.UUID) error {
    return p.client.Set(ctx, "presence:"+userID.String(), "online", 45*time.Second).Err()
}

func (p *RedisPresence) Heartbeat(ctx context.Context, userID uuid.UUID) error {
    return p.client.Expire(ctx, "presence:"+userID.String(), 45*time.Second).Err()
}

func (p *RedisPresence) IsOnline(ctx context.Context, userID uuid.UUID) (bool, error) {
    n, err := p.client.Exists(ctx, "presence:"+userID.String()).Result()
    return n > 0, err
}
```

## Deploy

- **Dominio**: `chat.jcrlabs.net`
- **Namespace**: `chat`
- **Ingress annotations** (críticas para WebSocket):
  ```yaml
  nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
  nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
  nginx.ingress.kubernetes.io/proxy-http-version: "1.1"
  nginx.ingress.kubernetes.io/upstream-hash-by: "$remote_addr"
  ```
- **Redis**: Deployment PVC 2Gi
- **PostgreSQL**: PVC 5Gi
- **HPA**: 2-4 replicas

## CI local

Ejecutar **antes de cada commit** para evitar que lleguen errores a GitHub Actions:

```bash
gofmt -l .                      # no debe mostrar ficheros
go vet ./...
golangci-lint run --timeout=5m
go test -race ./...
```
## Git

- Ramas: `feature/`, `bugfix/`, `hotfix/`, `release/` — sin prefijos adicionales
- Commits: convencional (`feat:`, `fix:`, `chore:`, etc.) — sin mencionar herramientas externas ni agentes en el mensaje
- PRs: título y descripción propios del cambio — sin mencionar herramientas externas ni agentes
- Comentarios y documentación: redactar en primera persona del equipo — sin atribuir autoría a herramientas
