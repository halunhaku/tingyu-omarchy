//go:build linux

package tokens

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Read asks the desktop for its appearance: the Omarchy theme in effect and
// Hyprland's corner radius. It never fails. A machine without Omarchy, without
// a state directory or without Hyprland leaves that part unread, and the
// design keeps its own accent and corners, which is the behaviour every other
// platform has.
func Read() Desktop {
	d := readTheme()
	if rounding, ok := readRounding(); ok {
		d.Rounding, d.HasRounding = rounding, true
	}
	return d
}

// statePath is Omarchy's current-theme directory. Omarchy switches a theme by
// removing and recreating the whole directory, so this is re-read on every
// poll instead of watched: a file watcher on the old inode would go quiet
// after the first switch.
func statePath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "omarchy", "current")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "omarchy", "current")
}

// readTheme reads theme.name and the mode/accent of colors.toml. The file is
// Omarchy's, not the client's, so a missing or unreadable one is a reason to
// keep the design, never an error to report.
func readTheme() Desktop {
	dir := statePath()
	if dir == "" {
		return Desktop{}
	}
	name, err := os.ReadFile(filepath.Join(dir, "theme.name"))
	if err != nil {
		return Desktop{}
	}
	d := Desktop{Theme: strings.TrimSpace(string(name))}
	if d.Theme == "" {
		return Desktop{}
	}
	colors, err := os.ReadFile(filepath.Join(dir, "theme", "colors.toml"))
	if err != nil {
		return d
	}
	d.Mode, d.Accent, d.HasAccent = parseColors(colors)
	return d
}

// parseColors reads the two keys the client needs out of colors.toml. The file
// is one flat table of `key = "#rrggbb"` lines, so a line scanner is enough;
// pull in a TOML parser only if the shape ever grows.
func parseColors(data []byte) (mode string, accent ui.Color, hasAccent bool) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch strings.TrimSpace(key) {
		case "mode":
			if value == "dark" || value == "light" {
				mode = value
			}
		case "accent":
			if c, ok := hexColor(value); ok {
				accent, hasAccent = c, true
			}
		}
	}
	return mode, accent, hasAccent
}

// hexColor parses the "#rrggbb" a theme file writes. A malformed value keeps
// the design's accent instead of panicking a window over someone's theme.
func hexColor(s string) (ui.Color, bool) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) != 6 {
		return ui.Color{}, false
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return ui.Color{}, false
	}
	return ui.RGB(uint8(v>>16), uint8(v>>8), uint8(v)), true
}

// readRounding asks Hyprland for decoration:rounding. Only the integer in the
// JSON is trusted: the human-readable form is formatted prose, and a keyword
// the compositor does not know still answers with the current value.
func readRounding() (float32, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "hyprctl", "-j", "getoption", "decoration:rounding").Output()
	if err != nil {
		return 0, false
	}
	var option struct {
		Int int `json:"int"`
	}
	if err := json.Unmarshal(out, &option); err != nil || option.Int < 0 {
		return 0, false
	}
	return float32(option.Int), true
}
