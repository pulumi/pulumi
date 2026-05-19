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

package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/project/newcmd"
)

// useTempFilestateBackend points the backend at a temp directory so tests don't hit the real backend.
func useTempFilestateBackend(t *testing.T) {
	t.Setenv("PULUMI_BACKEND_URL", "file://"+filepath.ToSlash(t.TempDir()))
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test")
}

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestNewCmdYesWritesMinimalPulumiYAMLWithExplicitName(t *testing.T) {
	useTempFilestateBackend(t)
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newcmd.NewNewCmd()
	cmd.SetArgs([]string{"-y", "--name", "my-project"})

	err := cmd.Execute()

	require.NoError(t, err)
	contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "name: my-project\n", string(contents))
}

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestNewCmdYesUsesCurrentDirectoryNameByDefault(t *testing.T) {
	useTempFilestateBackend(t)
	dir := filepath.Join(t.TempDir(), "my-project")
	require.NoError(t, os.Mkdir(dir, 0o755))
	t.Chdir(dir)
	cmd := newcmd.NewNewCmd()
	cmd.SetArgs([]string{"-y"})

	err := cmd.Execute()

	require.NoError(t, err)
	contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "name: my-project\n", string(contents))
}

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestNewCmdYesSanitizesDefaultDirectoryName(t *testing.T) {
	useTempFilestateBackend(t)
	dir := filepath.Join(t.TempDir(), "my project!")
	require.NoError(t, os.Mkdir(dir, 0o755))
	t.Chdir(dir)
	cmd := newcmd.NewNewCmd()
	cmd.SetArgs([]string{"-y"})

	err := cmd.Execute()

	require.NoError(t, err)
	contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "name: myproject\n", string(contents))
}

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestNewCmdYesRejectsInvalidExplicitName(t *testing.T) {
	useTempFilestateBackend(t)
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newcmd.NewNewCmd()
	cmd.SetArgs([]string{"-y", "--name", "my project"})

	err := cmd.Execute()

	require.ErrorContains(t, err, "'my project' is not a valid project name")
	_, statErr := os.Stat(filepath.Join(dir, "Pulumi.yaml"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestNewCmdYesDoesNotOverwriteExistingPulumiYAML(t *testing.T) {
	useTempFilestateBackend(t)
	dir := t.TempDir()
	existing := filepath.Join(dir, "Pulumi.yaml")
	require.NoError(t, os.WriteFile(existing, []byte("name: existing\n"), 0o600))
	t.Chdir(dir)
	cmd := newcmd.NewNewCmd()
	cmd.SetArgs([]string{"-y"})

	err := cmd.Execute()

	require.ErrorContains(t, err, dir+" is not empty;")
	contents, readErr := os.ReadFile(existing)
	require.NoError(t, readErr)
	assert.Equal(t, "name: existing\n", string(contents))
}
