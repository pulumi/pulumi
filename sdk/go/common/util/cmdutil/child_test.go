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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	ps "github.com/mitchellh/go-ps"

	"github.com/stretchr/testify/require"
)

func TestKillChildren(t *testing.T) {
	d := t.TempDir()

	exe := "processtree"
	if runtime.GOOS == "windows" {
		exe = "processtree.exe"
	}
	exe = filepath.Join(d, exe)

	gocmd := exec.Command("go", "build", "-o", exe)
	gocmd.Dir = "testdata"
	err := gocmd.Run()
	require.NoError(t, err)

	cmd := exec.Command(exe, "-depth", "3")
	RegisterProcessGroup(cmd)

	err = cmd.Start()
	require.NoError(t, err)

	// Give subprocess time to spawn children.
	time.Sleep(1 * time.Second)

	err = KillChildren(cmd.Process.Pid)
	require.NoError(t, err)

	// Give SIGKILL time to propagate.
	time.Sleep(1 * time.Second)

	procs, err := ps.Processes()
	require.NoError(t, err)

	for _, p := range procs {
		if strings.Contains(p.Executable(), "processtree") {
			t.Errorf("Runaway process: %s pid=%d", p.Executable(), p.Pid())
		}
	}
}
