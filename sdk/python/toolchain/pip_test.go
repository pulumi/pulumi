// Copyright 2016-2020, Pulumi Corporation.
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

package toolchain

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsVirtualEnv(t *testing.T) {
	t.Parallel()

	// Create a new empty test directory.
	tempdir := t.TempDir()

	// Assert the empty test directory is not a virtual environment.
	assert.False(t, IsVirtualEnv(tempdir))

	// Create and run a python command to create a virtual environment.
	venvDir := filepath.Join(tempdir, "venv")
	cmd, err := Command(context.Background(), "-m", "venv", venvDir)
	require.NoError(t, err)
	err = cmd.Run()
	require.NoError(t, err)

	// Assert the new venv directory is a virtual environment.
	assert.True(t, IsVirtualEnv(venvDir))
}

func TestActivateVirtualEnv(t *testing.T) {
	t.Parallel()

	venvDir := "/some/path/venv"
	venvBinDir := filepath.Join(venvDir, "bin")
	if runtime.GOOS == windows {
		venvBinDir = filepath.Join(venvDir, "Scripts")
	}

	tests := []struct {
		input    []string
		expected []string
	}{
		{
			input: []string{"PYTHONHOME=foo", "PATH=bar", "FOO=blah"},
			expected: []string{
				fmt.Sprintf("PATH=%s%sbar", venvBinDir, string(os.PathListSeparator)),
				"FOO=blah",
				"VIRTUAL_ENV=" + venvDir,
			},
		},
		{
			input: []string{"PYTHONHOME=foo", "FOO=blah"},
			expected: []string{
				"FOO=blah",
				"PATH=" + venvBinDir,
				"VIRTUAL_ENV=" + venvDir,
			},
		},
		{
			input: []string{"PythonHome=foo", "Path=bar"},
			expected: []string{
				fmt.Sprintf("Path=%s%sbar", venvBinDir, string(os.PathListSeparator)),
				"VIRTUAL_ENV=" + venvDir,
			},
		},
	}
	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%#v", test.input), func(t *testing.T) {
			t.Parallel()

			actual := ActivateVirtualEnv(test.input, venvDir)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestRunningPipInVirtualEnvironment(t *testing.T) {
	t.Parallel()

	// Skip during short test runs since this test involves downloading dependencies.
	if testing.Short() {
		t.Skip("Skipped in short test run")
	}

	// Create a new empty test directory.
	tempdir := t.TempDir()

	// Create and run a python command to create a virtual environment.
	venvDir := filepath.Join(tempdir, "venv")
	cmd, err := Command(context.Background(), "-m", "venv", venvDir)
	require.NoError(t, err)
	err = cmd.Run()
	require.NoError(t, err)

	// Create a requirements.txt file in the temp directory.
	requirementsFile := filepath.Join(tempdir, "requirements.txt")
	require.NoError(t, os.WriteFile(requirementsFile, []byte("pulumi==2.0.0\n"), 0o600))

	// Create a command to run pip from the virtual environment.
	pipCmd := VirtualEnvCommand(venvDir, "python", "-m", "pip", "install", "-r", "requirements.txt")
	pipCmd.Dir = tempdir
	pipCmd.Env = ActivateVirtualEnv(os.Environ(), venvDir)

	// Run the command.
	if output, err := pipCmd.CombinedOutput(); err != nil {
		assert.Failf(t, "pip install command failed with output: %s", string(output))
	}
}

func TestCommandNoVenv(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO[pulumi/pulumi#19675]: Fix this test on Windows
		t.Skip("Skipping tests on Windows")
	}
	t.Parallel()

	tc, err := newPip(".", "")
	require.NoError(t, err)

	cmd, err := tc.Command(context.Background())
	require.NoError(t, err)

	globalPython, err := exec.LookPath("python3")
	require.NoError(t, err)

	require.Equal(t, globalPython, cmd.Path, "Toolchain should use the global python executable")

	require.Nil(t, cmd.Env)
}

func TestCommandPulumiPythonCommand(t *testing.T) {
	t.Setenv("PULUMI_PYTHON_CMD", "python-not-found")

	tc, err := newPip(".", "")
	require.NoError(t, err)

	cmd, err := tc.Command(context.Background())
	require.ErrorContains(t, err, "python-not-found")
	require.Nil(t, cmd)
}
