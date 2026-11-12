//go:build linux

package app

import (
	"net/url"
	"os/exec"
	"path/filepath"
)

// revealPath shows a file in the desktop's file manager. The freedesktop
// FileManager1 interface selects the row itself and is what the file managers
// on Omarchy (Dolphin, Nautilus, Nemo, Thunar) implement; when no one owns the
// name, xdg-open on the folder at least puts the user in front of the file.
// Both are fire-and-forget, so a file manager that exits immediately is not an
// error here.
func revealPath(path string) error {
	target := (&url.URL{Scheme: "file", Path: path}).String()
	err := exec.Command("dbus-send",
		"--session",
		"--dest=org.freedesktop.FileManager1",
		"--type=method_call",
		"/org/freedesktop/FileManager1",
		"org.freedesktop.FileManager1.ShowItems",
		"array:string:"+target,
		"string:",
	).Start()
	if err == nil {
		return nil
	}
	return exec.Command("xdg-open", filepath.Dir(path)).Start()
}
