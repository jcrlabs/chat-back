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
	roomH := newRoomHandler(roomSvc, hub)
	mux.Handle("GET /api/rooms", protected(http.HandlerFunc(roomH.list)))
	mux.Handle("POST /api/rooms", protected(http.HandlerFunc(roomH.create)))
	mux.Handle("PATCH /api/rooms/{id}", protected(http.HandlerFunc(roomH.rename)))
	mux.Handle("DELETE /api/rooms/{id}", protected(http.HandlerFunc(roomH.delete)))
	mux.Handle("GET /api/rooms/{id}/members", protected(http.HandlerFunc(roomH.members)))
	mux.Handle("PUT /api/rooms/{id}/members/{uid}/role", protected(http.HandlerFunc(roomH.setMemberRole)))
	mux.Handle("DELETE /api/rooms/{id}/members/{uid}", protected(http.HandlerFunc(roomH.kickMember)))
	mux.Handle("POST /api/rooms/{id}/invites", protected(http.HandlerFunc(roomH.inviteUser)))
	mux.Handle("GET /api/rooms/{id}/invites", protected(http.HandlerFunc(roomH.listRoomInvites)))
	mux.Handle("POST /api/invites/{id}/accept", protected(http.HandlerFunc(roomH.acceptInvite)))
	mux.Handle("POST /api/invites/{id}/decline", protected(http.HandlerFunc(roomH.declineInvite)))
	mux.Handle("GET /api/me/invites", protected(http.HandlerFunc(roomH.myInvites)))

	// Messages
	msgH := newMessageHandler(msgSvc, roomSvc, hub)
	mux.Handle("GET /api/rooms/{id}/messages", protected(http.HandlerFunc(msgH.history)))
	mux.Handle("PATCH /api/messages/{id}", protected(http.HandlerFunc(msgH.editMessage)))
	mux.Handle("DELETE /api/messages/{id}", protected(http.HandlerFunc(msgH.deleteMessage)))

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

	// Admin (requires is_admin JWT claim)
	adminH := newAdminHandler(pool)
	adminAuth := func(h http.Handler) http.Handler { return protected(requireAdmin(h)) }
	mux.Handle("GET /api/admin/users", adminAuth(http.HandlerFunc(adminH.listUsers)))
	mux.Handle("DELETE /api/admin/users/{id}", adminAuth(http.HandlerFunc(adminH.deleteUser)))
	mux.Handle("GET /api/admin/rooms", adminAuth(http.HandlerFunc(adminH.listRooms)))
	mux.Handle("DELETE /api/admin/rooms/{id}", adminAuth(http.HandlerFunc(adminH.deleteRoom)))

	// Health
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": "0.1.0"})
	})

	return applyGlobalMiddleware(mux, cfg.AllowedOrigins)
}
