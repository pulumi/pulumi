// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build !windows

package cmdutil

import (
	"os/exec"
	"syscall"
)

// KillChildren calls os.Process.Kill() on every child process of `pid`'s, stoping after the first error (if any). It
// also only kills direct child process, not any children they may have.
func KillChildren(pid int) error {
	// A subprocess that was launched after calling `RegisterProcessGroup` below will
	// belong to a process group whose ID is the same as the PID. Passing the negation
	// of our PID (same as the PGID) sends a SIGKILL to all processes in our group.
	return syscall.Kill(-pid, syscall.SIGKILL)
}

// RegisterProcessGroup informs the OS that it needs to call `setpgid` on this
// child process. When it comes time to kill this process, we'll kill all processes
// in the same process group.
func RegisterProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
