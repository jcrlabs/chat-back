package http

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jcrlabs/chat-back/internal/middleware"
)

// globalLimiter allows 200 req/min per IP (1 token every 300ms).
var globalLimiter = middleware.RateLimit(200, 300*time.Millisecond)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func applyGlobalMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	limited := globalLimiter(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		origin := r.Header.Get("Origin")
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		limited.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s auth=%v",
			r.Method, r.URL.Path, rec.status,
			time.Since(start).Round(time.Millisecond),
			r.Header.Get("Authorization") != "",
		)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
