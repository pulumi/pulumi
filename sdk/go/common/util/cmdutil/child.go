// Copyright 2016-2022, Pulumi Corporation.
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
//go:build !windows
// +build !windows

package cmdutil

import (
	"os/exec"
	"syscall"
)

// `KillChildren` kills the root process respresented by `pid` process
// identifier, as well as any child or descendant processes it
// detects. It ignores errors if it appears that the processes are no
// longer live.
//
// `KillChildren` is Intended to be used with `RegisterProcessGroup`
// to make sure misbehaving commands do not leave any orphan
// sub-processes running:
//
//	cmd := exec.Command(name, args...)
//	cmdutil.RegisterProcessGroup(cmd)
//      cmd.Start() // or any other method that actually starts the process
//      err := cmdutil.KillChildren(cmd.Process.Pid)
//
// This is the UNIX-specific implementation that sends a SIGKILL to
// the process group associated with the `pid`.
func KillChildren(pid int) error {
	// A subprocess that was launched after calling `RegisterProcessGroup` below will
	// belong to a process group whose ID is the same as the PID. Passing the negation
	// of our PID (same as the PGID) sends a SIGKILL to all processes in our group.
	//
	// Relevant documentation: https://linux.die.net/man/2/kill
	// "If pid is less than -1, then sig is sent to every process in the
	// process group whose ID is -pid. "
	return syscall.Kill(-pid, syscall.SIGKILL)
}

// `RegisterProcessGroup` informs the OS that it needs to call
// `setpgid` (process group ID) on the process that the command `cmd`
// will be starting. When this process later starts, the OS will
// allocate a new PID and will assign its PGID=PID. All children and
// descendants of this process will then inherit the PGID value.
//
// Intended to be used with `KillChildren`.
//
// Usage:
//
//	cmd := exec.Command(name, args...)
//	cmdutil.RegisterProcessGroup(cmd)
//      cmd.Start()
//
func RegisterProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
