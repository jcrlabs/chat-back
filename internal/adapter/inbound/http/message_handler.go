package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/adapter/inbound/ws"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/domain"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

type messageHandler struct {
	svc     *app.MessageService
	roomSvc *app.RoomService
	hub     interface {
		PublishToRoom(ctx context.Context, roomID uuid.UUID, data []byte) error
	}
}

func newMessageHandler(svc *app.MessageService, roomSvc *app.RoomService, hub *ws.Hub) *messageHandler {
	return &messageHandler{svc: svc, roomSvc: roomSvc, hub: hub}
}

func (h *messageHandler) history(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}

	cursor := uuid.Nil
	if c := r.URL.Query().Get("cursor"); c != "" {
		if cursor, err = uuid.Parse(c); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("invalid cursor"))
			return
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if limit, err = strconv.Atoi(l); err != nil || limit <= 0 {
			writeJSON(w, http.StatusBadRequest, errBody("invalid limit"))
			return
		}
	}

	msgs, err := h.svc.History(r.Context(), roomID, cursor, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (h *messageHandler) editMessage(w http.ResponseWriter, r *http.Request) {
	msgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}

	requesterID := middleware.UserIDFromContext(r.Context())
	msg, err := h.svc.Edit(r.Context(), msgID, requesterID, body.Content)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errBody("not found"))
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		if errors.Is(err, domain.ErrBadRequest) {
			writeJSON(w, http.StatusBadRequest, errBody(err.Error()))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}

	out := ws.ServerMessage{
		Type:      ws.TypeMessageEdited,
		MessageID: msg.ID,
		RoomID:    msg.RoomID,
		Content:   msg.Content,
		EditedAt:  msg.EditedAt,
	}
	if data, err := json.Marshal(out); err == nil {
		_ = h.hub.PublishToRoom(r.Context(), msg.RoomID, data)
	}

	writeJSON(w, http.StatusOK, msg)
}

func (h *messageHandler) deleteMessage(w http.ResponseWriter, r *http.Request) {
	msgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}

	requesterID := middleware.UserIDFromContext(r.Context())
	msg, err := h.svc.GetByID(r.Context(), msgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errBody("not found"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}

	if msg.UserID != requesterID {
		// site admins can delete any message
		if !middleware.IsAdminFromContext(r.Context()) {
			// check if requester is room owner or admin role
			role, roleErr := h.roomSvc.GetMemberRole(r.Context(), msg.RoomID, requesterID)
			if roleErr != nil || (role != domain.RoleOwner && role != domain.RoleAdmin) {
				writeJSON(w, http.StatusForbidden, errBody("forbidden"))
				return
			}
		}
	}

	if err := h.svc.DeleteByID(r.Context(), msgID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}

	out := ws.ServerMessage{
		Type:      ws.TypeMessageDeleted,
		MessageID: msgID,
		RoomID:    msg.RoomID,
	}
	if data, err := json.Marshal(out); err == nil {
		_ = h.hub.PublishToRoom(r.Context(), msg.RoomID, data)
	}

	w.WriteHeader(http.StatusNoContent)
}
