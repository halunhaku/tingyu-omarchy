//go:build windows

package mpv

import (
	"debug/pe"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

// openLibrary loads a DLL by absolute path with LoadLibraryEx and
// LOAD_WITH_ALTERED_SEARCH_PATH, so libmpv-2.dll resolves its own neighbours
// (FFmpeg DLLs, should a build ship them) from its directory instead of the
// process's working directory.
func openLibrary(path string) (uintptr, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	const loadWithAlteredSearchPath = 0x00000008
	kernel32, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		return 0, err
	}
	proc, err := syscall.GetProcAddress(kernel32, "LoadLibraryExW")
	if err != nil {
		return 0, err
	}
	handle, _, callErr := syscall.SyscallN(proc, uintptr(unsafe.Pointer(name)), 0, loadWithAlteredSearchPath)
	runtime.KeepAlive(name)
	if handle == 0 {
		return 0, callErr
	}
	return handle, nil
}

// closeLibrary releases the handle from openLibrary.
func closeLibrary(handle uintptr) error {
	return syscall.FreeLibrary(syscall.Handle(handle))
}

// validateLibrary rejects anything the loader would otherwise figure out
// silently: a relative path, a missing file, a directory, a DLL built for
// another architecture. It returns the absolute path that is actually loaded.
func validateLibrary(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("mpv: resolve %s: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("mpv: libmpv DLL not found at %s: %w", abs, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("mpv: %s is not a regular file", abs)
	}
	f, err := pe.Open(abs)
	if err != nil {
		return "", fmt.Errorf("mpv: %s is not a usable PE image: %w", abs, err)
	}
	defer f.Close()
	want := map[string]uint16{
		"amd64": pe.IMAGE_FILE_MACHINE_AMD64,
		"arm64": pe.IMAGE_FILE_MACHINE_ARM64,
		"386":   pe.IMAGE_FILE_MACHINE_I386,
	}[runtime.GOARCH]
	if want != 0 && f.Machine != want {
		return "", fmt.Errorf("mpv: %s is built for PE machine %#x, this process is %s (machine %#x)",
			abs, f.Machine, runtime.GOARCH, want)
	}
	return abs, nil
}

// LibraryPath resolves libmpv on Windows: MPV_LIB (resolved to an absolute
// path; an explicit opt-in), then libmpv-2.dll next to the executable, then
// libmpv-2.dll in mpv\ next to the executable. It never falls back to the
// working directory or to PATH, so a DLL dropped into a random directory
// cannot be picked up silently.
func LibraryPath() string {
	if path := os.Getenv("MPV_LIB"); path != "" {
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
		return path
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(dir, "libmpv-2.dll"),
		filepath.Join(dir, "mpv", "libmpv-2.dll"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}
