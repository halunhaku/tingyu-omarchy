//go:build darwin

package library

import (
	"fmt"
	"os"
	"path/filepath"
)

// userDataDir is the macOS per-user application support directory the client
// owns since M7.
func userDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("library: home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", bundleID), nil
}
