//go:build linux

package app

// windowCloseQuits reports whether closing the last window ends the process.
// On Linux the window is the app, as on Windows: closing it quits, which also
// stops the engine and releases the library instead of leaving a windowless
// process for a launcher that cannot get its window back.
func windowCloseQuits() bool { return true }
