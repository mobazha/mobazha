//go:build windows && desktop

package main

import "os/exec"

func setDetachedProcAttr(cmd *exec.Cmd) {
	// Windows tray is built with -H windowsgui; no detach needed.
}
