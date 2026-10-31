package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/domain"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

type friendHandler struct {
	svc     *app.FriendService
	userSvc *app.UserService
}

func newFriendHandler(svc *app.FriendService, userSvc *app.UserService) *friendHandler {
	return &friendHandler{svc: svc, userSvc: userSvc}
}

// GET /api/users/search?q=
func (h *friendHandler) searchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if !strings.Contains(q, "#") {
		writeJSON(w, http.StatusBadRequest, errBody("q must be in username#tag format"))
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	users, err := h.userSvc.Search(r.Context(), q, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// POST /api/friends/request  body: {"addressee_id": "uuid"}
func (h *friendHandler) sendRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AddresseeID uuid.UUID `json:"addressee_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AddresseeID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.SendRequest(r.Context(), userID, req.AddresseeID); err != nil {
		if err == domain.ErrBadRequest {
			writeJSON(w, http.StatusBadRequest, errBody("cannot send request to yourself"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/friends/requests
func (h *friendHandler) listRequests(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	reqs, err := h.svc.ListPendingReceived(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, reqs)
}

// POST /api/friends/accept/{id}
func (h *friendHandler) acceptRequest(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.Accept(r.Context(), id, userID); err != nil {
		if err == domain.ErrNotFound {
			writeJSON(w, http.StatusNotFound, errBody("request not found"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/friends/{id}
func (h *friendHandler) remove(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.Remove(r.Context(), id, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/friends
func (h *friendHandler) listFriends(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	friends, err := h.svc.ListFriends(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, friends)
}

// POST /api/dms  body: {"user_id": "uuid"}
func (h *friendHandler) createDM(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	room, err := h.svc.GetOrCreateDM(r.Context(), userID, req.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, room)
}

// GET /api/dms
func (h *friendHandler) listDMs(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	dms, err := h.svc.ListDMs(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, dms)
}
