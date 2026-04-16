//go:build linux && desktop

package main

import (
	"os/exec"
	"syscall"
)

func setDetachedProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
