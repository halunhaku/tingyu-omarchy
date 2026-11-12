//go:build linux

package app

import (
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
)

// revealPath shows a file in the desktop's file manager. The freedesktop
// FileManager1 interface selects the row itself and is what the file managers
// on Omarchy offer through D-Bus (Nautilus and Dolphin do; Thunar does not,
// which is why xdg-open is the fallback rather than a nicety). Both calls are
// fire-and-forget, so their processes are waited for in the background to
// keep the process table clean.
func revealPath(path string) error {
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	// dbus-send parses the array from one comma separated argument and has no
	// escaping, so a comma in the file name would split it into two URIs.
	uri = strings.ReplaceAll(uri, ",", "%2C")
	// --print-reply is what makes the call report a failure: without it
	// dbus-send exits 0 as soon as the message is on the bus, whatever the
	// file manager answers.
	cmd := exec.Command("dbus-send",
		"--session",
		"--print-reply",
		"--dest=org.freedesktop.FileManager1",
		"--type=method_call",
		"/org/freedesktop/FileManager1",
		"org.freedesktop.FileManager1.ShowItems",
		"array:string:"+uri,
		"string:",
	)
	if err := cmd.Run(); err == nil {
		return nil
	}
	fallback := exec.Command("xdg-open", filepath.Dir(path))
	if err := fallback.Start(); err != nil {
		return err
	}
	go func() { _ = fallback.Wait() }()
	return nil
}
