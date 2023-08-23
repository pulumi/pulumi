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

package cmdutil

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerminateGraceful_go(t *testing.T) {
	t.Parallel()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	// Build a Go program that waits for SIGINT.
	src := filepath.Join("testdata", "term_wait.go")
	bin := filepath.Join(t.TempDir(), "main")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	buildOutput := iotest.LogWriterPrefixed(t, "go build: ")
	buildCmd := exec.Command(goBin, "build", "-o", bin, src)
	buildCmd.Stdout = buildOutput
	buildCmd.Stderr = buildOutput
	require.NoError(t, buildCmd.Run())

	cmd := exec.Command(bin)
	testTerminateGraceful(t, cmd)
}

func TestTerminateGraceful_node(t *testing.T) {
	t.Parallel()

	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	src := filepath.Join("testdata", "term_wait.js")
	cmd := exec.Command(nodeBin, src)
	testTerminateGraceful(t, cmd)
}

func TestTerminateGraceful_python(t *testing.T) {
	t.Parallel()

	pythonBin, err := exec.LookPath("python")
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	src := filepath.Join("testdata", "term_wait.py")
	cmd := exec.Command(pythonBin, src)
	testTerminateGraceful(t, cmd)
}

func testTerminateGraceful(t *testing.T, cmd *exec.Cmd) {
	RegisterProcessGroup(cmd)
	cmd.Stdout = iotest.LogWriterPrefixed(t, "child(stdout): ")
	cmd.Stderr = iotest.LogWriterPrefixed(t, "child(stderr): ")

	t.Log("Starting child process")
	require.NoError(t, cmd.Start())

	go func() {
		// TODO: instead of sleeping,
		// read stdout until we see "Waiting for SIGINT"
		time.Sleep(1 * time.Second)
		t.Log("Sending SIGINT")
		assert.NoError(t, TerminateProcess(cmd.Process, 5*time.Second))
	}()

	t.Log("Waiting for child process to exit")
	require.NoError(t, cmd.Wait())

	t.Log("Child process exited")
}
