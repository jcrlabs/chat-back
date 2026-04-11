package ws

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/domain"
)

// Envelope carries a parsed client message along with its sender.
type Envelope struct {
	client *Client
	msg    ClientMessage
}

// Broadcaster is the port for subscribing/publishing to Redis pub/sub.
type Broadcaster interface {
	Publish(ctx context.Context, roomID uuid.UUID, msg []byte) error
	Subscribe(ctx context.Context, roomID uuid.UUID, onMessage func([]byte)) error
	Unsubscribe(ctx context.Context, roomID uuid.UUID) error
}

// Hub is the central broker. It runs in a single goroutine to avoid races.
type Hub struct {
	clients    map[*Client]struct{}
	rooms      map[uuid.UUID]map[*Client]struct{} // roomID → clients
	voiceRooms map[uuid.UUID]map[*Client]struct{} // roomID → voice clients
	register   chan *Client
	unregister chan *Client
	incoming   chan *Envelope
	broadcast  chan broadcastMsg

	chatSvc     *app.ChatService
	presenceSvc *app.PresenceService
	roomSvc     *app.RoomService
	broadcaster Broadcaster
}

type broadcastMsg struct {
	roomID uuid.UUID
	data   []byte
}

func NewHub(chatSvc *app.ChatService, presenceSvc *app.PresenceService, roomSvc *app.RoomService, broadcaster Broadcaster) *Hub {
	return &Hub{
		clients:     make(map[*Client]struct{}),
		rooms:       make(map[uuid.UUID]map[*Client]struct{}),
		voiceRooms:  make(map[uuid.UUID]map[*Client]struct{}),
		register:    make(chan *Client, 64),
		unregister:  make(chan *Client, 64),
		incoming:    make(chan *Envelope, 256),
		broadcast:   make(chan broadcastMsg, 256),
		chatSvc:     chatSvc,
		presenceSvc: presenceSvc,
		roomSvc:     roomSvc,
		broadcaster: broadcaster,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = struct{}{}
			ctx := context.Background()
			_ = h.presenceSvc.SetOnline(ctx, c.userID)

		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
				for roomID := range c.rooms {
					h.leaveRoom(c, roomID)
				}
				// Clean up voice rooms
				for roomID := range h.voiceRooms {
					if _, inVoice := h.voiceRooms[roomID][c]; inVoice {
						h.handleVoiceLeave(c, roomID)
					}
				}
				ctx := context.Background()
				_ = h.presenceSvc.SetOffline(ctx, c.userID)
				h.broadcastPresence(c.userID, c.username, "offline")
			}

		case env := <-h.incoming:
			h.handleMessage(env)

		case bm := <-h.broadcast:
			h.deliverLocal(bm.roomID, bm.data)
		}
	}
}

// RegisterClient registers a new client and starts its pumps.
// It blocks until the client disconnects so that the caller's context
// (typically r.Context()) is not cancelled while the session is alive.
func (h *Hub) RegisterClient(ctx context.Context, c *Client) {
	h.register <- c
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.readPump(ctx)
	}()
	go c.writePump(ctx)
	<-done
}

func (h *Hub) handleMessage(env *Envelope) {
	c, msg := env.client, env.msg
	ctx := context.Background()

	switch msg.Type {
	case TypeJoinRoom:
		h.joinRoom(ctx, c, msg.RoomID)
	case TypeLeaveRoom:
		h.leaveRoom(c, msg.RoomID)
	case TypeChatMessage:
		h.handleChat(ctx, c, msg)
	case TypeTyping:
		h.handleTyping(c, msg)
	case TypeVoiceJoin:
		h.handleVoiceJoin(c, msg.RoomID)
	case TypeVoiceLeave:
		h.handleVoiceLeave(c, msg.RoomID)
	case TypeVoiceOffer, TypeVoiceAnswer, TypeICECandidate:
		h.relayVoiceSignal(c, msg)
	}
}

func (h *Hub) joinRoom(ctx context.Context, c *Client, roomID uuid.UUID) {
	if _, joined := c.rooms[roomID]; joined {
		return
	}
	// Check access for private rooms
	room, err := h.roomSvc.GetRoom(ctx, roomID)
	if err != nil {
		c.sendError("room_not_found", "room not found")
		return
	}
	if room.Type == "private" && room.OwnerID != c.userID {
		ok, err := h.roomSvc.IsMember(ctx, roomID, c.userID)
		if err != nil || !ok {
			c.sendError("forbidden", "not a member of this room")
			return
		}
	}
	c.rooms[roomID] = struct{}{}
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]struct{})
		// subscribe to Redis pub/sub for this room
		go func() {
			if err := h.broadcaster.Subscribe(ctx, roomID, func(data []byte) {
				h.broadcast <- broadcastMsg{roomID: roomID, data: data}
			}); err != nil {
				log.Printf("subscribe room %s: %v", roomID, err)
			}
		}()
	}
	h.rooms[roomID][c] = struct{}{}

	// build members list
	var members []domain.Member
	for client := range h.rooms[roomID] {
		members = append(members, domain.Member{UserID: client.userID, Username: client.username})
	}

	c.sendMsg(ServerMessage{
		Type:    TypeRoomJoined,
		RoomID:  roomID,
		Members: members,
	})
}

func (h *Hub) leaveRoom(c *Client, roomID uuid.UUID) {
	delete(c.rooms, roomID)
	if room, ok := h.rooms[roomID]; ok {
		delete(room, c)
		if len(room) == 0 {
			delete(h.rooms, roomID)
			_ = h.broadcaster.Unsubscribe(context.Background(), roomID)
		}
	}
}

func (h *Hub) handleChat(ctx context.Context, c *Client, msg ClientMessage) {
	saved, err := h.chatSvc.SendMessage(ctx, c.userID, c.username, msg.RoomID, msg.Content)
	if err != nil {
		c.sendError("send_failed", err.Error())
		return
	}

	out := ServerMessage{
		Type:        TypeChatMessage,
		RoomID:      saved.RoomID,
		UserID:      saved.UserID,
		Username:    saved.Username,
		DisplayName: c.displayName,
		AvatarURL:   c.avatarURL,
		Content:     saved.Content,
		Timestamp:   saved.CreatedAt,
	}
	data, _ := json.Marshal(out)

	// publish to Redis so all pods receive it
	_ = h.broadcaster.Publish(ctx, msg.RoomID, data)
}

func (h *Hub) handleTyping(c *Client, msg ClientMessage) {
	if msg.Typing == nil {
		return
	}
	out := ServerMessage{
		Type:     TypeTyping,
		RoomID:   msg.RoomID,
		UserID:   c.userID,
		Username: c.username,
	}
	data, _ := json.Marshal(out)
	h.deliverLocal(msg.RoomID, data)
}

func (h *Hub) deliverLocal(roomID uuid.UUID, data []byte) {
	for client := range h.rooms[roomID] {
		select {
		case client.send <- data:
		default:
		}
	}
}

func (h *Hub) handleVoiceJoin(c *Client, roomID uuid.UUID) {
	if h.voiceRooms[roomID] == nil {
		h.voiceRooms[roomID] = make(map[*Client]struct{})
	}
	h.voiceRooms[roomID][c] = struct{}{}

	// Send current participants to new joiner
	var participants []uuid.UUID
	for peer := range h.voiceRooms[roomID] {
		if peer != c {
			participants = append(participants, peer.userID)
		}
	}
	c.sendMsg(ServerMessage{Type: TypeVoiceParticipants, RoomID: roomID, Participants: participants})

	// Notify others
	joined, _ := json.Marshal(ServerMessage{Type: TypeVoiceJoined, RoomID: roomID, UserID: c.userID, Username: c.username})
	for peer := range h.voiceRooms[roomID] {
		if peer != c {
			select {
			case peer.send <- joined:
			default:
			}
		}
	}
}

func (h *Hub) handleVoiceLeave(c *Client, roomID uuid.UUID) {
	if room, ok := h.voiceRooms[roomID]; ok {
		delete(room, c)
		if len(room) == 0 {
			delete(h.voiceRooms, roomID)
		}
	}
	left, _ := json.Marshal(ServerMessage{Type: TypeVoiceLeft, RoomID: roomID, UserID: c.userID, Username: c.username})
	for peer := range h.voiceRooms[roomID] {
		select {
		case peer.send <- left:
		default:
		}
	}
}

func (h *Hub) relayVoiceSignal(c *Client, msg ClientMessage) {
	if msg.TargetUserID == (uuid.UUID{}) {
		return
	}
	out := ServerMessage{
		Type:          msg.Type,
		RoomID:        msg.RoomID,
		UserID:        c.userID,
		SDP:           msg.SDP,
		SDPType:       msg.SDPType,
		Candidate:     msg.Candidate,
		SDPMid:        msg.SDPMid,
		SDPMLineIndex: msg.SDPMLineIndex,
	}
	data, _ := json.Marshal(out)
	for peer := range h.voiceRooms[msg.RoomID] {
		if peer.userID == msg.TargetUserID {
			select {
			case peer.send <- data:
			default:
			}
			return
		}
	}
}

func (h *Hub) broadcastPresence(userID uuid.UUID, username, status string) {
	out := ServerMessage{
		Type:      TypePresence,
		UserID:    userID,
		Username:  username,
		Status:    status,
		Timestamp: time.Now().UTC(),
	}
	data, _ := json.Marshal(out)
	// deliver to all connected clients
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
		}
	}
}
