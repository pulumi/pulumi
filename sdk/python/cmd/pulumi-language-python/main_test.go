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
	"os"
	"os/exec"
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

func getOptions(t *testing.T, name, cwd string) toolchain.PythonOptions {
	t.Helper()
	switch name {
	case "pip":
		return toolchain.PythonOptions{
			Toolchain:  toolchain.Pip,
			Virtualenv: ".venv",
			Root:       cwd,
		}
	case "poetry":
		return toolchain.PythonOptions{
			Toolchain: toolchain.Poetry,
			Root:      cwd,
		}
	case "uv":
		return toolchain.PythonOptions{
			Toolchain: toolchain.Uv,
			Root:      cwd,
		}
	}
	t.Fatalf("unknown toolchain: %s", name)
	return toolchain.PythonOptions{}
}

// createVenv creates a virtual environment in the given directory with the toolchain and installs requirements.
func createVenv(t *testing.T, cwd, toolchainName string, opts toolchain.PythonOptions, requirements ...string) {
	t.Helper()
	//nolint:lll
	poetryToml := `[virtualenvs]
# Create the venv inside the project directory so it gets cleaned up when we remove the temp directory used for the tests.
in-project = true
`
	file, err := os.Create(filepath.Join(cwd, "poetry.toml"))
	require.NoError(t, err)
	defer file.Close()
	_, err = file.WriteString(poetryToml)
	require.NoError(t, err)
	switch toolchainName {
	case "poetry":
		cmd := exec.Command("poetry", "init", "--no-interaction")
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	case "uv":
		cmd := exec.Command("uv", "init")
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	case "pip":
		cmd := exec.Command("python3", "-m", "venv", ".venv")
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	for _, req := range requirements {
		switch toolchainName {
		case "poetry":
			cmd := exec.Command("poetry", "add", req)
			cmd.Dir = cwd
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
		case "uv":
			cmd := exec.Command("uv", "add", req)
			cmd.Dir = cwd
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
		case "pip":
			tc, err := toolchain.ResolveToolchain(opts)
			require.NoError(t, err)
			cmd, err := tc.ModuleCommand(t.Context(), "pip", "install", req)
			require.NoError(t, err)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
		}
	}
}

// pulumiWheel searches for the built pulumi wheel in the sdk/python/dist directory
// and returns its path.
func pulumiWheel(t *testing.T) string {
	dir, err := filepath.Abs(filepath.Join("..", "..", "build"))
	assert.NoError(t, err)
	files, err := os.ReadDir(dir)
	assert.NoError(t, err)
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".whl" {
			return filepath.Join(dir, file.Name())
		}
	}
	t.Fatalf("could not find wheel in %s", dir)
	return ""
}

func TestDeterminePulumiPackages(t *testing.T) {
	t.Parallel()

	// This range needs to be compatible with the pip version in `sdk/python/pyproject.toml`
	pipDependency := "pip>=24"

	for _, toolchainName := range []string{"pip", "poetry", "uv"} {
		toolchainName := toolchainName
		t.Run(toolchainName+"/empty", func(t *testing.T) {
			t.Parallel()
			cwd := t.TempDir()
			opts := getOptions(t, toolchainName, cwd)
			// We need `pip` installed. This is a dependency of `pulumi`, so it will always be
			// available Pulumi virtual environments.
			createVenv(t, cwd, toolchainName, opts, pipDependency)

			packages, err := determinePulumiPackages(t.Context(), opts)

			require.NoError(t, err)
			require.Empty(t, packages)
		})

		t.Run(toolchainName+"/non-empty", func(t *testing.T) {
			t.Parallel()
			cwd := t.TempDir()
			opts := getOptions(t, toolchainName, cwd)

			createVenv(t, cwd, toolchainName, opts, pulumiWheel(t), "pulumi-random", "pip-install-test")

			packages, err := determinePulumiPackages(t.Context(), opts)

			require.NoError(t, err)
			require.NotEmpty(t, packages)
			require.Equal(t, 1, len(packages))
			random := packages[0]
			require.Equal(t, "pulumi_random", random.Name)
			require.NotEmpty(t, random.Location)
		})

		t.Run(toolchainName+"/pulumiplugin", func(t *testing.T) {
			t.Parallel()

			cwd := t.TempDir()
			opts := getOptions(t, toolchainName, cwd)
			createVenv(t, cwd, toolchainName, opts, pipDependency, "pip-install-test==0.5")
			tc, err := toolchain.ResolveToolchain(opts)
			require.NoError(t, err)
			// Find sitePackages folder in Python that contains pip_install_test subfolder.
			var sitePackages string
			cmd, err := tc.Command(t.Context(), "-c",
				"import site; import json; print(json.dumps(site.getsitepackages()))")
			require.NoError(t, err)
			possibleSitePackages, err := cmd.Output()
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

			packages, err := determinePulumiPackages(t.Context(), opts)

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

		t.Run(toolchainName+"/pulumiplugin-resource-false", func(t *testing.T) {
			t.Parallel()

			cwd := t.TempDir()
			opts := getOptions(t, toolchainName, cwd)
			createVenv(t, cwd, toolchainName, opts, pulumiWheel(t))

			// Install a local pulumi SDK that has a pulumi-plugin.json file with `{ "resource": false }`.
			fooSdkDir, err := filepath.Abs(filepath.Join("testdata", "sdks", "foo-1.0.0"))
			assert.NoError(t, err)
			tc, err := toolchain.ResolveToolchain(opts)
			require.NoError(t, err)
			cmd, err := tc.ModuleCommand(t.Context(), "pip", "install", fooSdkDir)
			require.NoError(t, err)
			require.NoError(t, cmd.Run())

			// The package should be considered a Pulumi package since its name is prefixed with "pulumi_".
			packages, err := determinePulumiPackages(t.Context(), opts)
			require.NoError(t, err)
			assert.Equal(t, 1, len(packages))
			assert.Equal(t, "pulumi_foo", packages[0].Name)
			assert.NotEmpty(t, packages[0].Location)

			// There should be no associated plugin since its `resource` field is set to `false`.
			plugin, err := determinePackageDependency(packages[0])
			assert.NoError(t, err)
			assert.Nil(t, plugin)
		})

		t.Run(toolchainName+"/no-pulumiplugin.json-file", func(t *testing.T) {
			t.Parallel()

			cwd := t.TempDir()
			opts := getOptions(t, toolchainName, cwd)
			createVenv(t, cwd, toolchainName, opts, pulumiWheel(t))

			// Install a local old provider SDK that does not have a pulumi-plugin.json file.
			oldSdkDir, err := filepath.Abs(filepath.Join("testdata", "sdks", "old-1.0.0"))
			assert.NoError(t, err)
			tc, err := toolchain.ResolveToolchain(opts)
			require.NoError(t, err)
			cmd, err := tc.ModuleCommand(t.Context(), "pip", "install", oldSdkDir)
			require.NoError(t, err)
			require.NoError(t, cmd.Run())

			// The package should be considered a Pulumi package since its name is prefixed with "pulumi_".
			packages, err := determinePulumiPackages(t.Context(), opts)
			assert.NoError(t, err)
			assert.NotEmpty(t, packages)
			assert.Equal(t, 1, len(packages))
			old := packages[0]
			assert.Equal(t, "pulumi_old", old.Name)
			assert.NotEmpty(t, old.Location)
		})

		t.Run(toolchainName+"/pulumi-policy", func(t *testing.T) {
			t.Parallel()

			cwd := t.TempDir()
			opts := getOptions(t, toolchainName, cwd)
			createVenv(t, cwd, toolchainName, opts, pulumiWheel(t), "pulumi-policy")

			// The package should not be considered a Pulumi package since it is hardcoded not to be,
			// since it does not have an associated plugin.
			packages, err := determinePulumiPackages(t.Context(), opts)
			assert.NoError(t, err)
			assert.Empty(t, packages)
		})
	}
}
