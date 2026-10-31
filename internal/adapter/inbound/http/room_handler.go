package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/adapter/inbound/ws"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/domain"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

type roomHandler struct {
	svc *app.RoomService
	hub interface {
		PublishToRoom(ctx context.Context, roomID uuid.UUID, data []byte) error
	}
}

func newRoomHandler(svc *app.RoomService, hub *ws.Hub) *roomHandler {
	return &roomHandler{svc: svc, hub: hub}
}

func (h *roomHandler) list(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	isAdmin := middleware.IsAdminFromContext(r.Context())
	var rooms interface{}
	var err error
	if isAdmin {
		rooms, err = h.svc.ListAll(r.Context())
	} else {
		rooms, err = h.svc.List(r.Context(), userID)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, rooms)
}

func (h *roomHandler) unreadCounts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	counts, err := h.svc.GetUnreadCounts(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	type item struct {
		RoomID string `json:"room_id"`
		Count  int    `json:"count"`
	}
	result := make([]item, 0, len(counts))
	for roomID, count := range counts {
		result = append(result, item{RoomID: roomID.String(), Count: count})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *roomHandler) markRead(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.MarkRoomRead(r.Context(), userID, roomID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *roomHandler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string          `json:"name"`
		Type domain.RoomType `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}
	if body.Type == "" {
		body.Type = domain.RoomTypePublic
	}

	ownerID := middleware.UserIDFromContext(r.Context())
	room, err := h.svc.Create(r.Context(), body.Name, body.Type, ownerID)
	if err != nil {
		if errors.Is(err, domain.ErrBadRequest) {
			writeJSON(w, http.StatusBadRequest, errBody(err.Error()))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

func (h *roomHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	ownerID := middleware.UserIDFromContext(r.Context())
	isAdmin := middleware.IsAdminFromContext(r.Context())
	if err := h.svc.Delete(r.Context(), id, ownerID, isAdmin); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *roomHandler) members(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	members, err := h.svc.GetMembers(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *roomHandler) inviteUser(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	var body struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}
	inviterID := middleware.UserIDFromContext(r.Context())
	invite, err := h.svc.InviteUser(r.Context(), roomID, inviterID, body.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrBadRequest) {
			writeJSON(w, http.StatusBadRequest, errBody(err.Error()))
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errBody("not found"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusCreated, invite)
}

func (h *roomHandler) listRoomInvites(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	requesterID := middleware.UserIDFromContext(r.Context())
	invites, err := h.svc.ListRoomInvites(r.Context(), roomID, requesterID)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, invites)
}

func (h *roomHandler) acceptInvite(w http.ResponseWriter, r *http.Request) {
	inviteID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.AcceptInvite(r.Context(), inviteID, userID); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errBody("not found"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *roomHandler) declineInvite(w http.ResponseWriter, r *http.Request) {
	inviteID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.DeclineInvite(r.Context(), inviteID, userID); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errBody("not found"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *roomHandler) setMemberRole(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	targetID, err := uuid.Parse(r.PathValue("uid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid uid"))
		return
	}
	var body struct {
		Role domain.MemberRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}
	requesterID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.SetRole(r.Context(), roomID, requesterID, targetID, body.Role); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errBody("not found"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *roomHandler) kickMember(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	targetID, err := uuid.Parse(r.PathValue("uid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid uid"))
		return
	}
	requesterID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.KickMember(r.Context(), roomID, requesterID, targetID); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errBody("not found"))
			return
		}
		if errors.Is(err, domain.ErrBadRequest) {
			writeJSON(w, http.StatusBadRequest, errBody(err.Error()))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *roomHandler) myInvites(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	invites, err := h.svc.ListMyInvites(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, invites)
}

func (h *roomHandler) rename(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}
	requesterID := middleware.UserIDFromContext(r.Context())
	isAdminRename := middleware.IsAdminFromContext(r.Context())
	room, err := h.svc.Rename(r.Context(), id, requesterID, body.Name, isAdminRename)
	if err != nil {
		if errors.Is(err, domain.ErrBadRequest) {
			writeJSON(w, http.StatusBadRequest, errBody(err.Error()))
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errBody("not found"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}

	out := ws.ServerMessage{
		Type:   ws.TypeRoomRenamed,
		RoomID: room.ID,
		Name:   room.Name,
	}
	if data, err := json.Marshal(out); err == nil {
		_ = h.hub.PublishToRoom(r.Context(), room.ID, data)
	}

	writeJSON(w, http.StatusOK, room)
}

func errBody(msg string) map[string]string {
	return map[string]string{"error": msg}
}
