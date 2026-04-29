package http

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jcrlabs/chat-back/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

type authHandler struct {
	privKey   *rsa.PrivateKey
	pool      *pgxpool.Pool
	demoEmail string
}

func newAuthHandler(privKey *rsa.PrivateKey, pool *pgxpool.Pool, demoEmail string) *authHandler {
	return &authHandler{privKey: privKey, pool: pool, demoEmail: demoEmail}
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *authHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing fields"))
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}

	userID := uuid.New()
	tag, err := generateUniqueTag(r.Context(), h.pool)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`INSERT INTO users (id, username, email, password, tag) VALUES ($1, $2, $3, $4, $5)`,
		userID, req.Username, req.Email, string(hash), tag,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "users_username_key":
				writeJSON(w, http.StatusConflict, errBody("username already taken"))
			case "users_email_key":
				writeJSON(w, http.StatusConflict, errBody("email already in use"))
			default:
				writeJSON(w, http.StatusConflict, errBody("username or email already exists"))
			}
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}

	tokens, err := h.issueTokens(userID, req.Username, false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	setRefreshCookie(w, tokens.RefreshToken)
	writeJSON(w, http.StatusCreated, map[string]string{"access_token": tokens.AccessToken})
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}

	var userID uuid.UUID
	var username, hash string
	var isAdmin bool
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, username, password, is_admin FROM users WHERE email = $1`, req.Email,
	).Scan(&userID, &username, &hash, &isAdmin)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errBody("invalid credentials"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, errBody("invalid credentials"))
		return
	}

	tokens, err := h.issueTokens(userID, username, isAdmin)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	setRefreshCookie(w, tokens.RefreshToken)
	writeJSON(w, http.StatusOK, map[string]string{"access_token": tokens.AccessToken})
}

func (h *authHandler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errBody("no refresh token"))
		return
	}

	authMW := middleware.NewJWTMiddleware(&h.privKey.PublicKey)
	userID, _, _, err := authMW.ValidateToken(cookie.Value)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errBody("invalid refresh token"))
		return
	}

	// Re-fetch username and is_admin from DB so changes take effect immediately.
	var username string
	var isAdmin bool
	err = h.pool.QueryRow(r.Context(),
		`SELECT username, is_admin FROM users WHERE id = $1`, userID,
	).Scan(&username, &isAdmin)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errBody("user not found"))
		return
	}

	tokens, err := h.issueTokens(userID, username, isAdmin)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	setRefreshCookie(w, tokens.RefreshToken)
	writeJSON(w, http.StatusOK, map[string]string{"access_token": tokens.AccessToken})
}

type tokenPair struct {
	AccessToken  string
	RefreshToken string
}

func (h *authHandler) issueTokens(userID uuid.UUID, username string, isAdmin bool) (*tokenPair, error) {
	now := time.Now()

	access := jwt.NewWithClaims(jwt.SigningMethodRS256, middleware.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Username: username,
		IsAdmin:  isAdmin,
	})
	accessStr, err := access.SignedString(h.privKey)
	if err != nil {
		return nil, err
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodRS256, middleware.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Username: username,
	})
	refreshStr, err := refresh.SignedString(h.privKey)
	if err != nil {
		return nil, err
	}

	return &tokenPair{AccessToken: accessStr, RefreshToken: refreshStr}, nil
}

func (h *authHandler) demo(w http.ResponseWriter, r *http.Request) {
	var userID uuid.UUID
	var username string
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, username FROM users WHERE email = $1`, h.demoEmail,
	).Scan(&userID, &username)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("demo not available"))
		return
	}

	now := time.Now()
	expiresAt := now.Add(30 * time.Minute)
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, middleware.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Username: username,
		IsAdmin:  false,
	}).SignedString(h.privKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_in": int(time.Until(expiresAt).Seconds()),
		"expires_at": expiresAt,
	})
}

func generateUniqueTag(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	for range 20 {
		tag := fmt.Sprintf("%04d", rand.IntN(9999)+1)
		var exists bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE tag = $1)`, tag).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return tag, nil
		}
	}
	return "", fmt.Errorf("could not generate unique tag")
}

func setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/auth",
		MaxAge:   7 * 24 * 3600,
	})
}
