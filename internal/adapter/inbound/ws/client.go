package ws

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

const (
	sendBufferSize = 256
	pingInterval   = 30 * time.Second
)

// Client represents a single WebSocket connection.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	userID   uuid.UUID
	username string
	rooms    map[uuid.UUID]struct{} // rooms joined
	send     chan []byte            // buffered: writePump reads from here
}

func NewClient(hub *Hub, conn *websocket.Conn, userID uuid.UUID, username string) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		userID:   userID,
		username: username,
		rooms:    make(map[uuid.UUID]struct{}),
		send:     make(chan []byte, sendBufferSize),
	}
}

// readPump pumps messages from the WebSocket to the hub.
// One goroutine per connection.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.sendError("invalid_message", "invalid JSON")
			continue
		}

		c.hub.incoming <- &Envelope{client: c, msg: msg}
	}
}

// writePump pumps messages from the send channel to the WebSocket.
// One goroutine per connection.
func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.Ping(ctx); err != nil {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) sendError(code, message string) {
	msg := ServerMessage{
		Type:  TypeError,
		Error: &ServerError{Code: code, Message: message},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) sendMsg(msg ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal: %v", err)
		return
	}
	select {
	case c.send <- data:
	default:
		// drop if buffer full
	}
}
