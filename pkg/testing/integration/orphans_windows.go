// Copyright 2022, Pulumi Corporation.
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

// Pulumi may currently leave orphan sub-processes in memory, which
// may have been contributing to OOM issues on CI. This code
// compensates by killing off orphan processes after a test explicitly.
//
// TODO[pulumi/pulumi#8696] - remove once fixed.

package integration

import (
	"os"
	"os/exec"
	"testing"

	ps "github.com/mitchellh/go-ps"
)

type orphanProcessCleaner struct {
	cmd *exec.Cmd
}

func newOrphanProcessCleaner(cmd *exec.Cmd) *orphanProcessCleaner {
	return &orphanProcessCleaner{cmd: cmd}
}

func (o *orphanProcessCleaner) killOrphanProcesses(t *testing.T) {
	proc := o.cmd.Process

	procs, err := ps.Processes()
	if err != nil {
		t.Logf("Ignoring failure to list OS processes to kill orphans: %v", err)
		return
	}

	parent := map[int]int{}
	for _, p := range procs {
		pid := p.Pid()
		ppid := p.PPid()
		if ppid != pid {
			parent[pid] = ppid
		}
	}

	isDescendant := func(p int) bool {
		for {
			pp, gotParent := parent[p]
			if !gotParent {
				return false
			}
			if pp == proc.Pid {
				return true
			}
			p = pp
		}
	}

	for pid := range parent {
		d := isDescendant(pid)
		if d {
			p, err := os.FindProcess(pid)
			if err != nil {
				t.Logf("Ignoring os.FindProcess(%d) failure: %v", pid, err)
				continue
			}

			err = p.Kill()
			if err != nil {
				t.Logf("WARN failed to kill orphan subproces with pid=%d: %v", pid, err)
				continue
			}
			t.Logf("WARN killed orphan subproces with pid=%d", pid)
		}
	}
}
