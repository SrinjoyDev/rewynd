//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

// detachSysProcAttr starts the core detached (no console, new process group) so it outlives
// `rewynd run`.
func detachSysProcAttr(c *exec.Cmd) {
	const detachedProcess = 0x00000008
	const createNewProcessGroup = 0x00000200
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
}
