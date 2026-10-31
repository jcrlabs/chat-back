package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	ContextUserID   contextKey = "user_id"
	ContextUsername contextKey = "username"
	ContextIsAdmin  contextKey = "is_admin"
)

type Claims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin,omitempty"`
}

type JWTMiddleware struct {
	pubKey *rsa.PublicKey
}

func NewJWTMiddleware(pubKey *rsa.PublicKey) *JWTMiddleware {
	return &JWTMiddleware{pubKey: pubKey}
}

func (m *JWTMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := m.extractToken(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		userID, username, isAdmin, err := m.ValidateToken(token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ContextUserID, userID)
		ctx = context.WithValue(ctx, ContextUsername, username)
		ctx = context.WithValue(ctx, ContextIsAdmin, isAdmin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *JWTMiddleware) extractToken(r *http.Request) (string, error) {
	// Try Authorization header first
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer "), nil
	}
	// For WebSocket: token in query param
	if t := r.URL.Query().Get("token"); t != "" {
		return t, nil
	}
	return "", errors.New("no token")
}

func (m *JWTMiddleware) ValidateToken(tokenStr string) (uuid.UUID, string, bool, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
		return m.pubKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil || !token.Valid {
		return uuid.Nil, "", false, errors.New("invalid token")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, "", false, errors.New("invalid subject")
	}
	return userID, claims.Username, claims.IsAdmin, nil
}

// IsAdminFromContext returns true if the authenticated user is an admin.
func IsAdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(ContextIsAdmin).(bool)
	return v
}

// UserIDFromContext extracts the authenticated user ID.
func UserIDFromContext(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(ContextUserID).(uuid.UUID)
	return v
}

// UsernameFromContext extracts the authenticated username.
func UsernameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ContextUsername).(string)
	return v
}

// ParseRSAPrivateKey parses a PEM-encoded RSA private key (PKCS#1 or PKCS#8).
func ParseRSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not RSA private key")
	}
	return rsaKey, nil
}

// ParseRSAPublicKey parses a PEM-encoded RSA public key.
func ParseRSAPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not RSA public key")
	}
	return rsaPub, nil
}
