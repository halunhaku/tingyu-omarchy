package model

import (
	"testing"

	"github.com/halunhaku/tingyu/client/internal/ui/tokens"
)

// TestSystemDarkFollowsOmarchy is the brightness half of M4: on a desktop that
// states its mode, 跟随系统 means Omarchy's mode. The explicit 浅色/深色 settings
// still win, and a platform that says nothing keeps the window's theme.
func TestSystemDarkFollowsOmarchy(t *testing.T) {
	s := New(nil, nil, "", nil)
	if !s.SystemDark(true) || s.SystemDark(false) {
		t.Errorf("without a desktop the window's theme is not used")
	}

	s.SetDesktop(tokens.Desktop{Theme: "catppuccin-latte", Mode: "light"})
	if s.SystemDark(true) {
		t.Errorf("Omarchy's light theme did not win over the window's dark theme")
	}
	if s.Settings().Dark(s.SystemDark(true)) {
		t.Errorf("跟随系统 ignored Omarchy's light theme")
	}

	s.SetSettings(Settings{ThemeMode: "dark", Density: "comfortable", CoverTint: true})
	if !s.Settings().Dark(s.SystemDark(false)) {
		t.Errorf("an explicit 深色 did not overrule the desktop")
	}

	// A theme file without a mode says nothing about brightness.
	s.SetDesktop(tokens.Desktop{Theme: "tokyo-night"})
	if s.SystemDark(false) {
		t.Errorf("a theme without a mode claimed to be dark")
	}
}
