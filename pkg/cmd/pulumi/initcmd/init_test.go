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

package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestInitCmdWritesMinimalPulumiYAMLWithExplicitName(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var output bytes.Buffer
	cmd := NewInitCmd()
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"my-project"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Created Pulumi.yaml\n", output.String())
	contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "name: my-project\n", string(contents))
}

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestInitCmdUsesCurrentDirectoryNameByDefault(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-project")
	require.NoError(t, os.Mkdir(dir, 0o755))
	t.Chdir(dir)
	cmd := NewInitCmd()

	err := cmd.Execute()

	require.NoError(t, err)
	contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "name: my-project\n", string(contents))
}

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestInitCmdSanitizesDefaultDirectoryName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my project!")
	require.NoError(t, os.Mkdir(dir, 0o755))
	t.Chdir(dir)
	cmd := NewInitCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	require.NoError(t, err)
	contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "name: myproject\n", string(contents))
}

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestInitCmdRejectsInvalidExplicitName(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := NewInitCmd()
	cmd.SetArgs([]string{"my project"})

	err := cmd.Execute()

	require.ErrorContains(t, err, "'my project' is not a valid project name")
	_, statErr := os.Stat(filepath.Join(dir, "Pulumi.yaml"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

//nolint:paralleltest // These tests modify the working directory, so cannot be run in parallel.
func TestInitCmdDoesNotOverwriteExistingPulumiYAML(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "Pulumi.yaml")
	require.NoError(t, os.WriteFile(existing, []byte("name: existing\n"), 0o600))
	t.Chdir(dir)
	cmd := NewInitCmd()

	err := cmd.Execute()

	require.ErrorContains(t, err, existing+" already exists")
	contents, readErr := os.ReadFile(existing)
	require.NoError(t, readErr)
	assert.Equal(t, "name: existing\n", string(contents))
}

func TestInitCmdRejectsTooManyArgs(t *testing.T) {
	t.Parallel()

	cmd := NewInitCmd()
	cmd.SetArgs([]string{"one", "two"})

	err := cmd.Execute()

	require.ErrorContains(t, err, "accepts at most 1 arg(s), received 2")
}
