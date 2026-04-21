package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"45s", 45 * time.Second, false},
		{"0d", 0, false},
		{"0h", 0, false},
		{"0s", 0, false},

		// Errors.
		{"", 0, true},
		{"d", 0, true},
		{"7", 0, true},      // missing unit
		{"7y", 0, true},     // unsupported unit
		{"1h30m", 0, true},  // mixed units not supported (time.ParseDuration syntax)
		{"-7d", 0, true},    // negative
		{"+7d", 0, true},    // leading sign
		{"seven_d", 0, true},
		{"7.5h", 0, true}, // non-integer

		// Overflow / out-of-range.
		{"999999999d", 0, true},              // exceeds 100-year cap, would wrap int64 after *24*time.Hour
		{"999999999999999999999h", 0, true}, // exceeds int64 before multiply — ParseInt rejects
	}
	for _, tc := range cases {
		got, err := parseDuration(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDuration(%q): expected error, got %v", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDuration(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDuration(%q): want %v, got %v", tc.in, tc.want, got)
		}
	}
}

func TestDescriptionSnippet(t *testing.T) {
	// Build a deterministic 60-char string and a 61-char string without relying
	// on repeating one rune (so we can eyeball results).
	sixty := strings.Repeat("a", 60)
	sixtyOne := sixty + "b"
	embeddedNL := "line one\nline two\nline three"

	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"empty", "", 60, ""},
		{"short", "hello", 60, "hello"},
		{"exactly-60", sixty, 60, sixty},
		{"61-truncated", sixtyOne, 60, sixty + "\u2026"},
		{"embedded-newlines", embeddedNL, 60, "line one line two line three"},
		{"trim-whitespace", "   hello   world   ", 60, "hello world"},
		{"newlines-truncated", strings.Repeat("a\n", 80), 10, strings.Repeat("a ", 5)[:10] + "\u2026"},
		{"cjk-rune-boundary", strings.Repeat("日", 70), 60, strings.Repeat("日", 60) + "\u2026"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := descriptionSnippet(tc.in, tc.max)
			if got != tc.want {
				t.Errorf("descriptionSnippet(%q, %d):\n  want %q\n  got  %q", tc.in, tc.max, tc.want, got)
			}
		})
	}
}

func TestFormatBreadcrumb(t *testing.T) {
	// 2026-04-14T10:00:00Z = 1775347200000 ms. Use that as our fixed anchor
	// so the line is stable across runs.
	ts := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC).UnixMilli()

	info := breadcrumb{TS: ts, Category: "log", Level: "info", Message: "app started"}
	warn := breadcrumb{TS: ts, Category: "sync", Level: "warn", Message: "retrying"}
	errB := breadcrumb{TS: ts, Category: "net", Level: "error", Message: "timeout"}

	t.Run("no-color-info", func(t *testing.T) {
		got := formatBreadcrumb(info, false)
		want := "2026-04-14T10:00:00Z  INFO   log      app started"
		if got != want {
			t.Errorf("info:\n  want %q\n  got  %q", want, got)
		}
	})

	t.Run("no-color-warn", func(t *testing.T) {
		got := formatBreadcrumb(warn, false)
		want := "2026-04-14T10:00:00Z  WARN   sync     retrying"
		if got != want {
			t.Errorf("warn:\n  want %q\n  got  %q", want, got)
		}
	})

	t.Run("color-info-is-gray", func(t *testing.T) {
		got := formatBreadcrumb(info, true)
		if !strings.HasPrefix(got, ansiGray) || !strings.HasSuffix(got, ansiReset) {
			t.Errorf("info with color should be gray-wrapped, got %q", got)
		}
	})

	t.Run("color-warn-is-yellow", func(t *testing.T) {
		got := formatBreadcrumb(warn, true)
		if !strings.HasPrefix(got, ansiYellow) {
			t.Errorf("warn with color should start with yellow, got %q", got)
		}
	})

	t.Run("color-error-is-red", func(t *testing.T) {
		got := formatBreadcrumb(errB, true)
		if !strings.HasPrefix(got, ansiRed) {
			t.Errorf("error with color should start with red, got %q", got)
		}
	})

	t.Run("unknown-level-falls-back-to-gray", func(t *testing.T) {
		weird := breadcrumb{TS: ts, Category: "x", Level: "debug", Message: "m"}
		got := formatBreadcrumb(weird, true)
		if !strings.HasPrefix(got, ansiGray) {
			t.Errorf("unknown level should default to gray, got %q", got)
		}
	})
}

func TestHasData(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"null", false},
		{"{}", false},
		{" {} ", false},
		{`{"k":"v"}`, true},
		{`[1,2,3]`, true},
	}
	for _, tc := range cases {
		got := hasData(json.RawMessage(tc.in))
		if got != tc.want {
			t.Errorf("hasData(%q): want %v, got %v", tc.in, tc.want, got)
		}
	}
}

func TestCompactJSON(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := compactJSON(nil); got != "(empty)" {
			t.Errorf("empty: got %q", got)
		}
	})
	t.Run("pretty-compacted", func(t *testing.T) {
		in := json.RawMessage("{\n  \"a\": 1,\n  \"b\": 2\n}")
		got := compactJSON(in)
		want := `{"a":1,"b":2}`
		if got != want {
			t.Errorf("want %q, got %q", want, got)
		}
	})
	t.Run("invalid-falls-through", func(t *testing.T) {
		in := json.RawMessage(`not-json`)
		got := compactJSON(in)
		if got != "not-json" {
			t.Errorf("invalid: want passthrough, got %q", got)
		}
	})
}
