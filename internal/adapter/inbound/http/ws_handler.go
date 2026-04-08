package http

import (
	"net/http"

	"github.com/coder/websocket"
	"github.com/jcrlabs/chat-back/internal/adapter/inbound/ws"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

type wsHandler struct {
	hub    *ws.Hub
	authMW *middleware.JWTMiddleware
}

func newWSHandler(hub *ws.Hub, authMW *middleware.JWTMiddleware) *wsHandler {
	return &wsHandler{hub: hub, authMW: authMW}
}

func (h *wsHandler) handle(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query param token (WS can't set headers)
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID, username, err := h.authMW.ValidateToken(token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"chat.jcrlabs.net", "localhost:*"},
	})
	if err != nil {
		return
	}

	client := ws.NewClient(h.hub, conn, userID, username)
	h.hub.RegisterClient(r.Context(), client)
}
