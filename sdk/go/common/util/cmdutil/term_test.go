// Copyright 2016-2023, Pulumi Corporation.
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
	"bytes"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerminateProcessGracefulShutdown(t *testing.T) {
	t.Parallel()

	lookPathOrSkip := func(name string) string {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("Skipping test: %q not found: %v", name, err)
		}
		return path
	}

	t.Run("go", func(t *testing.T) {
		t.Parallel()

		goBin := lookPathOrSkip("go")

		src := filepath.Join("testdata", "term_graceful.go")
		bin := filepath.Join(t.TempDir(), "main")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}

		require.NoError(t,
			exec.Command(goBin, "build", "-o", bin, src).Run(),
			"error building test program")

		cmd := exec.Command(bin)
		testTerminateProcessGracefulShutdown(t, cmd)
	})

	t.Run("node", func(t *testing.T) {
		t.Parallel()

		nodeBin := lookPathOrSkip("node")

		src := filepath.Join("testdata", "term_graceful.js")
		cmd := exec.Command(nodeBin, src)
		testTerminateProcessGracefulShutdown(t, cmd)
	})

	t.Run("python", func(t *testing.T) {
		t.Parallel()

		pythonBin := lookPathOrSkip("python")

		src := filepath.Join("testdata", "term_graceful.py")
		cmd := exec.Command(pythonBin, src)
		testTerminateProcessGracefulShutdown(t, cmd)
	})
}

// testTerminateProcessGracefulShutdown runs the given command
// and expects it to shutdown gracefully.
//
// The contract for the given command is:
//
//   - It MUST print something to stdout
//     when it is ready to receive signals.
//     This is used to synchronize with the child process.
//   - It MUST exit with a zero code if it receives a SIGINT.
//   - It MUST exit with a non-zero code
//     if the signal wasn't received within a reasonable time.
//   - It MAY print diagnostic messages to stderr.
func testTerminateProcessGracefulShutdown(t *testing.T, cmd *exec.Cmd) {
	RegisterProcessGroup(cmd)

	var stdout lockedBuffer
	cmd.Stdout = io.MultiWriter(&stdout, iotest.LogWriterPrefixed(t, "child(stdout): "))
	cmd.Stderr = iotest.LogWriterPrefixed(t, "child(stderr): ")
	require.NoError(t, cmd.Start(), "error starting child process")

	done := make(chan struct{})
	go func() {
		defer close(done)

		// Wait until the child process is ready to receive signals.
		for stdout.Len() == 0 {
			time.Sleep(10 * time.Millisecond)
		}

		assert.NoError(t, TerminateProcess(cmd.Process, 1*time.Second),
			"error terminating child process")
	}()

	assert.NoError(t, cmd.Wait(), "child did not exit cleanly")
	<-done
}

type lockedBuffer struct {
	mu sync.RWMutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.b.Len()
}
