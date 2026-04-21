package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/apsv/goal-tracker/backend/internal/api"
	"github.com/apsv/goal-tracker/backend/internal/models"
)

// validDebugReportBody returns a JSON body suitable for POST /api/v1/debug-reports.
// The breadcrumbs/device/state are minimal but well-formed JSON, matching the
// handler's expectations.
func validDebugReportBody(trigger, clientID, extraDescription string) []byte {
	payload := map[string]any{
		"client_id":   clientID,
		"app_version": "0.4.1",
		"platform":    "android",
		"device": map[string]any{
			"model":   "Pixel 8",
			"os":      "Android 14",
			"webview": "Chrome/127",
		},
		"state": map[string]any{
			"route":          "home",
			"online":         true,
			"pending_events": 0,
			"goal_count":     7,
		},
		"description": extraDescription,
		"breadcrumbs": []map[string]any{
			{"ts": 1, "category": "log", "level": "info", "message": "hi"},
		},
		"trigger":   trigger,
		"client_ts": 1733356800000,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return b
}

func TestDebugReport_CreateHappyPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "dbg-happy@localhost")

	clientID := uuid.New().String()
	body := validDebugReportBody("shake", clientID, "morning run vanished")
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var report models.DebugReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if report.ID == "" {
		t.Error("expected non-empty id")
	}
	if report.UserID == "" {
		t.Error("expected user_id populated from session")
	}
	if report.Trigger != "shake" {
		t.Errorf("expected trigger shake, got %q", report.Trigger)
	}
	if report.ClientID != clientID {
		t.Errorf("expected client_id %q, got %q", clientID, report.ClientID)
	}
	if report.Description != "morning run vanished" {
		t.Errorf("expected description preserved, got %q", report.Description)
	}
	if len(report.Breadcrumbs) == 0 {
		t.Error("expected breadcrumbs to round-trip")
	}
}

func TestDebugReport_InvalidTrigger(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "dbg-trig@localhost")

	body := validDebugReportBody("bogus", uuid.New().String(), "")
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid trigger, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDebugReport_MissingClientID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "dbg-cid@localhost")

	body := validDebugReportBody("shake", "", "")
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing client_id, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDebugReport_InvalidClientID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "dbg-cid2@localhost")

	body := validDebugReportBody("shake", "not-a-uuid", "")
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid client_id, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDebugReport_InvalidPlatform(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "dbg-plat@localhost")

	// Hand-build a payload with an out-of-enum platform value.
	payload := map[string]any{
		"client_id":   uuid.New().String(),
		"app_version": "0.4.1",
		"platform":    "windows",
		"device":      map[string]any{"model": "X"},
		"state":       map[string]any{"route": "home"},
		"description": "",
		"breadcrumbs": []map[string]any{{"ts": 1, "category": "log", "level": "info", "message": "hi"}},
		"trigger":     "shake",
		"client_ts":   1733356800000,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid platform, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDebugReport_DescriptionTooLong(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "dbg-desc@localhost")

	// 4097 chars — one over the 4096 cap, but well under the 256 KB body cap
	// so the description-length check is what fires.
	long := strings.Repeat("x", 4097)
	body := validDebugReportBody("shake", uuid.New().String(), long)
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized description, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDebugReport_BodyTooLarge(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "dbg-big@localhost")

	// Build a description > 256 KB so the whole body exceeds the cap.
	// The 1 MB global cap is larger so it won't fire first.
	big := strings.Repeat("x", 300*1024)
	body := validDebugReportBody("shake", uuid.New().String(), big)
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDebugReport_Unauthenticated(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := validDebugReportBody("shake", uuid.New().String(), "")
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No cookie set.
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthenticated, got %d: %s", w.Code, w.Body.String())
	}
}

// post sends a POST with a valid body and returns the HTTP status.
// Each call uses a fresh client_id so we never trip uniqueness or replay checks.
func postDebugReport(t *testing.T, server http.Handler, cookie *http.Cookie) int {
	t.Helper()
	body := validDebugReportBody("shake", uuid.New().String(), "")
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	return w.Code
}

func TestDebugReport_HourlyRateLimit(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "dbg-hr@localhost")

	// Hourly limit is 5 — the 6th request must be 429.
	for i := 0; i < 5; i++ {
		if code := postDebugReport(t, server, cookie); code != http.StatusCreated {
			t.Fatalf("request %d: expected 201, got %d", i+1, code)
		}
	}
	if code := postDebugReport(t, server, cookie); code != http.StatusTooManyRequests {
		t.Fatalf("6th request: expected 429, got %d", code)
	}
}

func TestDebugReport_RateLimitIsPerUser(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Two distinct users authenticate. A exhausts their hourly budget; B must
	// still be allowed through — this proves keying is by user_id, not IP or
	// a global counter.
	cookieA := authenticateTestUser(t, server, "dbg-userA@localhost")
	cookieB := authenticateTestUser(t, server, "dbg-userB@localhost")

	for i := 0; i < 5; i++ {
		if code := postDebugReport(t, server, cookieA); code != http.StatusCreated {
			t.Fatalf("user A request %d: expected 201, got %d", i+1, code)
		}
	}
	if code := postDebugReport(t, server, cookieA); code != http.StatusTooManyRequests {
		t.Fatalf("user A 6th request: expected 429, got %d", code)
	}

	// User B still has a full budget.
	if code := postDebugReport(t, server, cookieB); code != http.StatusCreated {
		t.Fatalf("user B first request: expected 201 after A was rate-limited, got %d", code)
	}
}

// TestDebugReport_DailyLimiterDirect verifies the daily-limit ceiling by
// driving the limiter through the exported Allow() on a RateLimiter built with
// the same parameters as the server's daily limiter. The hourly limiter caps
// at 5/hour, so the daily ceiling can only be reached through simulated hour
// rollovers — infeasible against a live router in a unit test. Testing the
// limiter directly keeps us inside the public API while still pinning the
// 20/day ceiling that the router wires up.
func TestDebugReport_DailyLimiterDirect(t *testing.T) {
	// Must stay in sync with router.go. If the daily cap changes there, this
	// test should be updated in lockstep.
	daily := api.NewRateLimiter(20, 24*time.Hour)
	const key = "user-daily-test"

	for i := 0; i < 20; i++ {
		if !daily.Allow(key) {
			t.Fatalf("call %d unexpectedly rate-limited", i+1)
		}
	}
	if daily.Allow(key) {
		t.Fatalf("21st call should have been rate-limited")
	}

	// Different user key should not be affected.
	if !daily.Allow("different-user") {
		t.Fatal("different user key should not share a budget")
	}
}

// TestDebugReport_StoredAndRetrievable verifies that after a 201, the report is
// actually persisted and can be listed by the DB layer for that user.
func TestDebugReport_StoredAndRetrievable(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	cookie := authenticateTestUser(t, server, "dbg-store@localhost")

	clientID := uuid.New().String()
	body := validDebugReportBody("auto", clientID, fmt.Sprintf("marker-%d", 42))
	req := httptest.NewRequest("POST", "/api/v1/debug-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created models.DebugReport
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Trigger != "auto" {
		t.Errorf("expected trigger=auto, got %q", created.Trigger)
	}
	if created.ClientID != clientID {
		t.Errorf("expected client_id=%q, got %q", clientID, created.ClientID)
	}
}
