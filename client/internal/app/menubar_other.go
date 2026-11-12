//go:build !linux

package app

// menuBarAutoHides keeps the menu bar where the platform puts it: in the
// system menu bar on macOS, in the window on Windows.
func menuBarAutoHides() bool { return false }
