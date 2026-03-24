// Copyright 2024, Pulumi Corporation.
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

//go:build !windows && !js

package ints

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

func TestCancelSignal(t *testing.T) {
	t.Parallel()
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()
			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()
			stackName := ptesting.RandomStackName()
			e.ImportDirectory("cancel")
			sdkPath, err := filepath.Abs(filepath.Join("..", "..", "sdk", "python"))
			require.NoError(t, err)
			err = os.WriteFile(filepath.Join(e.CWD, "requirements.txt"), []byte(sdkPath+"\n"), 0o600)
			require.NoError(t, err)
			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "install")
			e.RunCommand("pulumi", "stack", "init", stackName)
			cmd := e.SetupCommandIn(t.Context(), e.CWD, "pulumi", "up", "--skip-preview", "--yes")

			var outBuffer bytes.Buffer
			var errBuffer bytes.Buffer
			reader, err := cmd.StdoutPipe()
			require.NoError(t, err)
			cmd.Stderr = &errBuffer
			stdout := io.TeeReader(reader, &outBuffer)

			// Start the command and wait for it to write `Sleeping` to stdout
			err = cmd.Start()
			require.NoError(t, err)
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), "Sleeping") {
					break
				}
			}

			// Send the signal
			require.NoError(t, syscall.Kill(cmd.Process.Pid, sig))

			// Read the rest of the output and wait for the command to finish
			for scanner.Scan() {
				scanner.Text()
			}
			err = cmd.Wait()

			// We should exit with a non 0 code and the output should contain `update canceled`.
			require.Error(t, err)
			if exiterr, ok := err.(*exec.ExitError); ok {
				require.NotEqual(t, 0, exiterr.ExitCode())
			} else {
				require.Fail(t, "Expected ExitError")
			}
			require.Contains(t, outBuffer.String(), "error: update canceled")
		})
	}
}

// TestCancelNoLeakedProcesses verifies that when a Pulumi program ignores SIGINT, sending a signal to the pulumi CLI
// still results in all child processes being cleaned up.
func TestCancelNoLeakedProcesses(t *testing.T) {
	t.Parallel()
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()
			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()

			stackName := ptesting.RandomStackName()
			e.ImportDirectory("cancel_no_leak")
			sdkPath, err := filepath.Abs(filepath.Join("..", "..", "sdk", "python"))
			require.NoError(t, err)
			err = os.WriteFile(filepath.Join(e.CWD, "requirements.txt"), []byte(sdkPath+"\n"), 0o600)
			require.NoError(t, err)
			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "install")
			e.RunCommand("pulumi", "stack", "init", stackName)

			cmd := e.SetupCommandIn(t.Context(), e.CWD, "pulumi", "up", "--skip-preview", "--yes")

			var outBuffer bytes.Buffer
			reader, err := cmd.StdoutPipe()
			require.NoError(t, err)
			cmd.Stderr = &bytes.Buffer{}
			stdout := io.TeeReader(reader, &outBuffer)

			err = cmd.Start()
			require.NoError(t, err)

			// Wait for the Python program to print "PID=<pid>".
			pidRe := regexp.MustCompile(`PID=(\d+)`)
			var pythonPID int
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				t.Logf("stdout: %s", line)
				if m := pidRe.FindStringSubmatch(line); m != nil {
					pid, err := strconv.Atoi(m[1])
					require.NoError(t, err, "parsing PID from program output")
					pythonPID = pid
					break
				}
			}
			go func() {
				for scanner.Scan() {
					scanner.Text()
				}
			}()

			require.NotZero(t, pythonPID, "should have captured the Python process PID")
			require.NoError(t, syscall.Kill(pythonPID, 0), "Python process should be alive before signal")

			// Now signal the process
			require.NoError(t, syscall.Kill(cmd.Process.Pid, sig))

			require.Eventually(t, func() bool {
				err := syscall.Kill(pythonPID, 0)
				return err == syscall.ESRCH // "no such process"
			}, 30*time.Second, 100*time.Millisecond,
				"Python process %d should have been killed after pulumi exited", pythonPID)
		})
	}
}
