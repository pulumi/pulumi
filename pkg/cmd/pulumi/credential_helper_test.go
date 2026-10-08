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

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--pulumi-command-test" {
		root, cleanup := NewPulumiCmd()
		root.SetArgs(os.Args[2:])
		err := root.Execute()
		cleanup()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runCredentialCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	return runCredentialCommandAt(t, executable, args...)
}

func runCredentialCommandAt(t *testing.T, executable string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")
	command := exec.CommandContext(t.Context(), executable, append([]string{
		"--pulumi-command-test", "--non-interactive",
	}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, output)
	}
	return string(output), nil
}

func TestCredentialHelperDoesNotRunWithoutBackend(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_CREDENTIAL_HELPER", filepath.Join(t.TempDir(), "missing-helper"))
	_, err := runCredentialCommand(t, "version")
	require.NoError(t, err)
}
