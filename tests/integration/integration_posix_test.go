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
	"bytes"
	"os/exec"
	"syscall"
	"testing"
	"time"

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
			e.ImportDirectory("cancel")

			stackName := ptesting.RandomStackName()
			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "install")
			e.RunCommand("pulumi", "stack", "init", stackName)
			cmd := e.SetupCommandIn(e.CWD, "pulumi", "up", "--skip-preview", "--yes")

			var outBuffer bytes.Buffer
			var errBuffer bytes.Buffer
			cmd.Stdout = &outBuffer
			cmd.Stderr = &errBuffer

			err := cmd.Start()
			require.NoError(t, err)

			time.Sleep(1 * time.Second)
			require.NoError(t, syscall.Kill(cmd.Process.Pid, sig))

			err = cmd.Wait()
			if exiterr, ok := err.(*exec.ExitError); ok {
				require.NotEqual(t, 0, exiterr.ExitCode())
			} else {
				require.Fail(t, "Expected ExitError")
			}
			require.Contains(t, outBuffer.String(), "error: update canceled")
		})
	}
}
