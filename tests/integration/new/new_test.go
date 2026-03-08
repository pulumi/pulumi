// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/BurntSushi/toml"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	testutil.SetupPulumiBinary()

	code := m.Run()
	os.Exit(code)
}

// When generating a new Python project, we call into the language runtime, so that it can setup the Python project for
// the chosen toolchain.
func TestPulumiNewPython(t *testing.T) {
	t.Parallel()

	t.Run("pip", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		require.NoError(t, os.Remove(filepath.Join(e.RootPath, ".yarnrc")))
		projectName := "python-pip"

		e.RunCommand("pulumi", "new", "python", "--generate-only", "--yes",
			"--name", projectName,
			"--runtime-options", "toolchain=pip",
		)
		require.True(t, e.PathExists("requirements.txt"))
	})

	for _, toolchain := range []string{"uv", "poetry"} {
		t.Run(toolchain, func(t *testing.T) {
			// Poetry causes issues when run in parallel on windows. See https://github.com/pulumi/pulumi/issues/17183
			if toolchain != "poetry" || runtime.GOOS != "windows" {
				t.Parallel()
			}

			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()
			require.NoError(t, os.Remove(filepath.Join(e.RootPath, ".yarnrc")))
			projectName := "python-" + toolchain
			templatePath, err := filepath.Abs(filepath.Join("python", "random"))
			require.NoError(t, err)

			e.RunCommand("pulumi", "new", templatePath, "--generate-only", "--yes",
				"--name", projectName,
				"--runtime-options", "toolchain="+toolchain,
			)

			require.False(t, e.PathExists("requirements.txt"), "requirements.txt should have been deleted")
			require.True(t, e.PathExists("pyproject.toml"))
			p := filepath.Join(e.RootPath, "pyproject.toml")
			b, err := os.ReadFile(p)
			require.NoError(t, err)
			pyproject := map[string]any{}
			require.NoError(t, toml.Unmarshal(b, &pyproject))
			project, ok := pyproject["project"].(map[string]any)
			require.True(t, ok, "pyproject = %v", pyproject)
			require.Equal(t, project["name"].(string), projectName, "the name should be set")
			switch toolchain {
			case "uv":
				// uv has the dependencies as a string list under `project.dependencies`
				deps, ok := project["dependencies"].([]any)
				require.True(t, ok, "pyproject = %v", pyproject)
				stringDeps := make([]string, len(deps))
				for i, dep := range deps {
					stringDeps[i] = dep.(string)
				}
				require.Contains(t, stringDeps, "pulumi>=3.0.0,<4.0.0")
				require.Contains(t, stringDeps, "pulumi-random>=4.0.0,<5.0.0")
			case "poetry":
				// poetry has the dependencies as a map under `tool.poetry.dependencies`
				tool, ok := pyproject["tool"].(map[string]any)
				require.True(t, ok, "pyproject = %v", pyproject)
				deps, ok := tool["poetry"].(map[string]any)["dependencies"].(map[string]any)
				require.True(t, ok, "pyproject = %v", pyproject)
				require.Equal(t, deps["pulumi"], ">=3.0.0,<4.0.0")
				require.Equal(t, deps["pulumi-random"], ">=4.0.0,<5.0.0")
			}
		})
	}
}

func TestPulumiNewWithPackages(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	require.NoError(t, os.Remove(filepath.Join(e.RootPath, ".yarnrc")))

	templatePath, err := filepath.Abs(filepath.Join("nodejs", "with-packages"))
	require.NoError(t, err)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	e.RunCommand("pulumi", "new", templatePath, "--yes", "--force",
		"--name", "test-packages",
		"--stack", "test-packages",
	)

	e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview")
}
