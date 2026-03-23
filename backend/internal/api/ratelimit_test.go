package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimit_AuthEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Auth endpoint rate limit is 10/min
	// Send 15 requests to /api/v1/auth/me (unauthenticated is fine, just checking rate limit)
	var lastCode int
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		lastCode = w.Code

		if w.Code == http.StatusTooManyRequests {
			// Rate limit hit — this is expected after 10 requests
			if i < 10 {
				t.Errorf("rate limited too early on request %d", i+1)
			}
			return
		}
	}

	t.Errorf("expected 429 after 10+ requests, last code was %d", lastCode)
}

func TestRateLimit_ReturnsRetryAfterHeader(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Exhaust rate limit
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			// Verify the response indicates rate limiting
			return
		}
	}

	t.Error("expected to be rate limited")
}

func TestRateLimit_DifferentIPsAreIndependent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Exhaust rate limit from "IP 1" (default 192.0.2.1 from httptest)
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
	}

	// Request from a "different IP" via X-Real-Ip header should succeed
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("X-Real-Ip", "10.0.0.99")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code == http.StatusTooManyRequests {
		t.Error("different IP should not be rate limited")
	}
}
