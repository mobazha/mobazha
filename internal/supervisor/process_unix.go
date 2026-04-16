//go:build !windows

package supervisor

import "os/exec"

func setProcAttr(cmd *exec.Cmd) {
	// On Unix, no special attributes needed for the child process;
	// stdout/stderr are redirected to the log file.
}
