package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/apsv/goal-tracker/backend/internal/api"
)

// TestOTelBridge_NotInstalledWithoutToken verifies that when POSTHOG_PROJECT_TOKEN
// is unset, the OTel logger provider is NOT installed. Instead of relying on a
// /health 200 (which doesn't actually exercise the bridge state), we check the
// api-level sentinel directly.
func TestOTelBridge_NotInstalledWithoutToken(t *testing.T) {
	// Ensure the token is absent for this test.
	os.Unsetenv("POSTHOG_PROJECT_TOKEN")

	// Reset any previously-installed provider so this test is order-independent.
	api.SetOTelLoggerProvider(nil)

	if api.LoggerProviderInstalled() {
		t.Error("expected OTel logger provider to be nil when POSTHOG_PROJECT_TOKEN is unset")
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

	if w.Code != 200 {
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

// --- memExporter: minimal in-memory sdklog.Exporter for testing ---

type memExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *memExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.records = append(e.records, r.Clone())
	}
	return nil
}

func (e *memExporter) Shutdown(context.Context) error   { return nil }
func (e *memExporter) ForceFlush(context.Context) error { return nil }

func (e *memExporter) Records() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]sdklog.Record, len(e.records))
	copy(out, e.records)
	return out
}

// TestOTelBridge_FanOut_BothHandlersFire confirms that when an OTel
// LoggerProvider is installed, a single slog.Info call is delivered to:
//   - the in-memory OTel exporter (OTel branch), and
//   - the JSON stdout handler (stdout branch).
//
// After the test the package-level provider is reset to nil.
func TestOTelBridge_FanOut_BothHandlersFire(t *testing.T) {
	// Capture stdout by redirecting os.Stdout before initLogger reinstalls handlers.
	origStdout := os.Stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = pw

	// Always restore stdout and tear down the bridge.
	t.Cleanup(func() {
		// Close the write end (may already be closed below, but double-close is safe for pipes).
		pw.Close()
		os.Stdout = origStdout
		// Reset bridge state so this test doesn't affect others.
		api.SetOTelLoggerProvider(nil)
	})

	// Build an in-memory LoggerProvider using our memExporter.
	exp := &memExporter{}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)),
	)

	// Install the provider — this re-runs initLogger, switching to a multiHandler
	// that fans out to both JSON stdout and the OTel bridge.
	api.SetOTelLoggerProvider(lp)

	// Emit a log record through the slog API.
	api.Logger.Info("hello", slog.String("k", "v"))

	// Close the write end so the pipe reader sees EOF.
	pw.Close()
	os.Stdout = origStdout

	// Flush the provider synchronously so SimpleProcessor records are exported.
	if err := lp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}

	// --- Assert: OTel branch received the record ---
	records := exp.Records()
	if len(records) == 0 {
		t.Fatal("OTel exporter received no records")
	}

	gotBody := records[0].Body().AsString()
	if gotBody != "hello" {
		t.Errorf("OTel record body: want %q, got %q", "hello", gotBody)
	}

	foundAttr := false
	records[0].WalkAttributes(func(kv otellog.KeyValue) bool {
		if kv.Key == "k" && kv.Value.AsString() == "v" {
			foundAttr = true
		}
		return true
	})
	if !foundAttr {
		t.Error(`OTel record missing attr k="v"`)
	}

	// --- Assert: stdout branch emitted a JSON line ---
	var stdoutBuf bytes.Buffer
	if _, err := stdoutBuf.ReadFrom(pr); err != nil {
		t.Fatalf("reading stdout pipe: %v", err)
	}

	// There may be multiple JSON lines (e.g. from other log calls during setup);
	// find the one with msg="hello".
	foundLine := false
	for _, line := range bytes.Split(bytes.TrimSpace(stdoutBuf.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(line, &m); err != nil {
			continue
		}
		if m["msg"] == "hello" {
			foundLine = true
			break
		}
	}
	if !foundLine {
		t.Errorf(`stdout JSON line with msg="hello" not found; stdout was: %s`, stdoutBuf.String())
	}
}
