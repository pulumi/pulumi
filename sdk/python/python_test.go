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

package python

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsVirtualEnv(t *testing.T) {
	testBody := func(t *testing.T, forceStartProcess bool) {
		// Create a new empty test directory.
		tempdir, _ := ioutil.TempDir("", "test-env")
		defer os.RemoveAll(tempdir)

		// Assert the empty test directory is not a virtual environment.
		assert.False(t, IsVirtualEnv(tempdir))

		// Create and run a python command to create a virtual environment.
		venvDir := filepath.Join(tempdir, "venv")
		cmd, err := Command([]string{"-m", "venv", venvDir})
		assert.NoError(t, err)
		asWrappedCmd := cmd.(*wrappedCmd)
		asWrappedCmd.useStartProcess = forceStartProcess
		err = cmd.Run()
		assert.NoError(t, err)

		// Assert the new venv directory is a virtual environment.
		assert.True(t, IsVirtualEnv(venvDir))
	}

	// Run with the default exec.Cmd support
	t.Run("DefaultCmd", func(t *testing.T) {
		testBody(t, false)
	})

	// Run with StartProcess
	t.Run("StartProcess", func(t *testing.T) {
		testBody(t, true)
	})
}

func TestActivateVirtualEnv(t *testing.T) {
	venvName := "venv"
	venvDir := filepath.Join(venvName, "bin")
	if runtime.GOOS == windows {
		venvDir = filepath.Join(venvName, "Scripts")
	}

	tests := []struct {
		input    []string
		expected []string
	}{
		{
			input:    []string{"PYTHONHOME=foo", "PATH=bar", "FOO=blah"},
			expected: []string{fmt.Sprintf("PATH=%s%sbar", venvDir, string(os.PathListSeparator)), "FOO=blah"},
		},
		{
			input:    []string{"PYTHONHOME=foo", "FOO=blah"},
			expected: []string{"FOO=blah", fmt.Sprintf("PATH=%s", venvDir)},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v", test.input), func(t *testing.T) {
			actual := ActivateVirtualEnv(test.input, venvName)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestRunningPipInVirtualEnvironment(t *testing.T) {
	// Skip during short test runs since this test involves downloading dependencies.
	if testing.Short() {
		t.Skip("Skipped in short test run")
	}

	testBody := func(t *testing.T, forceStartProcess bool) {
		// Create a new empty test directory.
		tempdir, _ := ioutil.TempDir("", "test-env")
		defer os.RemoveAll(tempdir)

		// Create and run a python command to create a virtual environment.
		venvDir := filepath.Join(tempdir, "venv")
		cmd, err := Command([]string{"-m", "venv", venvDir})
		assert.NoError(t, err)
		err = cmd.Run()
		assert.NoError(t, err)

		// Create a requirements.txt file in the temp directory.
		requirementsFile := filepath.Join(tempdir, "requirements.txt")
		assert.NoError(t, ioutil.WriteFile(requirementsFile, []byte("pulumi==2.0.0\n"), 0600))

		// Create a command to run pip from the virtual environment.
		pipCmd, err := VirtualEnvCommand(
			venvDir,
			"python",
			[]string{"-m", "pip", "install", "-r", "requirements.txt"},
			WithDir(tempdir),
			WithEnv(ActivateVirtualEnv(os.Environ(), venvDir)),
		)

		asWrappedCmd := pipCmd.(*wrappedCmd)
		asWrappedCmd.useStartProcess = forceStartProcess
		// Run the command.
		output, err := pipCmd.CombinedOutput()
		t.Logf("%s", string(output))
		if err != nil {
			assert.Failf(t, "pip install command failed with output: %s", string(output))
		}
	}

	// Run with the default exec.Cmd support
	t.Run("DefaultCmd", func(t *testing.T) {
		testBody(t, false)
	})

	// Run with StartProcess
	t.Run("StartProcess", func(t *testing.T) {
		testBody(t, true)
	})
}
