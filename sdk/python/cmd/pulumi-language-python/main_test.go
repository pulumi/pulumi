// Copyright 2016-2021, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterminePluginVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
		err      error
	}{
		{
			input:    "0.1",
			expected: "0.1.0",
		},
		{
			input:    "1.0",
			expected: "1.0.0",
		},
		{
			input:    "1.0.0",
			expected: "1.0.0",
		},
		{
			input: "",
			err:   fmt.Errorf("Cannot parse empty string"),
		},
		{
			input:    "4.3.2.1",
			expected: "4.3.2.1",
		},
		{
			input: " 1 . 2 . 3 ",
			err:   fmt.Errorf(`' 1 . 2 . 3 ' still unparsed`),
		},
		{
			input:    "2.1a123456789",
			expected: "2.1.0-alpha.123456789",
		},
		{
			input:    "2.14.0a1605583329",
			expected: "2.14.0-alpha.1605583329",
		},
		{
			input:    "1.2.3b123456",
			expected: "1.2.3-beta.123456",
		},
		{
			input:    "3.2.1rc654321",
			expected: "3.2.1-rc.654321",
		},
		{
			input: "1.2.3dev7890",
			err:   fmt.Errorf("'dev7890' still unparsed"),
		},
		{
			input:    "1.2.3.dev456",
			expected: "1.2.3+dev456",
		},
		{
			input: "1.",
			err:   fmt.Errorf("'.' still unparsed"),
		},
		{
			input:    "3.2.post32",
			expected: "3.2.0+post32",
		},
		{
			input:    "0.3.0b8",
			expected: "0.3.0-beta.8",
		},
		{
			input: "10!3.2.1",
			err:   fmt.Errorf("Epochs are not supported"),
		},
		{
			input:    "3.2.post1.dev0",
			expected: "3.2.0+post1dev0",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			result, err := determinePluginVersion(tt.input)
			if tt.err != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeterminePulumiPackages(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		cwd := t.TempDir()
		_, err := runPythonCommand("", cwd, "-m", "venv", "venv")
		assert.NoError(t, err)
		packages, err := determinePulumiPackages("venv", cwd)
		assert.NoError(t, err)
		assert.Empty(t, packages)
	})
	t.Run("non-empty", func(t *testing.T) {
		t.Parallel()

		cwd := t.TempDir()
		_, err := runPythonCommand("", cwd, "-m", "venv", "venv")
		assert.NoError(t, err)
		_, err = runPythonCommand("venv", cwd, "-m", "pip", "install", "pulumi-random")
		assert.NoError(t, err)
		_, err = runPythonCommand("venv", cwd, "-m", "pip", "install", "pip-install-test")
		assert.NoError(t, err)
		packages, err := determinePulumiPackages("venv", cwd)
		assert.NoError(t, err)
		assert.NotEmpty(t, packages)
		assert.Equal(t, 1, len(packages))
		random := packages[0]
		assert.Equal(t, "pulumi-random", random.Name)
		assert.NotEmpty(t, random.Location)
	})
	t.Run("pulumiplugin", func(t *testing.T) {
		t.Parallel()

		cwd := t.TempDir()
		_, err := runPythonCommand("", cwd, "-m", "venv", "venv")
		require.NoError(t, err)
		_, err = runPythonCommand("venv", cwd, "-m", "pip", "install", "pip-install-test")
		require.NoError(t, err)
		// Find sitePackages folder in Python that contains pip_install_test subfolder.
		var sitePackages string
		possibleSitePackages, err := runPythonCommand("venv", cwd, "-c",
			"import site; import json; print(json.dumps(site.getsitepackages()))")
		require.NoError(t, err)
		var possibleSitePackagePaths []string
		err = json.Unmarshal(possibleSitePackages, &possibleSitePackagePaths)
		require.NoError(t, err)
		for _, dir := range possibleSitePackagePaths {
			_, err := os.Stat(filepath.Join(dir, "pip_install_test"))
			if os.IsNotExist(err) {
				continue
			}
			require.NoError(t, err)
			sitePackages = dir
		}
		if sitePackages == "" {
			t.Error("None of Python site.getsitepackages() folders contain a pip_install_test subfolder")
			t.FailNow()
		}
		path := filepath.Join(sitePackages, "pip_install_test", "pulumi-plugin.json")
		bytes := []byte(`{ "name": "thing1", "version": "thing2", "server": "thing3", "resource": true }` + "\n")
		err = os.WriteFile(path, bytes, 0600)
		require.NoError(t, err)
		t.Logf("Wrote pulumipluing.json file: %s", path)
		packages, err := determinePulumiPackages("venv", cwd)
		require.NoError(t, err)
		assert.Equal(t, 1, len(packages))
		pipInstallTest := packages[0]
		assert.NotNil(t, pipInstallTest.plugin)
		assert.Equal(t, "pip-install-test", pipInstallTest.Name)
		assert.NotEmpty(t, pipInstallTest.Location)
		assert.Equal(t, "thing1", pipInstallTest.plugin.Name)
		assert.Equal(t, "thing2", pipInstallTest.plugin.Version)
		assert.Equal(t, "thing3", pipInstallTest.plugin.Server)
		assert.True(t, pipInstallTest.plugin.Resource)
	})
}
