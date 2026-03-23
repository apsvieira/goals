package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthRequired_ProtectedEndpoints(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/goals", ""},
		{"POST", "/api/v1/goals", `{"name":"x","color":"#000000"}`},
		{"PATCH", "/api/v1/goals/some-id", `{"name":"x"}`},
		{"DELETE", "/api/v1/goals/some-id", ""},
		{"PUT", "/api/v1/goals/reorder", `{"goal_ids":["a"]}`},
		{"GET", "/api/v1/completions?from=2026-01-01&to=2026-01-31", ""},
		{"POST", "/api/v1/completions", `{"goal_id":"x","date":"2026-01-01"}`},
		{"DELETE", "/api/v1/completions/some-id", ""},
		{"GET", "/api/v1/calendar?month=2026-01", ""},
		{"POST", "/api/v1/sync", `{"goals":[],"completions":[]}`},
		{"POST", "/api/v1/devices", `{"token":"x","platform":"android"}`},
		{"DELETE", "/api/v1/devices/some-id", ""},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body *bytes.Buffer
			if tc.body != "" {
				body = bytes.NewBufferString(tc.body)
			} else {
				body = &bytes.Buffer{}
			}

			req := httptest.NewRequest(tc.method, tc.path, body)
			req.Header.Set("Content-Type", "application/json")
			// No cookie — unauthenticated
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", w.Code)
			}
		})
	}
}
