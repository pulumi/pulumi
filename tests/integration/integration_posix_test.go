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
	"context"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

func TestCancelSignal(t *testing.T) {
	t.Parallel()
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		sig := sig
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()
			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()
			stackName := ptesting.RandomStackName()
			e.ImportDirectory("cancel")
			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "install")
			e.RunCommand("pulumi", "stack", "init", stackName)
			cmd := e.SetupCommandIn(context.Background(), e.CWD, "pulumi", "up", "--skip-preview", "--yes")

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
