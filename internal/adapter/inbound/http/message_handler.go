package http

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/app"
)

type messageHandler struct {
	svc *app.MessageService
}

func newMessageHandler(svc *app.MessageService) *messageHandler {
	return &messageHandler{svc: svc}
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
