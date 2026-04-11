package http

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

type adminHandler struct{ pool *pgxpool.Pool }

func newAdminHandler(pool *pgxpool.Pool) *adminHandler { return &adminHandler{pool: pool} }

func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !middleware.IsAdminFromContext(r.Context()) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *adminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, username, email, tag, display_name, is_admin, created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	defer rows.Close()
	type userRow struct {
		ID          string  `json:"id"`
		Username    string  `json:"username"`
		Email       string  `json:"email"`
		Tag         string  `json:"tag"`
		DisplayName *string `json:"display_name"`
		IsAdmin     bool    `json:"is_admin"`
		CreatedAt   string  `json:"created_at"`
	}
	var users []userRow
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Tag, &u.DisplayName, &u.IsAdmin, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	if users == nil {
		users = []userRow{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *adminHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	// prevent self-delete
	if middleware.UserIDFromContext(r.Context()) == id {
		writeJSON(w, http.StatusBadRequest, errBody("cannot delete yourself"))
		return
	}
	if _, err := h.pool.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandler) listRooms(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT r.id, r.name, r.type, r.owner_id, u.username, r.created_at,
		        (SELECT COUNT(*) FROM room_members rm WHERE rm.room_id = r.id) AS member_count
		 FROM rooms r JOIN users u ON u.id = r.owner_id ORDER BY r.created_at DESC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	defer rows.Close()
	type roomRow struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		OwnerID     string `json:"owner_id"`
		OwnerName   string `json:"owner_username"`
		CreatedAt   string `json:"created_at"`
		MemberCount int    `json:"member_count"`
	}
	var rooms []roomRow
	for rows.Next() {
		var ro roomRow
		if err := rows.Scan(&ro.ID, &ro.Name, &ro.Type, &ro.OwnerID, &ro.OwnerName, &ro.CreatedAt, &ro.MemberCount); err != nil {
			continue
		}
		rooms = append(rooms, ro)
	}
	if rooms == nil {
		rooms = []roomRow{}
	}
	writeJSON(w, http.StatusOK, rooms)
}

func (h *adminHandler) deleteRoom(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid id"))
		return
	}
	if _, err := h.pool.Exec(r.Context(), `DELETE FROM rooms WHERE id = $1`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
