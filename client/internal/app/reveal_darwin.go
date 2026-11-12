//go:build darwin

package app

import "os/exec"

// revealPath shows a file in the platform's file manager: Finder on macOS.
func revealPath(path string) error {
	return exec.Command("open", "-R", path).Start()
}
