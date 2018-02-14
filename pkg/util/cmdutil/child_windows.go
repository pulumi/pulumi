// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build windows

package cmdutil

import (
	"os"
	"os/exec"

	multierror "github.com/hashicorp/go-multierror"
	ps "github.com/mitchellh/go-ps"
)

// KillChildren calls os.Process.Kill() on every child process of `pid`'s, stoping after the first error (if any). It
// also only kills direct child process, not any children they may have. This function is only implemented on Windows.
func KillChildren(pid int) error {
	procs, err := ps.Processes()
	if err != nil {
		return err
	}

	var result error

	for _, proc := range procs {
		if proc.PPid() == pid {
			toKill, err := os.FindProcess(proc.Pid())
			if err != nil {
				// It's possible that the process has already exited, let's see if it still exists. Either way, we won't
				// try to kill it but we will add the original error from os.FindProcess() if we can't prove it doesn't
				// exits.
				exists, existsErr := processExistsWithParent(proc.Pid(), proc.PPid())
				if existsErr != nil || exists {
					result = multierror.Append(result, err)
				}
				continue
			}

			err = toKill.Kill()
			if err != nil {
				result = multierror.Append(result, err)
			}
		}
	}

	return result
}

func processExistsWithParent(pid int, ppid int) (bool, error) {
	procs, err := ps.Processes()
	if err != nil {
		return false, err
	}

	for _, proc := range procs {
		if proc.Pid() == pid {
			return proc.PPid() == ppid, nil
		}
	}

	return false, nil
}

// RegisterProcessGroup does nothing on Windows.
func RegisterProcessGroup(cmd *exec.Cmd) {
	// nothing to do on Windows.
}
