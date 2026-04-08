package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/domain"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

type roomHandler struct {
	svc *app.RoomService
}

func newRoomHandler(svc *app.RoomService) *roomHandler {
	return &roomHandler{svc: svc}
}

func (h *roomHandler) list(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.svc.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, rooms)
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
	if err := h.svc.Delete(r.Context(), id, ownerID); err != nil {
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

func errBody(msg string) map[string]string {
	return map[string]string{"error": msg}
}
