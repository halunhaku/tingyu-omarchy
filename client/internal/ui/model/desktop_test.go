package model

import (
	"testing"

	"github.com/halunhaku/tingyu/client/internal/ui/tokens"
)

// TestSystemDarkFollowsOmarchy is the brightness half of M4: on a desktop that
// states its mode, 跟随系统 means Omarchy's mode. The explicit 浅色/深色 settings
// still win, a platform that says nothing keeps the window's theme, and
// 跟随桌面主题 off keeps the desktop out of the frame entirely.
func TestSystemDarkFollowsOmarchy(t *testing.T) {
	s := New(nil, nil, "", nil)
	if !s.Settings().FollowDesktop {
		t.Fatalf("跟随桌面主题 is off by default")
	}
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
	if !s.Appearance().Equal(s.Desktop()) {
		t.Errorf("the desktop reading does not reach the frame")
	}

	s.SetSettings(Settings{ThemeMode: "dark", Density: "comfortable", CoverTint: true, FollowDesktop: true})
	if !s.Settings().Dark(s.SystemDark(false)) {
		t.Errorf("an explicit 深色 did not overrule the desktop")
	}

	// The switch off: the frame is the design again, brightness included.
	off := s.Settings()
	off.FollowDesktop = false
	s.SetSettings(off)
	if got := s.Appearance(); !got.Equal(tokens.Desktop{}) {
		t.Errorf("the switch is off but the desktop still reaches the frame: %+v", got)
	}
	if s.SystemDark(false) {
		t.Errorf("the switch is off but the desktop's mode is still used")
	}

	// Back on; a theme file without a mode says nothing about brightness.
	on := s.Settings()
	on.FollowDesktop = true
	s.SetSettings(on)
	s.SetDesktop(tokens.Desktop{Theme: "tokyo-night"})
	if s.SystemDark(false) {
		t.Errorf("a theme without a mode claimed to be dark")
	}
}
