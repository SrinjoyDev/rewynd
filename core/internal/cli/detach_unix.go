//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

// detachSysProcAttr puts the spawned core in its own session so it outlives `rewynd run`.
func detachSysProcAttr(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
