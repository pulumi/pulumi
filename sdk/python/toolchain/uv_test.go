// Copyright 2024, Pulumi Corporation.
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
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
)

func TestUvVirtualenvPath(t *testing.T) {
	t.Parallel()

	t.Run("no virtualenv specified", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		uv, err := newUv(root, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, ".venv"), uv.virtualenvPath, "virtualenv is in the project root")
	})

	t.Run("no virtualenv specified, in a subfolder", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		pulumiRoot := filepath.Join(root, "subfolder")
		require.NoError(t, os.WriteFile(filepath.Join(root, "uv.lock"), []byte{}, 0o600))
		require.NoError(t, os.Mkdir(pulumiRoot, 0o755))

		uv, err := newUv(pulumiRoot, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, ".venv"), uv.virtualenvPath, "virtualenv is next to uv.lock")
	})

	t.Run("no virtualenv specified, in a subfolder", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		pulumiRoot := filepath.Join(root, "subfolder")
		require.NoError(t, os.Mkdir(pulumiRoot, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(root, "uv.lock"), []byte{}, 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(pulumiRoot, "uv.lock"), []byte{}, 0o600))

		uv, err := newUv(pulumiRoot, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(pulumiRoot, ".venv"), uv.virtualenvPath,
			"virtualenv is next to the uv.lock closest to the project root")
	})

	t.Run("virtualenv option is provided", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		uv, err := newUv(root, "banana")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, "banana"), uv.virtualenvPath, "virtualenv is in the project root")
	})

	t.Run("virtualenv option is provided, in  subfolder", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		pulumiRoot := filepath.Join(root, "subfolder")
		require.NoError(t, os.Mkdir(pulumiRoot, 0o755))

		uv, err := newUv(pulumiRoot, "banana")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(pulumiRoot, "banana"), uv.virtualenvPath, "virtualenv is in the project root")
	})
}

func TestUvVirtualenvPathUvProjectEnvironment(t *testing.T) {
	t.Run("absolute path", func(t *testing.T) {
		root := t.TempDir()
		venv := filepath.Join(t.TempDir(), "custom-env")
		t.Setenv("UV_PROJECT_ENVIRONMENT", venv)

		uv, err := newUv(root, "")
		require.NoError(t, err)
		require.Equal(t, venv, uv.virtualenvPath, "virtualenv is taken from UV_PROJECT_ENVIRONMENT")
	})

	t.Run("relative path resolves against the project directory", func(t *testing.T) {
		root := t.TempDir()
		pulumiRoot := filepath.Join(root, "subfolder")
		require.NoError(t, os.Mkdir(pulumiRoot, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "uv.lock"), []byte{}, 0o600))
		t.Setenv("UV_PROJECT_ENVIRONMENT", "custom-env")

		uv, err := newUv(pulumiRoot, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, "custom-env"), uv.virtualenvPath,
			"relative UV_PROJECT_ENVIRONMENT is resolved next to uv.lock")
	})

	t.Run("virtualenv option takes precedence", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("UV_PROJECT_ENVIRONMENT", filepath.Join(t.TempDir(), "custom-env"))

		uv, err := newUv(root, "banana")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, "banana"), uv.virtualenvPath,
			"the virtualenv runtime option wins over UV_PROJECT_ENVIRONMENT")
	})

	t.Run("empty value is ignored", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("UV_PROJECT_ENVIRONMENT", "")

		uv, err := newUv(root, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, ".venv"), uv.virtualenvPath, "virtualenv is in the project root")
	})
}

func TestUvVersion(t *testing.T) {
	t.Parallel()

	for _, versionString := range []string{
		"uv 0.6.15",
		"uv 0.6.15 (Homebrew 2024-10-23)",
		"uv 0.6.15 (d2cd09bbd 2024-10-25)",
	} {
		v, err := ParseUvVersion(versionString)
		require.NoError(t, err)
		require.Equal(t, semver.MustParse("0.6.15"), v)
	}

	_, err := ParseUvVersion("uv 0.6.14")
	require.ErrorContains(t, err, "less than the minimum required version")
}

func TestUvCommandSyncsEnvironment(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pyproject, err := os.ReadFile(filepath.Join("testdata", "project", "pyproject.toml"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(root, "pyproject.toml"), pyproject, 0o600)
	require.NoError(t, err)

	uv, err := newUv(root, "")
	require.NoError(t, err)

	// Run a python command, this should run `uv sync` as side effect
	cmd, err := uv.Command(t.Context(), "-c", "print('hello')")
	require.NoError(t, err)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.Equal(t, "hello", strings.TrimSpace(string(out)))

	// check that .venv exists
	require.DirExists(t, filepath.Join(root, ".venv"))

	// `wheel`, the project's dependency, should be installed
	cmd, err = uv.ModuleCommand(t.Context(), "wheel", "version")
	require.NoError(t, err)
	out, err = cmd.CombinedOutput()
	require.NoError(t, err)
	require.True(t, strings.Contains(string(out), "wheel"), "unexpected output: %s", out)
}

func TestUvCommandSyncsEnvironmentCustomVenv(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pyproject, err := os.ReadFile(filepath.Join("testdata", "project", "pyproject.toml"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(root, "pyproject.toml"), pyproject, 0o600)
	require.NoError(t, err)

	uv, err := newUv(root, "my_venv")
	require.NoError(t, err)

	// Run a python command, this should run `uv sync` as side effect
	cmd, err := uv.Command(t.Context(), "-c", "print('hello')")
	require.NoError(t, err)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.Equal(t, "hello", strings.TrimSpace(string(out)))

	// check that my_venv exists
	require.DirExists(t, filepath.Join(root, "my_venv"))

	// `wheel`, the project's dependency, should be installed
	cmd, err = uv.ModuleCommand(t.Context(), "wheel", "version")
	require.NoError(t, err)
	out, err = cmd.CombinedOutput()
	require.NoError(t, err)
	require.True(t, strings.Contains(string(out), "wheel"), "unexpected output: %s", out)
}

func TestUvCommandSyncsEnvironmentUvProjectEnvironment(t *testing.T) {
	root := t.TempDir()
	pyproject, err := os.ReadFile(filepath.Join("testdata", "project", "pyproject.toml"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(root, "pyproject.toml"), pyproject, 0o600)
	require.NoError(t, err)

	venv := filepath.Join(t.TempDir(), "custom-env")
	t.Setenv("UV_PROJECT_ENVIRONMENT", venv)

	uv, err := newUv(root, "")
	require.NoError(t, err)

	cmd, err := uv.Command(t.Context(), "-c", "print('hello')")
	require.NoError(t, err)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.Equal(t, "hello", strings.TrimSpace(string(out)))

	require.DirExists(t, venv)
	require.NoDirExists(t, filepath.Join(root, ".venv"))

	cmd, err = uv.ModuleCommand(t.Context(), "wheel", "version")
	require.NoError(t, err)
	out, err = cmd.CombinedOutput()
	require.NoError(t, err)
	require.True(t, strings.Contains(string(out), "wheel"), "unexpected output: %s", out)
}

// Test that we show the underlying error from `uv` when linking fails
func TestUvLinkPackagesError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pyproject := `[project]
name = "my-project"
version = "1.2.3"
dependencies = []
`
	err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0o600)
	require.NoError(t, err)
	uv, err := newUv(root, "")
	require.NoError(t, err)

	err = uv.LinkPackages(t.Context(), map[string]string{"nope": "." + string(filepath.Separator) + "nope"})

	require.Regexp(t, "Distribution not found at:.*nope", err.Error())
}

// Test that we show the underlying error from `uv` when dependency installation fails
func TestUvInstallDependenciesError(t *testing.T) {
	t.Parallel()

	pyproject := `[project]
name = "my-project"
version = "1.2.3"
dependencies = ["fail-to-install"]
[tool.uv.sources]
fail-to-install = { path = "./fail-to-install" }
`

	t.Run("show output false", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0o600)
		require.NoError(t, err)
		uv, err := newUv(root, "")
		require.NoError(t, err)

		err = uv.InstallDependencies(t.Context(), root, false, false /* showOutput */, nil, nil)
		require.Regexp(t, "Distribution not found at:.*fail-to-install", err.Error())
	})

	t.Run("show output true", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0o600)
		require.NoError(t, err)
		uv, err := newUv(root, "")
		require.NoError(t, err)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		err = uv.InstallDependencies(t.Context(), root, false, true /* showOutput */, stdout, stderr)

		require.ErrorContains(t, err, "exit status")
		require.Regexp(t, "Distribution not found at:.*fail-to-install", stderr)
	})
}

func TestUvFailsWhenPyprojectMissingProjectSection(t *testing.T) {
	t.Parallel()

	t.Run("InstallDependencies", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		err := os.WriteFile(filepath.Join(root, "pyproject.toml"),
			[]byte("[tool.something]\nvalue = 1\n"), 0o600)
		require.NoError(t, err)
		uv, err := newUv(root, "")
		require.NoError(t, err)

		err = uv.InstallDependencies(t.Context(), root, false, false, nil, nil)
		require.ErrorContains(t, err, "missing a [project] section")
	})

	t.Run("Command", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		err := os.WriteFile(filepath.Join(root, "pyproject.toml"),
			[]byte("[tool.something]\nvalue = 1\n"), 0o600)
		require.NoError(t, err)
		uv, err := newUv(root, "")
		require.NoError(t, err)

		_, err = uv.Command(t.Context(), "-c", "print('hello')")
		require.ErrorContains(t, err, "missing a [project] section")
	})
}

func TestUvMergeRequirements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// pyproject.toml without [project] section
	pyproject := `[tool.pylint."MESSAGES CONTROL"]
max-line-length = 120
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("wheel\n"), 0o600))

	uv, err := newUv(root, "")
	require.NoError(t, err)

	err = uv.InstallDependencies(t.Context(), root, false, false, nil, nil)
	require.NoError(t, err)

	require.NoFileExists(t, filepath.Join(root, "requirements.txt"), "requirements.txt should be deleted")
	py, err := LoadPyproject(root)
	require.NoError(t, err)
	require.NotNil(t, py.Project)
	hasWheel := false
	for _, dep := range py.Project.Dependencies {
		if strings.HasPrefix(dep, "wheel") {
			hasWheel = true
			break
		}
	}
	require.True(t, hasWheel, "expected wheel in dependencies, got %v", py.Project.Dependencies)

	contents, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	require.NoError(t, err)
	require.Contains(t, string(contents), "[tool.pylint", "expected [tool.pylint] section to still be present")
}

// Test that link actually runs in the root directory.
func TestUvLinkCorrectDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pyproject := `[project]
name = "my-project"
version = "invalid"
dependencies = []
`
	err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0o600)
	require.NoError(t, err)
	uv, err := newUv(root, "")
	require.NoError(t, err)

	err = uv.LinkPackages(t.Context(), map[string]string{"nope": "." + string(filepath.Separator) + "nope"})

	require.ErrorContains(t, err, "expected version to start with a number, but no leading ASCII digits were found")
}

func TestUvExportWorkspacePackages(t *testing.T) {
	t.Parallel()

	list := func(t *testing.T, u Toolchain, transitive bool) map[string]string {
		t.Helper()
		packages, err := u.ListPackages(t.Context(), transitive)
		require.NoError(t, err)
		versions := map[string]string{}
		for _, pkg := range packages {
			require.NotContains(t, versions, pkg.Name, "only one version should be reported")
			versions[pkg.Name] = pkg.Version
		}
		return versions
	}

	t.Run("exclude installed sibling dependencies", func(t *testing.T) {
		t.Parallel()
		root, memberA := newUvExportTestWorkspace(t)

		// Both providers are installed, but each member should only report its own dependencies.
		cmd, err := memberA.Command(t.Context(), "-c",
			"import importlib.metadata; print(importlib.metadata.version('pulumi-sibling'))")
		require.NoError(t, err)
		out, err := cmd.Output()
		require.NoError(t, err)
		require.Equal(t, "1.0.0", strings.TrimSpace(string(out)))

		packages := list(t, memberA, true)
		require.Len(t, packages, 7)
		require.Contains(t, packages, "packaging")
		require.Contains(t, packages, "pulumi-alpha")
		require.NotContains(t, packages, "pulumi-sibling")

		memberB, err := newUv(filepath.Join(root, "b"), "")
		require.NoError(t, err)
		require.Equal(t, map[string]string{"pulumi-sibling": "1.0.0"}, list(t, memberB, true))
	})

	t.Run("separate virtual environments", func(t *testing.T) {
		t.Parallel()
		// Use the virtualenv runtime option to install each member's dependencies into its own a/.venv or b/.venv
		// directory.
		root := writeUvExportTestWorkspace(t)
		members := make(map[string]Toolchain)
		for _, name := range []string{"a", "b"} {
			dir := filepath.Join(root, name)
			member, err := ResolveToolchain(PythonOptions{
				Toolchain: Uv, Root: dir, ProgramDir: dir, Virtualenv: ".venv",
			})
			require.NoError(t, err)
			require.NoError(t, member.InstallDependencies(t.Context(), dir, false, false, nil, nil))
			cmd, err := member.Command(t.Context(), "-c", "import sys; print(sys.prefix)")
			require.NoError(t, err)
			out, err := cmd.Output()
			require.NoError(t, err)
			venv, err := filepath.EvalSymlinks(filepath.Join(dir, ".venv"))
			require.NoError(t, err)
			prefix, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
			require.NoError(t, err)
			require.Equal(t, venv, prefix)
			members[name] = member
		}
		require.NoDirExists(t, filepath.Join(root, ".venv"), "there is no shared environment")

		packages := list(t, members["a"], true)
		require.Len(t, packages, 7)
		require.Equal(t, "1.0.0", packages["pulumi-alpha"])
		require.Equal(t, "4.5.6", packages["pulumi-editable"])
		require.NotContains(t, packages, "pulumi-sibling")

		require.Equal(t, map[string]string{"pulumi-sibling": "1.0.0"}, list(t, members["b"], true))
	})

	t.Run("include transitive extras", func(t *testing.T) {
		t.Parallel()
		_, member := newUvExportTestWorkspace(t)

		packages := list(t, member, true)
		require.Equal(t, "1.0.0", packages["pulumi-alpha"])
		require.Equal(t, "1.0.0", packages["pulumi-extra"])
	})

	t.Run("use configured default groups", func(t *testing.T) {
		t.Parallel()
		_, member := newUvExportTestWorkspace(t)

		packages := list(t, member, true)
		require.Equal(t, "1.0.0", packages["pulumi-tools"])
		require.NotContains(t, packages, "pulumi-dev")
	})

	t.Run("evaluate markers", func(t *testing.T) {
		t.Parallel()
		_, member := newUvExportTestWorkspace(t)
		cmd, err := member.Command(t.Context(), "-c",
			"import sys; print('1.0.0' if sys.version_info < (3, 12) else '2.0.0')")
		require.NoError(t, err)
		out, err := cmd.Output()
		require.NoError(t, err)

		packages := list(t, member, true)
		require.Equal(t, strings.TrimSpace(string(out)), packages["pulumi-conditional"])
		require.NotContains(t, packages, "pulumi-inactive")
	})

	t.Run("preserve local package versions", func(t *testing.T) {
		t.Parallel()
		_, member := newUvExportTestWorkspace(t)

		packages := list(t, member, true)
		require.Equal(t, "1.2.3", packages["pulumi-wheel"])
		require.Equal(t, "4.5.6", packages["pulumi-editable"])
	})

	t.Run("direct dependencies belong to the member", func(t *testing.T) {
		t.Parallel()
		_, member := newUvExportTestWorkspace(t)

		packages := list(t, member, false)
		require.Equal(t, "1.0.0", packages["pulumi-alpha"])
		require.NotContains(t, packages, "pulumi-extra")
		require.NotContains(t, packages, "pulumi-sibling")
	})

	t.Run("leave the lockfile unchanged", func(t *testing.T) {
		t.Parallel()
		root, member := newUvExportTestWorkspace(t)
		lockPath := filepath.Join(root, "uv.lock")
		before, err := os.ReadFile(lockPath)
		require.NoError(t, err)

		list(t, member, true)

		after, err := os.ReadFile(lockPath)
		require.NoError(t, err)
		require.Equal(t, before, after)
	})

	t.Run("reject a stale lockfile without rewriting it", func(t *testing.T) {
		t.Parallel()
		root, _ := newUvExportTestWorkspace(t)
		lockPath := filepath.Join(root, "uv.lock")
		before, err := os.ReadFile(lockPath)
		require.NoError(t, err)
		projectPath := filepath.Join(root, "b", "pyproject.toml")
		project, err := os.ReadFile(projectPath)
		require.NoError(t, err)
		project = bytes.ReplaceAll(project, []byte(`["pulumi-sibling"]`), []byte(`["pulumi-sibling", "pulumi-dev"]`))
		require.NoError(t, os.WriteFile(projectPath, project, 0o600))
		member, err := newUv(filepath.Join(root, "b"), "")
		require.NoError(t, err)

		_, err = member.ListPackages(t.Context(), true)

		require.ErrorContains(t, err, "exporting uv dependencies")
		after, err := os.ReadFile(lockPath)
		require.NoError(t, err)
		require.Equal(t, before, after)
	})
}

// newUvExportTestWorkspace creates the fixture, runs uv sync in B, then uv sync --inexact
// in A, and returns the workspace root and a toolchain for A's program directory.
func newUvExportTestWorkspace(t *testing.T) (string, *uv) {
	t.Helper()
	root := writeUvExportTestWorkspace(t)
	for _, args := range [][]string{{"b", "sync"}, {"a", "sync", "--inexact"}} {
		cmd := exec.CommandContext(t.Context(), "uv", args[1:]...)
		cmd.Dir = filepath.Join(root, args[0])
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	member, err := newUv(filepath.Join(root, "a", "program"), "")
	require.NoError(t, err)
	return root, member
}

// writeUvExportTestWorkspace writes out the workspace fixture using synthetic test packages:
//
//	workspace/
//	├── pyproject.toml
//	├── uv.lock
//	├── a/
//	│   ├── pyproject.toml
//	│   └── program/__main__.py
//	├── b/
//	│   └── pyproject.toml
//	└── editable/
//	    ├── pyproject.toml
//	    └── pulumi_editable/__init__.py
//
//	workspace: no dependencies
//	member-a:
//	  packaging >= 26.0                   evaluates environment markers, a dependency of the core SDK in a real setup
//	  pulumi-alpha[providers]             pulls in pulumi-extra via the providers extra
//	  pulumi-wheel == 1.2.3               local wheel
//	  pulumi-editable                     local editable package, version 4.5.6
//	  pulumi-conditional == 1.0.0         Python < 3.12
//	  pulumi-conditional == 2.0.0         Python >= 3.12
//	  pulumi-inactive                     sys_platform == 'never'; never installed
//	  tools group: pulumi-tools           enabled by member A's default-groups = ["tools"]
//	  dev group: pulumi-dev               excluded by that override; uv normally enables dev
//	member-b: pulumi-sibling
//	pulumi-editable: no runtime dependencies
func writeUvExportTestWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		path = filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	wheel := func(name, version, metadata string) string {
		t.Helper()
		name = strings.ReplaceAll(name, "-", "_")
		path := filepath.Join(root, fmt.Sprintf("%s-%s-py3-none-any.whl", name, version))
		file, err := os.Create(path)
		require.NoError(t, err)
		archive := zip.NewWriter(file)
		for suffix, content := range map[string]string{
			"METADATA": fmt.Sprintf("Metadata-Version: 2.1\nName: %s\nVersion: %s\n%s", name, version, metadata),
			"WHEEL":    "Wheel-Version: 1.0\nRoot-Is-Purelib: true\nTag: py3-none-any\n",
			"RECORD":   "",
		} {
			entry, err := archive.Create(fmt.Sprintf("%s-%s.dist-info/%s", name, version, suffix))
			require.NoError(t, err)
			_, err = entry.Write([]byte(content))
			require.NoError(t, err)
		}
		require.NoError(t, archive.Close())
		require.NoError(t, file.Close())
		return filepath.Base(path)
	}
	wheel("pulumi-alpha", "1.0.0", "Provides-Extra: providers\nRequires-Dist: pulumi-extra; extra == 'providers'\n")
	for _, name := range []string{"pulumi-extra", "pulumi-sibling", "pulumi-dev", "pulumi-tools", "pulumi-inactive"} {
		wheel(name, "1.0.0", "")
	}
	wheel("pulumi-conditional", "1.0.0", "")
	wheel("pulumi-conditional", "2.0.0", "")
	directWheel := wheel("pulumi-wheel", "1.2.3", "")
	write("pyproject.toml", `[project]
name = "workspace"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = []
[tool.uv.workspace]
members = ["a", "b"]
[tool.uv]
find-links = ["."]
`)
	write("a/pyproject.toml", fmt.Sprintf(`[project]
name = "member-a"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = [
    "packaging>=26.0", "pulumi-alpha[providers]", "pulumi-wheel", "pulumi-editable",
    "pulumi-conditional==1.0.0; python_version < '3.12'",
    "pulumi-conditional==2.0.0; python_version >= '3.12'",
    "pulumi-inactive; sys_platform == 'never'",
]
[dependency-groups]
dev = ["pulumi-dev"]
tools = ["pulumi-tools"]
[tool.uv]
default-groups = ["tools"]
[tool.uv.sources]
pulumi-wheel = {path = "../%s"}
pulumi-editable = {path = "../editable", editable = true}
`, directWheel))
	write("b/pyproject.toml", `[project]
name = "member-b"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = ["pulumi-sibling"]
`)
	write("editable/pyproject.toml", `[project]
name = "pulumi-editable"
version = "4.5.6"
[build-system]
requires = ["setuptools>=61.0"]
build-backend = "setuptools.build_meta"
`)
	write("editable/pulumi_editable/__init__.py", "")
	write("a/program/__main__.py", "")
	return root
}
