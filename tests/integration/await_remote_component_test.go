// Copyright 2026, Pulumi Corporation.
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

package ints

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/tests/testutil"
)

// TestAwaitingChildOfRemoteComponent checks that the output of an awaiting resource that is exposed by a remote
// component stays unknown in the program during a real update, so applies on it do not run with an empty value.
//
//nolint:paralleltest // NewEnvironment installs a per-test PULUMI_HOME that must not be shared concurrently.
func TestAwaitingChildOfRemoteComponent(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	abs := func(p string) string {
		a, err := filepath.Abs(p)
		require.NoError(t, err)
		return a
	}
	programDir := abs(filepath.Join("await_remote_component", "program"))
	componentDir := abs(filepath.Join("await_remote_component", "component"))
	readyFile := filepath.Join(e.CWD, "ready")
	marker := filepath.Join(e.CWD, "apply-marker")

	pulumiYaml := fmt.Sprintf(`name: await-remote-component
runtime: go
main: %s
plugins:
  providers:
    - name: testprovider
      path: %s
    - name: awaitcomp
      path: %s
`, programDir, testutil.TestProviderDir(t), componentDir)
	require.NoError(t, os.WriteFile(filepath.Join(e.CWD, "Pulumi.yaml"), []byte(pulumiYaml), 0o600))

	e.Backend = e.LocalURL()
	e.Env = append(e.Env, "PULUMI_CONFIG_PASSPHRASE=", "AWAIT_READY_FILE="+readyFile, "AWAIT_APPLY_MARKER="+marker)
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "test", "--secrets-provider", "passphrase")

	up := func() (int, string) {
		cmd := e.SetupCommandIn(t.Context(), e.CWD, "pulumi", "up", "--yes", "--skip-preview")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return 0, string(out)
		}
		exitErr, ok := err.(*exec.ExitError)
		require.True(t, ok, "unexpected error %v", err)
		return exitErr.ExitCode(), string(out)
	}

	// First update: the child awaits, so the component output must stay unknown and the apply must not run.
	code, out := up()
	assert.Equal(t, 10, code, "expected the await exit code; output:\n%s", out)
	got, err := os.ReadFile(marker)
	assert.ErrorIs(t, err, os.ErrNotExist,
		"apply on an unknown component output ran with value %q; output:\n%s", got, out)
	_ = os.Remove(marker)

	// Second update: the child succeeds, so the apply runs with the real value.
	require.NoError(t, os.WriteFile(readyFile, nil, 0o600))
	code, out = up()
	require.Equal(t, 0, code, "output:\n%s", out)
	got, err = os.ReadFile(marker)
	require.NoError(t, err)
	assert.Equal(t, "ready", string(got))
}
