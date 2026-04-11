package ws

import (
	"time"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/domain"
)

// ClientMessage — messages sent from client to server
type ClientMessage struct {
	Type    string    `json:"type"` // join_room | leave_room | chat_message | typing | voice_*
	RoomID  uuid.UUID `json:"room_id"`
	Content string    `json:"content,omitempty"`   // chat_message only
	Typing  *bool     `json:"is_typing,omitempty"` // typing only
	// Voice signaling — target peer for offer/answer/ice
	TargetUserID  uuid.UUID `json:"target_user_id,omitempty"`
	SDP           string    `json:"sdp,omitempty"`
	SDPType       string    `json:"sdp_type,omitempty"`
	Candidate     string    `json:"candidate,omitempty"`
	SDPMid        string    `json:"sdp_mid,omitempty"`
	SDPMLineIndex *uint16   `json:"sdp_m_line_index,omitempty"`
}

// ServerMessage — messages sent from server to client
type ServerMessage struct {
	Type        string          `json:"type"` // chat_message | typing | presence | room_joined | error | voice_*
	RoomID      uuid.UUID       `json:"room_id,omitempty"`
	UserID      uuid.UUID       `json:"user_id,omitempty"`
	Username    string          `json:"username,omitempty"`
	DisplayName string          `json:"display_name,omitempty"`
	AvatarURL   string          `json:"avatar_url,omitempty"`
	Content     string          `json:"content,omitempty"`
	Timestamp   time.Time       `json:"timestamp,omitempty"`
	Status      string          `json:"status,omitempty"` // online | offline
	Members     []domain.Member `json:"members,omitempty"`
	Error       *ServerError    `json:"error,omitempty"`
	// Voice signaling
	SDP          string      `json:"sdp,omitempty"`
	SDPType      string      `json:"sdp_type,omitempty"`
	Candidate    string      `json:"candidate,omitempty"`
	SDPMid       string      `json:"sdp_mid,omitempty"`
	SDPMLineIndex *uint16    `json:"sdp_m_line_index,omitempty"`
	Participants []uuid.UUID `json:"participants,omitempty"`
}

type ServerError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	TypeJoinRoom    = "join_room"
	TypeLeaveRoom   = "leave_room"
	TypeChatMessage = "chat_message"
	TypeTyping      = "typing"
	TypePresence    = "presence"
	TypeRoomJoined  = "room_joined"
	TypeError       = "error"
	// Voice
	TypeVoiceJoin         = "voice_join"
	TypeVoiceLeave        = "voice_leave"
	TypeVoiceOffer        = "voice_offer"
	TypeVoiceAnswer       = "voice_answer"
	TypeICECandidate      = "ice_candidate"
	TypeVoiceJoined       = "voice_joined"
	TypeVoiceLeft         = "voice_left"
	TypeVoiceParticipants = "voice_participants"
)
