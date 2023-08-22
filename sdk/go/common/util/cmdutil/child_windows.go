// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package cmdutil

import (
	"os"
	"os/exec"
	"syscall"

	multierror "github.com/hashicorp/go-multierror"
	ps "github.com/mitchellh/go-ps"
)

// killProcessGroup kills a process group by calling Process.Kill()
// (which calls TerminateProcess on Windows) on all processes in the group.
//
// This is different from [KillChildren] which only kills child processes.
//
// This is a helper function for TerminateProcessGroup;
// a Unix version with the same signature exists in child_unix.go.
func killProcessGroup(proc *os.Process) error {
	if err := KillChildren(proc.Pid); err != nil {
		return err
	}

	// Kill the root process since KillChildren only kills child processes.
	return proc.Kill()
}

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

// RegisterProcessGroup informs the OS that it should create a new process group
// rooted at the given process.
//
// When it comes time to kill this process,
// we'll kill all processes in the same process group.
func RegisterProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
