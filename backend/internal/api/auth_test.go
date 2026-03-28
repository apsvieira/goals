package api

import (
	"testing"
)

func TestMobileRedirectScheme(t *testing.T) {
	// This scheme MUST match the Android manifest and Capacitor config.
	if MobileRedirectScheme != "tinytracker" {
		t.Errorf("MobileRedirectScheme = %q, want %q", MobileRedirectScheme, "tinytracker")
	}
}
