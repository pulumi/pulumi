// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build windows

package cmdutil

import (
	"os"

	ps "github.com/mitchellh/go-ps"
)

// KillChildren calls os.Process.Kill() on every child process of `pid`'s, stoping after the first error (if any). It also only kills
// direct child process, not any children they may have. This function is only implemented on Windows.
func KillChildren(pid int) error {
	procs, err := ps.Processes()
	if err != nil {
		return err
	}

	for _, proc := range procs {
		if proc.PPid() == pid {
			toKill, err := os.FindProcess(proc.Pid())
			if err != nil {
				return err
			}

			err = toKill.Kill()
			if err != nil {
				return err
			}
		}
	}

	return nil
}
