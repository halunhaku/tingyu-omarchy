package model

import (
	"runtime"
	"strings"
)

// ShortcutLabel follows MyGo's ui.Cmd: Command on macOS, Control elsewhere.
func ShortcutLabel(mac string) string {
	if runtime.GOOS == "darwin" {
		return mac
	}
	return strings.NewReplacer(
		"⌘", "Ctrl+", "⇧", "Shift+", "⌥", "Alt+",
		"→", "Right", "←", "Left", "↑", "Up", "↓", "Down",
	).Replace(mac)
}
