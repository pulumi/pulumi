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
	"context"
	"errors"
	"fmt"
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

func TestTerminate_gracefulShutdown(t *testing.T) {
	t.Parallel()

	// This test runs commands in a child process, signals them,
	// and expects them to shutdown gracefully.
	//
	// The contract for the child process is as follows:
	//
	//   - It MUST print something to stdout when it is ready to receive signals.
	//   - It MUST exit with a zero code if it receives a SIGINT.
	//   - It MUST exit with a non-zero code if the signal wasn't received within 3 seconds.
	//   - It MAY print diagnostic messages to stderr.

	tests := []struct {
		desc string
		prog testProgram
	}{
		{desc: "go", prog: goTestProgram.From("graceful.go")},
		{desc: "node", prog: nodeTestProgram.From("graceful.js")},
		{desc: "python", prog: pythonTestProgram.From("graceful.py")},
		{desc: "with child", prog: goTestProgram.From("graceful_with_child.go")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			cmd := tt.prog.Build(t)

			var stdout lockedBuffer
			cmd.Stdout = io.MultiWriter(&stdout, iotest.LogWriterPrefixed(t, "stdout: "))
			cmd.Stderr = iotest.LogWriterPrefixed(t, "stderr: ")
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
		})
	}
}

func TestTerminate_gracefulShutdown_exitError(t *testing.T) {
	t.Parallel()

	// This test runs commands in a child process, signals them,
	// and expects them to shutdown gracefully
	// but with a non-zero exit code.

	cmd := goTestProgram.From("graceful.go").Args("-exit-code", "1").Build(t)

	var stdout lockedBuffer
	cmd.Stdout = io.MultiWriter(&stdout, iotest.LogWriterPrefixed(t, "stdout: "))
	cmd.Stderr = iotest.LogWriterPrefixed(t, "stderr: ")
	require.NoError(t, cmd.Start(), "error starting child process")

	// Wait until the child process is ready to receive signals.
	for stdout.Len() == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	ok, err := TerminateProcessGroup(cmd.Process, 1*time.Second)
	assert.True(t, ok, "child process did not exit gracefully")
	require.Error(t, err, "child process must exit with non-zero code")

	var exitErr *exec.ExitError
	if assert.ErrorAs(t, err, &exitErr, "expected ExitError from child process") {
		assert.Equal(t, 1, exitErr.ExitCode(), "unexpected exit code from child process")
	}
}

func TestTerminate_forceKill(t *testing.T) {
	t.Parallel()

	// This test runs commands in a child process, signals them,
	// and expects them to not exit in a timely manner.
	//
	// The contract for the child process is the same as gracefulShutdown,
	// except:
	//
	//   - It MUST freeze for at least 1 second after it receives a SIGINT.
	//   - It MAY exit with a non-zero code if it receives a SIGINT.

	tests := []struct {
		desc string
		prog testProgram
	}{
		{desc: "go", prog: goTestProgram.From("frozen.go")},
		{desc: "node", prog: nodeTestProgram.From("frozen.js")},
		{desc: "python", prog: pythonTestProgram.From("frozen.py")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			cmd := tt.prog.Build(t)

			var stdout lockedBuffer
			cmd.Stdout = io.MultiWriter(&stdout, iotest.LogWriterPrefixed(t, "stdout: "))
			cmd.Stderr = iotest.LogWriterPrefixed(t, "stderr: ")
			require.NoError(t, cmd.Start(), "error starting child process")

			// Wait until the child process is ready to receive signals.
			for stdout.Len() == 0 {
				time.Sleep(10 * time.Millisecond)
			}

			pid := cmd.Process.Pid
			done := make(chan struct{})
			go func() {
				defer close(done)

				ok, err := TerminateProcessGroup(cmd.Process, 50*time.Millisecond)
				assert.False(t, ok, "child process should not exit gracefully")
				assert.NoError(t, err, "error terminating child process")
			}()

			select {
			case <-done:
				// continue

			case <-time.After(200 * time.Millisecond):
				// If the process is not killed,
				// cmd.Wait() will block until it exits.
				t.Fatal("Took too long to kill child process")
			}

			assert.NoError(t,
				waitPidDead(pid, 100*time.Millisecond),
				"error waiting for process to die")
		})
	}
}

func TestTerminate_forceKill_processGroup(t *testing.T) {
	t.Parallel()

	// This is a variant of TestTerminate_forceKill
	// that verifies that a child process of the test process
	// is also killed.

	cmd := goTestProgram.From("frozen_with_child.go").Build(t)

	var stdout lockedBuffer
	cmd.Stdout = io.MultiWriter(&stdout, iotest.LogWriterPrefixed(t, "stdout: "))
	cmd.Stderr = iotest.LogWriterPrefixed(t, "stderr: ")
	require.NoError(t, cmd.Start(), "error starting child process")

	// Wait until the child process is ready to receive signals.
	for stdout.Len() == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	pid := cmd.Process.Pid
	childPid := -1

	procs, err := ps.Processes()
	require.NoError(t, err, "error listing processes")
	for _, proc := range procs {
		if proc.PPid() == pid {
			childPid = proc.Pid()
			break
		}
	}
	require.NotEqual(t, -1, childPid, "child process not found")

	done := make(chan struct{})
	go func() {
		defer close(done)

		ok, err := TerminateProcessGroup(cmd.Process, time.Millisecond)
		assert.False(t, ok, "child process should not exit gracefully")
		assert.NoError(t, err, "error terminating child process")
	}()

	select {
	case <-done:
		// continue

	case <-time.After(100 * time.Millisecond):
		// If the child process is not killed,
		// cmd.Wait() will block until it exits.
		t.Fatal("Took too long to kill child process")
	}

	for _, pid := range []int{pid, childPid} {
		assert.NoError(t,
			waitPidDead(pid, 100*time.Millisecond),
			"error waiting for process to die")
	}
}

func TestTerminate_unhandledInterrupt(t *testing.T) {
	t.Parallel()

	// This test runs programs that do not have an interrupt handler.
	// Contract for child process:
	//
	// - It MUST print to stdout when it's ready.
	// - It MUST exit with a non-zero code if it does not get terminated within 3 seconds.

	tests := []struct {
		desc string
		prog testProgram
	}{
		{desc: "go", prog: goTestProgram.From("unhandled.go")},
		{desc: "node", prog: nodeTestProgram.From("unhandled.js")},
		{desc: "python", prog: pythonTestProgram.From("unhandled.py")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			cmd := tt.prog.Build(t)

			var stdout lockedBuffer
			cmd.Stdout = io.MultiWriter(&stdout, iotest.LogWriterPrefixed(t, "stdout: "))
			cmd.Stderr = iotest.LogWriterPrefixed(t, "stderr: ")
			require.NoError(t, cmd.Start(), "error starting child process")

			// Wait until the child process is ready to receive signals.
			for stdout.Len() == 0 {
				time.Sleep(10 * time.Millisecond)
			}

			pid := cmd.Process.Pid
			done := make(chan struct{})
			go func() {
				defer close(done)

				ok, err := TerminateProcessGroup(cmd.Process, 400*time.Millisecond)
				assert.True(t, ok, "child process did not exit gracefully")
				assert.Error(t, err, "child process should have exited with an error")
			}()

			select {
			case <-done:
				// continue

			case <-time.After(500 * time.Millisecond):
				// Took too long to kill the child process.
				t.Fatal("Took too long to kill child process")
			}

			assert.NoError(t,
				waitPidDead(pid, 100*time.Millisecond),
				"error waiting for process to die")
		})
	}
}

type testProgramKind int

const (
	goTestProgram testProgramKind = iota
	nodeTestProgram
	pythonTestProgram
)

func (k testProgramKind) String() string {
	switch k {
	case goTestProgram:
		return "go"
	case nodeTestProgram:
		return "node"
	case pythonTestProgram:
		return "python"
	default:
		return fmt.Sprintf("testProgramKind(%d)", int(k))
	}
}

// From builds a testProgram of this kind
// with the given source file.
//
// Usage:
//
//	goTestProgram.From("main.go")
func (k testProgramKind) From(path string) testProgram {
	return testProgram{
		kind: k,
		src:  path,
	}
}

// testProgram is a test program inside the testdata directory.
type testProgram struct {
	// kind is the kind of test program.
	kind testProgramKind

	// src is the path to the source file
	// relative to the testdata directory.
	src string

	// args specifies additional arguments to pass to the program.
	args []string
}

func (p testProgram) Args(args ...string) testProgram {
	p.args = args
	return p
}

// Build builds an exec.Cmd for the test program.
// It skips the test if the program runner is not found.
func (p testProgram) Build(t *testing.T) (cmd *exec.Cmd) {
	t.Helper()

	defer func() {
		// Make sure that the returned command
		// is part of the process group.
		if cmd != nil {
			RegisterProcessGroup(cmd)
		}
	}()

	src := filepath.Join("testdata", p.src)
	switch p.kind {
	case goTestProgram:
		goBin := lookPathOrSkip(t, "go")
		bin := filepath.Join(t.TempDir(), "main")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}

		buildCmd := exec.Command(goBin, "build", "-o", bin, src)
		buildOutput := iotest.LogWriterPrefixed(t, "build: ")
		buildCmd.Stdout = buildOutput
		buildCmd.Stderr = buildOutput
		require.NoError(t, buildCmd.Run(), "error building test program")

		return exec.Command(bin, p.args...)

	case nodeTestProgram:
		nodeBin := lookPathOrSkip(t, "node")
		return exec.Command(nodeBin, append([]string{src}, p.args...)...)

	case pythonTestProgram:
		pythonCmds := []string{"python3", "python"}
		if runtime.GOOS == "windows" {
			pythonCmds = []string{"python", "python3"}
		}
		pythonBin := ""
		for _, bin := range pythonCmds {
			bin, err := exec.LookPath(bin)
			if err == nil {
				pythonBin = bin
				break
			}
		}
		if pythonBin == "" {
			t.Skipf("Skipping test: could not find python3 or python executable")
			return nil
		}
		return exec.Command(pythonBin, append([]string{src}, p.args...)...) //nolint:gosec

	default:
		t.Fatalf("unknown test program kind: %v", p.kind)
		return nil
	}
}

func lookPathOrSkip(t *testing.T, name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("Skipping test: %q not found: %v", name, err)
	}
	return path
}

// lockedBuffer is a thread-safe bytes.Buffer
// that can be used to capture stdout/stderr of a command.
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

// Waits until the process with the given pid doesn't exist anymore
// or the given timeout has elapsed.
//
// Returns an error if the timeout has elapsed.
func waitPidDead(pid int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var (
		proc ps.Process
		err  error
	)
	for {
		select {
		case <-ctx.Done():
			var errs []error
			if proc != nil {
				errs = append(errs, fmt.Errorf("process %d still exists: %v", pid, proc))
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("find process: %w", err))
			}

			return fmt.Errorf("waitPidDead %v: %w", pid, errors.Join(errs...))

		default:
			proc, err = ps.FindProcess(pid)
			if err == nil && proc == nil {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
