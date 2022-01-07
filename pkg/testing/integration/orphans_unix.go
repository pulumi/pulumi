//go:build !windows
// +build !windows

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
	"os/exec"
	"syscall"
	"testing"
)

type orphanProcessCleaner struct {
	cmd *exec.Cmd
}

func newOrphanProcessCleaner(cmd *exec.Cmd) *orphanProcessCleaner {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	return &orphanProcessCleaner{cmd: cmd}
}

func (o *orphanProcessCleaner) killOrphanProcesses(t *testing.T) {
	pid := o.cmd.Process.Pid
	err := syscall.Kill(-pid, syscall.SIGKILL)

	if err != nil {
		t.Logf("Ignoring failure to kill process group pgid=%d: %v", pid, err)
	}
}
