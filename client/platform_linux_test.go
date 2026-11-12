//go:build linux

package main

import (
	"os"
	"testing"
)

// The client must ask GTK for the portal file chooser: its own chooser opens
// partly off screen under Hyprland. The variable is set in an init, so the
// test binary (which runs the same inits) is enough to observe it.
func TestPortalFileChooserIsRequested(t *testing.T) {
	if got := os.Getenv("GTK_USE_PORTAL"); got != "1" {
		t.Fatalf("GTK_USE_PORTAL = %q, want 1", got)
	}
}
