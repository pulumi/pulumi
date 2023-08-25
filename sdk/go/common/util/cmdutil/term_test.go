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

	ps "github.com/mitchellh/go-ps"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lookPathOrSkip(t *testing.T, name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("Skipping test: %q not found: %v", name, err)
	}
	return path
}

func TestTerminate_gracefulShutdown(t *testing.T) {
	t.Parallel()

	t.Run("go", func(t *testing.T) {
		t.Parallel()

		goBin := lookPathOrSkip(t, "go")

		src := filepath.Join("testdata", "term_graceful.go")
		bin := filepath.Join(t.TempDir(), "main")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}

		require.NoError(t,
			exec.Command(goBin, "build", "-o", bin, src).Run(),
			"error building test program")

		cmd := exec.Command(bin)
		testTerminateGracefulShutdown(t, cmd)
	})

	t.Run("node", func(t *testing.T) {
		t.Parallel()

		nodeBin := lookPathOrSkip(t, "node")

		src := filepath.Join("testdata", "term_graceful.js")
		cmd := exec.Command(nodeBin, src)
		testTerminateGracefulShutdown(t, cmd)
	})

	t.Run("python", func(t *testing.T) {
		t.Parallel()

		pythonBin := lookPathOrSkip(t, "python")

		src := filepath.Join("testdata", "term_graceful.py")
		cmd := exec.Command(pythonBin, src)
		testTerminateGracefulShutdown(t, cmd)
	})
}

// testTerminateGracefulShutdown runs the given command
// and expects it to shutdown gracefully.
//
// The contract for the given command is:
//
//   - It MUST print something to stdout
//     when it is ready to receive signals.
//     This is used to synchronize with the child process.
//   - It MUST exit with a zero code if it receives a SIGINT.
//   - It MUST exit with a non-zero code
//     if the signal wasn't received within 3 seconds.
//   - It MAY print diagnostic messages to stderr.
func testTerminateGracefulShutdown(t *testing.T, cmd *exec.Cmd) {
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

		ok, err := TerminateProcessGroup(cmd.Process, 1*time.Second)
		assert.True(t, ok, "child process did not exit gracefully")
		assert.NoError(t, err, "error terminating child process")
	}()

	err := cmd.Wait()
	if isWaitAlreadyExited(err) {
		err = nil
	}
	assert.NoError(t, err, "child did not exit cleanly")
	<-done
}

func TestTerminate_forceKill(t *testing.T) {
	t.Parallel()

	t.Run("go", func(t *testing.T) {
		t.Parallel()

		goBin := lookPathOrSkip(t, "go")

		src := filepath.Join("testdata", "term_frozen.go")
		bin := filepath.Join(t.TempDir(), "main")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}

		require.NoError(t,
			exec.Command(goBin, "build", "-o", bin, src).Run(),
			"error building test program")

		cmd := exec.Command(bin)
		testTerminateForceKill(t, cmd)
	})

	t.Run("node", func(t *testing.T) {
		t.Parallel()

		nodeBin := lookPathOrSkip(t, "node")

		src := filepath.Join("testdata", "term_frozen.js")
		cmd := exec.Command(nodeBin, src)
		testTerminateForceKill(t, cmd)
	})

	t.Run("python", func(t *testing.T) {
		t.Parallel()

		pythonBin := lookPathOrSkip(t, "python")

		src := filepath.Join("testdata", "term_frozen.py")
		cmd := exec.Command(pythonBin, src)
		testTerminateForceKill(t, cmd)
	})
}

// testTerminateForceKill runs the given command
// and expects it to be force-killed.
//
// The contract for the given command is similar to
// testTerminateGracefulShutdown, except:
//
//   - It MUST freeze for at least 1 second after it receives a SIGINT.
//   - It MAY exit with a non-zero code if it receives a SIGINT.
func testTerminateForceKill(t *testing.T, cmd *exec.Cmd) {
	RegisterProcessGroup(cmd)

	var stdout lockedBuffer
	cmd.Stdout = io.MultiWriter(&stdout, iotest.LogWriterPrefixed(t, "child(stdout): "))
	cmd.Stderr = iotest.LogWriterPrefixed(t, "child(stderr): ")
	require.NoError(t, cmd.Start(), "error starting child process")

	pid := cmd.Process.Pid

	done := make(chan struct{})
	go func() {
		defer close(done)

		// Wait until the child process is ready to receive signals.
		for stdout.Len() == 0 {
			time.Sleep(10 * time.Millisecond)
		}

		ok, err := TerminateProcessGroup(cmd.Process, time.Millisecond)
		assert.False(t, ok, "child process should not exit gracefully")
		assert.NoError(t, err, "error terminating child process")
	}()

	// cmd.Wait() will fail if we kill the child process.
	// We can't rely on that to test if the process was SIGKILLed.
	// Instead, we check if the process is still alive.
	_ = cmd.Wait()
	<-done

	proc, err := ps.FindProcess(pid)
	assert.NoError(t, err, "error finding process")
	assert.Nil(t, proc, "child process should be dead")
}

func TestTerminateChildren_gracefulShutdown(t *testing.T) {
	t.Parallel()

	// We've already verified signal handling cross-language
	// so this test won't bother with other languages.

	goBin := lookPathOrSkip(t, "go")

	src := filepath.Join("testdata", "term_graceful_with_child.go")
	bin := filepath.Join(t.TempDir(), "main")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	require.NoError(t,
		exec.Command(goBin, "build", "-o", bin, src).Run(),
		"error building test program")

	cmd := exec.Command(bin)
	testTerminateGracefulShutdown(t, cmd)
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
