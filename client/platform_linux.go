//go:build linux

package main

import "os"

// GTK on Linux can either open its own GtkFileChooserDialog or ask the
// desktop's portal. Its own dialog is unusable on Hyprland: the compositor
// places it partly off screen, with the confirm button out of reach (see
// docs/plan/2026-11-12-omarchy-migration-scope.md §3 M3). The portal's
// chooser (xdg-desktop-portal-gtk here) opens at a sane size with both
// buttons visible, so the client asks for it before GTK starts.
//
// An explicit GTK_USE_PORTAL in the environment still wins: someone running
// without a portal (a bare session, a test harness) must be able to turn this
// off.
func init() {
	if os.Getenv("GTK_USE_PORTAL") == "" {
		os.Setenv("GTK_USE_PORTAL", "1")
	}
}
