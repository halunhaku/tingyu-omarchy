//go:build darwin || linux || freebsd || netbsd

package mpv

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ebitengine/purego"
)

// validateLibrary checks the path the same way the Windows build does: the
// loader only sees an absolute path to a regular file.
func validateLibrary(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("mpv: resolve %s: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("mpv: libmpv not found at %s: %w", abs, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("mpv: %s is not a regular file", abs)
	}
	return abs, nil
}

// closeLibrary releases the handle from openLibrary.
func closeLibrary(handle uintptr) error { return purego.Dlclose(handle) }

// openLibrary loads the shared library with dlopen. RTLD_LAZY matters on
// macOS: the bundled audio-only Mpv.framework has unresolved libass symbols,
// so RTLD_NOW fails.
func openLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
}

// ResolveLibrary resolves libmpv: MPV_LIB, then the app bundle's
// Contents/Frameworks, then (development only) the media_kit pub cache that
// ships the same LGPL build the Flutter client distributes, then the
// platform's system fallback (Linux: the libmpv SONAMEs, see
// libpath_linux.go). The error explains what was tried when the fallback is
// the only thing that failed; it is nil when MPV_LIB or a bundled copy won.
func ResolveLibrary() (string, error) {
	if path := os.Getenv("MPV_LIB"); path != "" {
		return path, nil
	}
	if exe, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exe), "..", "Frameworks", "Mpv.framework", "Versions", "A", "Mpv")
		if info, err := os.Stat(bundled); err == nil && !info.IsDir() {
			return bundled, nil
		}
	}
	if path := devLibraryPath(); path != "" {
		return path, nil
	}
	return systemLibraryPath()
}

// LibraryPath is ResolveLibrary for callers that only need the path to
// display. An empty result means no libmpv was found.
func LibraryPath() string {
	path, _ := ResolveLibrary()
	return path
}

// devLibraryPath finds the newest media_kit_libs_macos_audio in the pub cache.
// It is a development fallback: the binary still needs an LC_RPATH or the
// frameworks next to it (scripts/bundle.sh), and a signed binary ignores
// DYLD_FRAMEWORK_PATH.
func devLibraryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	pattern := filepath.Join(home, ".pub-cache", "hosted", "pub.dev", "media_kit_libs_macos_audio-*", "macos", "Frameworks", "Mpv.framework", "Versions", "A", "Mpv")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[len(matches)-1]
}
