//go:build windows

package library

import (
	"fmt"
	"os"
	"path/filepath"
)

// userDataDir is the Windows per-user roaming application data directory,
// %AppData%\com.halunhaku.tingyu (Roaming, like Application Support on macOS,
// so a roaming profile keeps the library). TINGYU_DATA_DIR still wins.
func userDataDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("library: user config directory: %w", err)
	}
	return filepath.Join(dir, bundleID), nil
}
