//go:build linux

package library

import (
	"fmt"
	"os"
	"path/filepath"
)

// userDataDir is the Linux per-user data directory,
// $XDG_DATA_HOME/com.halunhaku.tingyu (~/.local/share/com.halunhaku.tingyu
// when XDG_DATA_HOME is unset or not absolute, as the XDG base directory spec
// requires), which is where the spec puts application data and where backup
// tools look for it. TINGYU_DATA_DIR still wins.
func userDataDir() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("library: home directory: %w", err)
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, bundleID), nil
}
