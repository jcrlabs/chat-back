package http

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	"github.com/jcrlabs/chat-back/internal/adapter/inbound/ws"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

type wsHandler struct {
	hub     *ws.Hub
	authMW  *middleware.JWTMiddleware
	userSvc *app.UserService
}

func newWSHandler(hub *ws.Hub, authMW *middleware.JWTMiddleware, userSvc *app.UserService) *wsHandler {
	return &wsHandler{hub: hub, authMW: authMW, userSvc: userSvc}
}

func (h *wsHandler) handle(w http.ResponseWriter, r *http.Request) {
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

	var displayName, avatarURL string
	if user, err := h.userSvc.GetByID(r.Context(), userID); err == nil {
		displayName = user.DisplayName
		if user.HasAvatar {
			avatarURL = "/api/users/" + userID.String() + "/avatar"
		}
	}

	client := ws.NewClient(h.hub, conn, userID, username, displayName, avatarURL)
	h.hub.RegisterClient(context.Background(), client)
}
