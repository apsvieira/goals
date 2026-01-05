package api

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter provides simple in-memory rate limiting per IP address
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string]*rateLimitEntry
	limit    int
	window   time.Duration
}

type rateLimitEntry struct {
	count     int
	windowEnd time.Time
}

// NewRateLimiter creates a new rate limiter with the specified limit per window duration
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateLimitEntry),
		limit:    limit,
		window:   window,
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return rl
}

// Allow checks if the request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.requests[ip]

	if !exists || now.After(entry.windowEnd) {
		// New window
		rl.requests[ip] = &rateLimitEntry{
			count:     1,
			windowEnd: now.Add(rl.window),
		}
		return true
	}

	if entry.count >= rl.limit {
		return false
	}

	entry.count++
	return true
}

// cleanup periodically removes expired entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, entry := range rl.requests {
			if now.After(entry.windowEnd) {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware returns HTTP middleware that applies rate limiting
func RateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			// Use X-Real-IP if available (set by middleware.RealIP)
			if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
				ip = realIP
			}

			if !rl.Allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded, please try again later"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
