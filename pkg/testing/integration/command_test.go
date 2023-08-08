// Copyright 2016-2013, Pulumi Corporation.
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

package integration

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockedWriter(t *testing.T) {
	t.Parallel()

	// We run our tests with '-race'
	// so to verify that lockedWriter is safe for concurrent use,
	// we just write to it from multiple goroutines.
	// The race-detector will complain if there's a problem.

	const (
		NumWorkers = 10
		NumWrites  = 1000
		Message    = "potatoes"
	)

	var buf bytes.Buffer
	w := &lockedWriter{W: &buf}

	// Two wait groups:
	// 'ready' makes sure that all workers are ready before we start,
	// increasing the chances of hitting a race condition.
	// 'done' makes sure that all workers are done before we check the results.
	var ready, done sync.WaitGroup
	ready.Add(NumWorkers)
	done.Add(NumWorkers)
	for i := 0; i < NumWorkers; i++ {
		go func() {
			defer done.Done()

			ready.Done() // I'm ready.
			ready.Wait() // Wait for everyone else to be ready.

			for j := 0; j < NumWrites; j++ {
				_, err := io.WriteString(w, Message)
				assert.NoError(t, err)
			}
		}()
	}
	done.Wait() // Wait for workers to finish.

	// We can't assert the exact contents of the buffer because
	// writes from different goroutines can interleave
	// between calls to Writer.Write.
	// We can, however, assert that the total length is correct.
	assert.Equal(t, NumWorkers*NumWrites*len(Message), buf.Len())
}

func TestRunCommand_verbose(t *testing.T) {
	t.Parallel()

	echo, err := exec.LookPath("echo")
	if err != nil {
		t.Skipf("Couldn't find echo on PATH: %v", err)
	}

	wd := t.TempDir()
	fakeT := fakeTestingT{TB: t}
	err = RunCommand(&fakeT, "echo", []string{echo, "hello", "world"}, wd, &ProgramTestOptions{
		Verbose: true,
	})
	require.NoError(t, err)

	logs := fakeT.logs.String()
	assert.Contains(t, logs, "echo hello world")
	assert.Contains(t, logs, "Logging to")
	assert.Contains(t, logs, "[echo] hello world\n")
}

func TestRunCommand_failure(t *testing.T) {
	t.Parallel()

	falseExe, err := exec.LookPath("false")
	if err != nil {
		t.Skipf("Couldn't find false on PATH: %v", err)
	}

	wd := t.TempDir()
	fakeT := fakeTestingT{TB: t}
	err = RunCommand(&fakeT, "false", []string{falseExe}, wd, nil /* opts */)
	assert.Error(t, err)

	logs := fakeT.logs.String()
	assert.Contains(t, logs, "false' failed")
}

func TestRunCommand_failureStdoutStderr(t *testing.T) {
	t.Parallel()

	// Verifies that the provided stdout, stderr are written to if the command fails
	// regardless of verbose mode.

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("Couln't find node on PATH: %v", err)
	}

	tests := []struct {
		name    string
		verbose bool
	}{
		{"verbose", true},
		{"not verbose", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			err = RunCommand(t,
				"node",
				[]string{node, "-e", "console.log('stdout'); console.error('stderr'); process.exit(1);"},
				t.TempDir(),
				&ProgramTestOptions{
					Verbose: tt.verbose,
					Stdout:  &stdout,
					Stderr:  &stderr,
				})
			assert.Error(t, err, "expected command to fail")

			assert.Equal(t, "stdout\n", stdout.String())
			assert.Equal(t, "stderr\n", stderr.String())
		})
	}
}

// fakeTestingT is a testing.TB that records all Logf calls.
type fakeTestingT struct {
	testing.TB

	logs bytes.Buffer
}

func (t *fakeTestingT) Logf(format string, args ...interface{}) {
	fmt.Fprintf(&t.logs, format+"\n", args...)
}
