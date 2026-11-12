//go:build !linux

package mpv

// systemLibraryPath has no fallback off Linux: macOS resolves the app bundle
// and the pub cache, Windows the DLLs next to the executable. It reports no
// error so those platforms keep their existing "no libmpv" message.
func systemLibraryPath() (string, error) { return "", nil }

// acceptSoname is always false off Linux: every path goes through
// validateLibrary, so a stray "libmpv.so.2" in the working directory cannot be
// picked up on macOS.
func acceptSoname(string) bool { return false }
