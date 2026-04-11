package http

import (
	"crypto/rsa"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/chat-back/internal/adapter/inbound/ws"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/config"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

func NewServer(
	cfg *config.Config,
	pool *pgxpool.Pool,
	hub *ws.Hub,
	roomSvc *app.RoomService,
	msgSvc *app.MessageService,
	presenceSvc *app.PresenceService,
	userSvc *app.UserService,
	friendSvc *app.FriendService,
	authMW *middleware.JWTMiddleware,
	privKey *rsa.PrivateKey,
) http.Handler {
	mux := http.NewServeMux()

	// Auth handlers (strict rate limit: 5 req/15min per IP)
	authH := newAuthHandler(privKey, pool)
	authLimit := middleware.RateLimit(5, 3*time.Minute)
	mux.Handle("POST /api/auth/register", authLimit(http.HandlerFunc(authH.register)))
	mux.Handle("POST /api/auth/login", authLimit(http.HandlerFunc(authH.login)))
	mux.Handle("POST /api/auth/refresh", authLimit(http.HandlerFunc(authH.refresh)))

	// Protected routes
	protected := authMW.Authenticate

	// WebSocket
	wsH := newWSHandler(hub, authMW, userSvc)
	mux.Handle("GET /api/ws", http.HandlerFunc(wsH.handle))

	// Rooms
	roomH := newRoomHandler(roomSvc)
	mux.Handle("GET /api/rooms", protected(http.HandlerFunc(roomH.list)))
	mux.Handle("POST /api/rooms", protected(http.HandlerFunc(roomH.create)))
	mux.Handle("DELETE /api/rooms/{id}", protected(http.HandlerFunc(roomH.delete)))
	mux.Handle("GET /api/rooms/{id}/members", protected(http.HandlerFunc(roomH.members)))

	// Messages
	msgH := newMessageHandler(msgSvc)
	mux.Handle("GET /api/rooms/{id}/messages", protected(http.HandlerFunc(msgH.history)))

	// User profile
	userH := newUserHandler(userSvc)
	mux.Handle("GET /api/me", protected(http.HandlerFunc(userH.me)))
	mux.Handle("PUT /api/me", protected(http.HandlerFunc(userH.updateProfile)))
	mux.Handle("POST /api/me/avatar", protected(http.HandlerFunc(userH.uploadAvatar)))
	mux.Handle("GET /api/users/{id}/avatar", http.HandlerFunc(userH.serveAvatar)) // public

	// Friends & DMs
	friendH := newFriendHandler(friendSvc, userSvc)
	mux.Handle("GET /api/users/search", protected(http.HandlerFunc(friendH.searchUsers)))
	mux.Handle("POST /api/friends/request", protected(http.HandlerFunc(friendH.sendRequest)))
	mux.Handle("GET /api/friends/requests", protected(http.HandlerFunc(friendH.listRequests)))
	mux.Handle("POST /api/friends/accept/{id}", protected(http.HandlerFunc(friendH.acceptRequest)))
	mux.Handle("DELETE /api/friends/{id}", protected(http.HandlerFunc(friendH.remove)))
	mux.Handle("GET /api/friends", protected(http.HandlerFunc(friendH.listFriends)))
	mux.Handle("POST /api/dms", protected(http.HandlerFunc(friendH.createDM)))
	mux.Handle("GET /api/dms", protected(http.HandlerFunc(friendH.listDMs)))

	// Health
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": "0.1.0"})
	})

	return applyGlobalMiddleware(mux, cfg.AllowedOrigins)
}
