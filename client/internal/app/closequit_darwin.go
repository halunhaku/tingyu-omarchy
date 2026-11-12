//go:build darwin

package app

// windowCloseQuits reports whether closing the last window ends the process.
// On macOS it does not: playback continues and the Dock icon reopens the
// window, which is the platform's convention.
func windowCloseQuits() bool { return false }
