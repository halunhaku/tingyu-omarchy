//go:build windows

package app

import (
	"os/exec"
	"syscall"
)

// revealPath shows a file in Explorer with the row selected. explorer.exe
// returns a non-zero exit code even when it succeeds, so Start is enough and
// the exit status is not inspected.
func revealPath(path string) error {
	cmd := exec.Command("explorer.exe", "/select,"+path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
