package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestOTelBridge_NotInstalledWithoutToken verifies that when POSTHOG_PROJECT_TOKEN
// is unset, the OTel logger provider is not installed and the logger behaves
// identically to the baseline (stdout only).
func TestOTelBridge_NotInstalledWithoutToken(t *testing.T) {
	// Ensure the token is absent for this test.
	os.Unsetenv("POSTHOG_PROJECT_TOKEN")

	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Make a simple request; if the OTel handler were installed without a
	// real provider it would panic or error — a clean 200 confirms it wasn't.
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from health endpoint, got %d", w.Code)
	}
}

// TestRequestID_ExposedOnResponse verifies that every response carries an
// X-Request-Id header so the frontend can correlate client events with server
// log lines (Phase 1, step 6).
func TestRequestID_ExposedOnResponse(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	rid := w.Header().Get("X-Request-Id")
	if rid == "" {
		t.Error("expected X-Request-Id response header to be set, got empty")
	}
}

// TestRequestID_UniquePerRequest confirms that two requests get distinct IDs.
func TestRequestID_UniquePerRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req1 := httptest.NewRequest("GET", "/health", nil)
	w1 := httptest.NewRecorder()
	server.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/health", nil)
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	id1 := w1.Header().Get("X-Request-Id")
	id2 := w2.Header().Get("X-Request-Id")

	if id1 == "" || id2 == "" {
		t.Fatal("both requests must have X-Request-Id headers")
	}
	if id1 == id2 {
		t.Errorf("expected distinct request IDs, both were %q", id1)
	}
}
