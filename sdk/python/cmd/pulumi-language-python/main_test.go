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
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveReleaseCandidateSuffix(t *testing.T) {
	t.Parallel()

	require.Equal(t, "3.13.0", removeReleaseCandidateSuffix("3.13.0rc0"))
	require.Equal(t, "3.13.0", removeReleaseCandidateSuffix("3.13.0rc1"))
	require.Equal(t, "3.13.0", removeReleaseCandidateSuffix("3.13.0rc345"))
	require.Equal(t, "3.13.0-banana", removeReleaseCandidateSuffix("3.13.0-banana"))
}

func TestDeterminePluginVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
		err      string
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
			err:   "cannot parse empty string",
		},
		{
			input:    "4.3.2.1",
			expected: "4.3.2.1",
		},
		{
			input: " 1 . 2 . 3 ",
			err:   `' 1 . 2 . 3 ' still unparsed`,
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
			err:   "'dev7890' still unparsed",
		},
		{
			input:    "1.2.3.dev456",
			expected: "1.2.3+dev456",
		},
		{
			input: "1.",
			err:   "'.' still unparsed",
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
			err:   "epochs are not supported",
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
			if tt.err != "" {
				assert.EqualError(t, err, tt.err)
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
		_, err := runPythonModuleCommand(t, "", cwd, "venv", "venv")
		assert.NoError(t, err)
		packages, err := determinePulumiPackages(context.Background(), toolchain.PythonOptions{
			Toolchain:  toolchain.Pip,
			Root:       cwd,
			Virtualenv: "venv",
		})
		assert.NoError(t, err)
		assert.Empty(t, packages)
	})
	t.Run("non-empty", func(t *testing.T) {
		t.Parallel()

		cwd := t.TempDir()
		_, err := runPythonModuleCommand(t, "", cwd, "venv", "venv")
		assert.NoError(t, err)

		// Install the local Pulumi SDK into the virtual environment.
		sdkDir, err := filepath.Abs(filepath.Join("..", "..", "env", "src"))
		assert.NoError(t, err)
		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", "-e", sdkDir)
		assert.NoError(t, err)

		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", "pulumi-random")
		assert.NoError(t, err)
		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", "pip-install-test")
		assert.NoError(t, err)
		packages, err := determinePulumiPackages(context.Background(), toolchain.PythonOptions{
			Toolchain:  toolchain.Pip,
			Root:       cwd,
			Virtualenv: "venv",
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, packages)
		assert.Equal(t, 1, len(packages))
		random := packages[0]
		assert.Equal(t, "pulumi_random", random.Name)
		assert.NotEmpty(t, random.Location)
	})
	t.Run("pulumiplugin", func(t *testing.T) {
		t.Parallel()

		cwd := t.TempDir()
		_, err := runPythonModuleCommand(t, "", cwd, "venv", "venv")
		require.NoError(t, err)
		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", "pip-install-test")
		require.NoError(t, err)
		// Find sitePackages folder in Python that contains pip_install_test subfolder.
		var sitePackages string
		possibleSitePackages, err := runPythonCommand(t, "venv", cwd, "-c",
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
		err = os.WriteFile(path, bytes, 0o600)
		require.NoError(t, err)
		t.Logf("Wrote pulumi-plugin.json file: %s", path)
		packages, err := determinePulumiPackages(context.Background(), toolchain.PythonOptions{
			Toolchain:  toolchain.Pip,
			Root:       cwd,
			Virtualenv: "venv",
		})
		require.NoError(t, err)
		assert.Equal(t, 1, len(packages))
		pipInstallTest := packages[0]
		assert.Equal(t, "pip-install-test", pipInstallTest.Name)
		assert.NotEmpty(t, pipInstallTest.Location)

		plugin, err := determinePackageDependency(pipInstallTest)
		assert.NoError(t, err)
		assert.NotNil(t, plugin)
		assert.Equal(t, "thing1", plugin.Name)
		assert.Equal(t, "vthing2", plugin.Version)
		assert.Equal(t, "thing3", plugin.Server)
		assert.Equal(t, "resource", plugin.Kind)
	})
	t.Run("pulumiplugin-resource-false", func(t *testing.T) {
		t.Parallel()

		cwd := t.TempDir()
		_, err := runPythonModuleCommand(t, "", cwd, "venv", "venv")
		assert.NoError(t, err)

		_, err = runPythonModuleCommand(t,
			"venv", cwd, "pip", "install", "--upgrade", "pip", "setuptools")
		assert.NoError(t, err)

		// Install the local Pulumi SDK into the virtual environment.
		sdkDir, err := filepath.Abs(filepath.Join("..", "..", "env", "src"))
		assert.NoError(t, err)
		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", "-e", sdkDir)
		assert.NoError(t, err)

		// Install a local pulumi SDK that has a pulumi-plugin.json file with `{ "resource": false }`.
		fooSdkDir, err := filepath.Abs(filepath.Join("testdata", "sdks", "foo-1.0.0"))
		assert.NoError(t, err)
		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", fooSdkDir)
		assert.NoError(t, err)

		// The package should be considered a Pulumi package since its name is prefixed with "pulumi_".
		packages, err := determinePulumiPackages(context.Background(), toolchain.PythonOptions{
			Toolchain:  toolchain.Pip,
			Root:       cwd,
			Virtualenv: "venv",
		})
		require.NoError(t, err)
		assert.Equal(t, 1, len(packages))
		assert.Equal(t, "pulumi_foo", packages[0].Name)
		assert.NotEmpty(t, packages[0].Location)

		// There should be no associated plugin since its `resource` field is set to `false`.
		plugin, err := determinePackageDependency(packages[0])
		assert.NoError(t, err)
		assert.Nil(t, plugin)
	})
	t.Run("no-pulumiplugin.json-file", func(t *testing.T) {
		t.Parallel()

		cwd := t.TempDir()
		_, err := runPythonModuleCommand(t, "", cwd, "venv", "venv")
		assert.NoError(t, err)

		_, err = runPythonModuleCommand(t,
			"venv", cwd, "pip", "install", "--upgrade", "pip", "setuptools")
		assert.NoError(t, err)

		// Install the local Pulumi SDK into the virtual environment.
		sdkDir, err := filepath.Abs(filepath.Join("..", "..", "env", "src"))
		assert.NoError(t, err)
		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", "-e", sdkDir)
		assert.NoError(t, err)

		// Install a local old provider SDK that does not have a pulumi-plugin.json file.
		oldSdkDir, err := filepath.Abs(filepath.Join("testdata", "sdks", "old-1.0.0"))
		assert.NoError(t, err)
		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", oldSdkDir)
		assert.NoError(t, err)

		// The package should be considered a Pulumi package since its name is prefixed with "pulumi_".
		packages, err := determinePulumiPackages(context.Background(), toolchain.PythonOptions{
			Toolchain:  toolchain.Pip,
			Root:       cwd,
			Virtualenv: "venv",
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, packages)
		assert.Equal(t, 1, len(packages))
		old := packages[0]
		assert.Equal(t, "pulumi_old", old.Name)
		assert.NotEmpty(t, old.Location)
	})
	t.Run("pulumi-policy", func(t *testing.T) {
		t.Parallel()

		cwd := t.TempDir()
		_, err := runPythonModuleCommand(t, "", cwd, "venv", "venv")
		assert.NoError(t, err)

		_, err = runPythonModuleCommand(t,
			"venv", cwd, "pip", "install", "--upgrade", "pip", "setuptools")
		assert.NoError(t, err)

		// Install the local Pulumi SDK into the virtual environment.
		sdkDir, err := filepath.Abs(filepath.Join("..", "..", "env", "src"))
		assert.NoError(t, err)
		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", "-e", sdkDir)
		assert.NoError(t, err)

		// Install pulumi-policy.
		assert.NoError(t, err)
		_, err = runPythonModuleCommand(t, "venv", cwd, "pip", "install", "pulumi-policy")
		assert.NoError(t, err)

		// The package should not be considered a Pulumi package since it is hardcoded not to be,
		// since it does not have an associated plugin.
		packages, err := determinePulumiPackages(context.Background(), toolchain.PythonOptions{
			Toolchain:  toolchain.Pip,
			Root:       cwd,
			Virtualenv: "venv",
		})
		assert.NoError(t, err)
		assert.Empty(t, packages)
	})
}

func runPythonModuleCommand(t *testing.T, virtualenv, cwd, module string, args ...string) ([]byte, error) {
	return runPythonCommand(t, virtualenv, cwd, append([]string{"-m", module}, args...)...)
}

func runPythonCommand(t *testing.T, virtualenv, cwd string, args ...string) ([]byte, error) {
	t.Helper()

	tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
		Toolchain:  toolchain.Pip,
		Root:       cwd,
		Virtualenv: virtualenv,
	})
	require.NoError(t, err)
	cmd, err := tc.Command(context.Background(), args...)
	require.NoError(t, err)
	cmd.Dir = cwd

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return output, err
}
