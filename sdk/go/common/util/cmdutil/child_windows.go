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

package cmdutil

import (
	"fmt"
	"os"
	"os/exec"

	multierror "github.com/hashicorp/go-multierror"
	ps "github.com/mitchellh/go-ps"
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
// This is the Windows-specific implementation that scans all system
// processes using native syscalls (via go-ps) and attempts to kill
// them, aggregating any errors it encounters.
func KillChildren(pid int) error {
	procs, err := ps.Processes()
	if err != nil {
		return err
	}

	descendants := filterDescendants(pid, procs)

	// Try to kill the descendants and collect errors by PID.
	// These errors may not be relevant if the descendant
	// terminates in some other way.
	errors := map[int]error{}
	for _, proc := range descendants {
		procPid := proc.Pid()
		toKill, err := os.FindProcess(procPid)
		if err != nil {
			errors[procPid] = fmt.Errorf("FindProcess(%d) failed: %w", procPid, err)
		}

		err = toKill.Kill()
		if err != nil {
			errors[procPid] = fmt.Errorf("proc.Kill() failed for pid=%d: %w", procPid, err)
		}
	}

	survivingProcesses, err := ps.Processes()
	if err != nil {
		return err
	}

	survivingPids := map[int]bool{}
	for _, p := range survivingProcesses {
		survivingPids[p.Pid()] = true
	}

	// Only report errors for descendants that survived our
	// attempt to kill them.
	//
	// There are races inherent in sending a `Kill()` and
	// observing the process in the `survivingPids`. This method
	// would rather return a `nil` error when unsure than error
	// out incorrectly.
	var result error

	for _, proc := range descendants {
		_, surviving := survivingPids[proc.Pid()]
		if surviving {
			err, gotErr := errors[proc.Pid()]
			if gotErr {
				result = multierror.Append(result, err)
			}
		}
	}
	return result
}

func filterDescendants(rootPid int, processes []ps.Process) []ps.Process {
	parents := map[int]int{}
	for _, p := range processes {
		pid := p.Pid()
		ppid := p.PPid()
		// Can have PID=0 with PPID=0, ignore it.
		if ppid != pid {
			parents[pid] = ppid
		}
	}
	filtered := []ps.Process{}
	for _, p := range processes {
		if isDescendant(p.Pid(), rootPid, parents) || p.Pid() == rootPid {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func isDescendant(descendant, ancestor int, parents map[int]int) bool {
	p := descendant
	for {
		pp, gotParent := parents[p]
		if !gotParent {
			return false
		}
		if pp == ancestor {
			return true
		}
		p = pp
	}
}

// RegisterProcessGroup does nothing on Windows.
func RegisterProcessGroup(cmd *exec.Cmd) {
	// nothing to do on Windows.
}
