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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestDeterminePluginVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		err      error
	}{
		{
			input:    "0.1",
			expected: "0.1",
		},
		{
			input:    "1.0",
			expected: "1.0",
		},
		{
			input:    "1.0.0",
			expected: "1.0.0",
		},
		{
			input: "",
			err:   errors.New(`unexpected number of components in version ""`),
		},
		{
			input: "2",
			err:   errors.New(`unexpected number of components in version "2"`),
		},
		{
			input: "4.3.2.1",
			err:   errors.New(`unexpected number of components in version "4.3.2.1"`),
		},
		{
			input: " 1 . 2 . 3 ",
			err:   errors.New(`parsing major: " 1 "`),
		},
		{
			input: "2.1a123456789",
			err:   errors.New(`parsing minor: "1a123456789"`),
		},
		{
			input: "2.14.0a1605583329",
			err:   errors.New(`parsing patch: "0a1605583329"`),
		},
		{
			input: "1.2.3b123456",
			err:   errors.New(`parsing patch: "3b123456"`),
		},
		{
			input: "3.2.1rc654321",
			err:   errors.New(`parsing patch: "1rc654321"`),
		},
		{
			input: "1.2.3dev7890",
			err:   errors.New(`parsing patch: "3dev7890"`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
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
	t.Run("empty", func(t *testing.T) {
		cwd := t.TempDir()
		_, err := runPythonCommand("", cwd, "-m", "venv", "venv")
		assert.NoError(t, err)
		packages, err := determinePulumiPackages("venv", cwd)
		assert.NoError(t, err)
		assert.Empty(t, packages)
	})
	t.Run("non-empty", func(t *testing.T) {
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
		cwd := t.TempDir()
		_, err := runPythonCommand("", cwd, "-m", "venv", "venv")
		assert.NoError(t, err)
		_, err = runPythonCommand("venv", cwd, "-m", "pip", "install", "pip-install-test")
		assert.NoError(t, err)
		sitePackages, err := runPythonCommand("venv", cwd, "-c", "import site; print(site.getsitepackages()[0])")
		assert.NoError(t, err)
		path := filepath.Join(strings.TrimSpace(string(sitePackages)), "pip_install_test", "pulumiplugin.json")
		t.Logf("Wrote pulumipluing.json file: %s", path)
		bytes := []byte(`{ "name": "thing1", "version": "thing2", "server": "thing3", "resource": true }` + "\n")
		err = os.WriteFile(path, bytes, 0600)
		assert.NoError(t, err)
		packages, err := determinePulumiPackages("venv", cwd)
		assert.NoError(t, err)
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
