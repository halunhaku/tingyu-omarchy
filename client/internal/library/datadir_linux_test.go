//go:build linux

package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The XDG base directory spec says a relative XDG_DATA_HOME is invalid and
// must be ignored: honouring it would put the library in whatever directory
// the process happens to start in, which looks like a lost library.
func TestUserDataDirIgnoresRelativeXDGDataHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relative/share")

	dir, err := userDataDir()
	if err != nil {
		t.Fatalf("userDataDir: %v", err)
	}
	want := filepath.Join(home, ".local", "share", bundleID)
	if dir != want {
		t.Fatalf("userDataDir = %q, want %q", dir, want)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("userDataDir returned a relative path %q", dir)
	}
}

func TestUserDataDirUsesAbsoluteXDGDataHome(t *testing.T) {
	base := filepath.Join(t.TempDir(), "data")
	t.Setenv("XDG_DATA_HOME", base)

	dir, err := userDataDir()
	if err != nil {
		t.Fatalf("userDataDir: %v", err)
	}
	if want := filepath.Join(base, bundleID); dir != want {
		t.Fatalf("userDataDir = %q, want %q", dir, want)
	}
}

// TINGYU_DATA_DIR wins over the platform default; this is the override every
// acceptance run uses to work on a copy of a library.
func TestDataDirOverrideWins(t *testing.T) {
	custom := t.TempDir()
	t.Setenv("TINGYU_DATA_DIR", custom)
	t.Setenv("XDG_DATA_HOME", strings.TrimSuffix(t.TempDir(), "/"))

	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	if dir != custom {
		t.Fatalf("DataDir = %q, want the TINGYU_DATA_DIR value %q", dir, custom)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Fatalf("override directory disappeared: %v", err)
	}
}
