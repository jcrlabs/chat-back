package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// tokenBucket is a simple per-key token bucket rate limiter.
type tokenBucket struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	capacity  int
	refillPer time.Duration // refill 1 token per this duration
}

type bucket struct {
	tokens   int
	lastSeen time.Time
}

func newTokenBucket(capacity int, refillPer time.Duration) *tokenBucket {
	tb := &tokenBucket{
		buckets:   make(map[string]*bucket),
		capacity:  capacity,
		refillPer: refillPer,
	}
	// cleanup goroutine
	go func() {
		for range time.Tick(5 * time.Minute) {
			tb.mu.Lock()
			for k, b := range tb.buckets {
				if time.Since(b.lastSeen) > 10*time.Minute {
					delete(tb.buckets, k)
				}
			}
			tb.mu.Unlock()
		}
	}()
	return tb
}

// allow returns true if the request is allowed.
func (tb *tokenBucket) allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, ok := tb.buckets[key]
	if !ok {
		b = &bucket{tokens: tb.capacity}
		tb.buckets[key] = b
	}

	// refill tokens based on elapsed time
	elapsed := time.Since(b.lastSeen)
	refilled := int(elapsed / tb.refillPer)
	if refilled > 0 {
		b.tokens = min(tb.capacity, b.tokens+refilled)
		b.lastSeen = time.Now()
	}

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	if b.lastSeen.IsZero() {
		b.lastSeen = time.Now()
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RateLimit returns a middleware that limits requests per IP.
func RateLimit(capacity int, refillPer time.Duration) func(http.Handler) http.Handler {
	tb := newTokenBucket(capacity, refillPer)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !tb.allow(ip) {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// X-Forwarded-For may be "client, proxy1, proxy2" — take the first entry
		if idx := strings.IndexByte(fwd, ','); idx != -1 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	// strip port from RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
