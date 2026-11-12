//go:build linux

package app

// menuBarAutoHides reports whether the window keeps its menu bar out of sight
// until Alt or F10. On Linux the menu bar is drawn inside the window, and an
// in-window macOS-style menu bar is what Omarchy's own apps never show: they
// are driven from the keyboard. The menu stays reachable for anyone who looks
// for it, and the command palette (Ctrl+K) covers the same ground.
func menuBarAutoHides() bool { return true }
