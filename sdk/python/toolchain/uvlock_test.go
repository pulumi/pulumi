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

package toolchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

// workspaceLock is a uv.lock for a workspace with two members. `member-a` depends on
// `pulumi-onepassword`, `member-b` does not. It also exercises extras (`pytest-cov` pulls in
// `coverage[toml]`, which pulls in `tomli`) and dependency groups (`member-b` has a `dev` group).
const workspaceLock = `version = 1
revision = 3
requires-python = ">=3.10"

[[package]]
name = "member-a"
version = "0.1.0"
source = { virtual = "packages/member-a" }
dependencies = [
    { name = "pulumi-onepassword" },
]

[[package]]
name = "member-b"
version = "0.1.0"
source = { virtual = "packages/member-b" }
dependencies = [
    { name = "pulumi-random" },
    { name = "pytest-cov" },
]

[package.dev-dependencies]
dev = [
    { name = "pulumi-docker" },
]
lint = [
    { name = "ruff" },
]

[[package]]
name = "workspace-root"
version = "0.1.0"
source = { virtual = "." }

[[package]]
name = "pulumi"
version = "3.264.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "pulumi-onepassword"
version = "1.1.3"
source = { registry = "https://pypi.org/simple" }
dependencies = [
    { name = "pulumi" },
]

[[package]]
name = "pulumi-random"
version = "4.16.7"
source = { registry = "https://pypi.org/simple" }
dependencies = [
    { name = "pulumi" },
]

[[package]]
name = "pulumi-docker"
version = "4.5.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "ruff"
version = "0.15.22"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "pytest-cov"
version = "7.1.0"
source = { registry = "https://pypi.org/simple" }
dependencies = [
    { name = "coverage", extra = ["toml"] },
]

[[package]]
name = "coverage"
version = "7.15.2"
source = { registry = "https://pypi.org/simple" }

[package.optional-dependencies]
toml = [
    { name = "tomli" },
]

[[package]]
name = "tomli"
version = "2.2.1"
source = { registry = "https://pypi.org/simple" }
`

// writeUvWorkspace lays out the workspace described by workspaceLock on disk and returns its root.
func writeUvWorkspace(t *testing.T, memberPyproject map[string]string) string {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "uv.lock"), []byte(workspaceLock), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(`[project]
name = "workspace-root"
version = "0.1.0"
requires-python = ">=3.10"

[tool.uv.workspace]
members = ["packages/*"]
`), 0o600))

	for member, pyproject := range memberPyproject {
		dir := filepath.Join(root, "packages", member)
		require.NoError(t, os.MkdirAll(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0o600))
	}

	return root
}

const memberAPyproject = `[project]
name = "member-a"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = ["pulumi-onepassword"]
`

const memberBPyproject = `[project]
name = "member-b"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = ["pulumi-random", "pytest-cov"]

[dependency-groups]
dev = ["pulumi-docker"]
lint = ["ruff"]
`

func packageNames(packages []plugin.DependencyInfo) []string {
	names := make([]string, len(packages))
	for i, pkg := range packages {
		names[i] = pkg.Name
	}
	return names
}

// TestUvListPackagesScopedToWorkspaceMember is a regression test for
// https://github.com/pulumi/pulumi/issues/24014: in a uv workspace, `uv.lock` holds the union of
// every member's dependencies, but `uv sync` only installs the current member's. Reporting the
// whole lock file made Pulumi try to install plugins for packages that are not in the virtualenv,
// which fails for plugins that need the `pulumi-plugin.json` metadata of the installed package
// (for example a custom `server`).
func TestUvListPackagesScopedToWorkspaceMember(t *testing.T) {
	t.Parallel()

	root := writeUvWorkspace(t, map[string]string{
		"member-a": memberAPyproject,
		"member-b": memberBPyproject,
	})

	uv, err := newUv(filepath.Join(root, "packages", "member-b"), "")
	require.NoError(t, err)

	packages, err := uv.ListPackages(t.Context(), true /* transitive */)
	require.NoError(t, err)

	names := packageNames(packages)
	// member-a's dependency must not show up when operating on member-b.
	require.NotContains(t, names, "pulumi-onepassword")
	// Neither member is installed into the virtualenv as a distribution.
	require.NotContains(t, names, "member-a")
	require.NotContains(t, names, "member-b")
	require.NotContains(t, names, "workspace-root")
	// member-b's own runtime, transitive and default dependency-group packages are reported.
	require.Contains(t, names, "pulumi-random")
	require.Contains(t, names, "pulumi")
	require.Contains(t, names, "pytest-cov")
	require.Contains(t, names, "pulumi-docker")
	// `coverage[toml]` must pull in the packages of coverage's `toml` extra.
	require.Contains(t, names, "coverage")
	require.Contains(t, names, "tomli")
	// `lint` is not a default group, so `uv sync` does not install it.
	require.NotContains(t, names, "ruff")

	for _, pkg := range packages {
		require.NotEmpty(t, pkg.Version, "no version reported for %s", pkg.Name)
	}
}

// TestUvListPackagesDirectDependenciesScopedToWorkspaceMember checks that the non-transitive
// listing is scoped to the current member too. Before the fix this read the [project] section of
// the pyproject.toml next to uv.lock, which in a workspace belongs to the workspace root rather
// than to the project being operated on.
func TestUvListPackagesDirectDependenciesScopedToWorkspaceMember(t *testing.T) {
	t.Parallel()

	root := writeUvWorkspace(t, map[string]string{
		"member-a": memberAPyproject,
		"member-b": memberBPyproject,
	})

	uv, err := newUv(filepath.Join(root, "packages", "member-b"), "")
	require.NoError(t, err)

	packages, err := uv.ListPackages(t.Context(), false /* transitive */)
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"pulumi-random", "pytest-cov", "pulumi-docker"}, packageNames(packages))
}

// TestUvListPackagesDefaultGroups checks that we follow `tool.uv.default-groups`, which controls
// the dependency groups `uv sync` installs.
func TestUvListPackagesDefaultGroups(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		defaultGroups string
		expected      []string
	}{
		{name: "implicit dev group", defaultGroups: "", expected: []string{"pulumi-docker"}},
		{name: "no groups", defaultGroups: `default-groups = []`, expected: nil},
		{name: "explicit groups", defaultGroups: `default-groups = ["lint"]`, expected: []string{"ruff"}},
		{name: "all groups", defaultGroups: `default-groups = "all"`, expected: []string{"pulumi-docker", "ruff"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pyproject := memberBPyproject
			if tt.defaultGroups != "" {
				pyproject += "\n[tool.uv]\n" + tt.defaultGroups + "\n"
			}
			root := writeUvWorkspace(t, map[string]string{"member-b": pyproject})

			uv, err := newUv(filepath.Join(root, "packages", "member-b"), "")
			require.NoError(t, err)

			packages, err := uv.ListPackages(t.Context(), true /* transitive */)
			require.NoError(t, err)

			names := packageNames(packages)
			for _, group := range []string{"pulumi-docker", "ruff"} {
				if slices.Contains(tt.expected, group) {
					require.Contains(t, names, group)
				} else {
					require.NotContains(t, names, group)
				}
			}
			// Runtime dependencies are always reported.
			require.Contains(t, names, "pulumi-random")
		})
	}
}

// TestUvListPackagesFallsBackToWholeLock checks that we keep reporting every package in the lock
// file when we cannot tell which project the lock belongs to, rather than reporting nothing.
func TestUvListPackagesFallsBackToWholeLock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	lock := `version = 1
revision = 3
requires-python = ">=3.10"

[[package]]
name = "somewhere-else"
version = "0.1.0"
source = { virtual = "somewhere/else" }
dependencies = [
    { name = "pulumi-random" },
]

[[package]]
name = "pulumi-random"
version = "4.16.7"
source = { registry = "https://pypi.org/simple" }
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "uv.lock"), []byte(lock), 0o600))

	uv, err := newUv(root, "")
	require.NoError(t, err)

	packages, err := uv.ListPackages(t.Context(), true /* transitive */)
	require.NoError(t, err)

	names := packageNames(packages)
	require.Contains(t, names, "pulumi-random")
	require.NotContains(t, names, "somewhere-else")
}

// TestUvListPackagesRealWorkspace is an end-to-end version of
// TestUvListPackagesScopedToWorkspaceMember that has `uv` itself produce the lock file, so that we
// notice if the shape of uv.lock changes. The workspace only depends on packages within itself, so
// resolving it needs no network access.
func TestUvListPackagesRealWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile := func(dir, name, content string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	writeFile(root, "pyproject.toml", `[project]
name = "workspace-root"
version = "0.1.0"
requires-python = ">=3.9"
dependencies = []

[tool.uv.workspace]
members = ["packages/*"]
`)

	// Two buildable packages standing in for provider SDKs, so that they show up in the lock file
	// as real distributions rather than as virtual workspace members.
	for _, dep := range []string{"pulumi-onepassword", "pulumi-random"} {
		dir := filepath.Join(root, "packages", dep)
		writeFile(dir, "pyproject.toml", `[project]
name = "`+dep+`"
version = "1.0.0"
requires-python = ">=3.9"
dependencies = []

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"
`)
		writeFile(filepath.Join(dir, "src", strings.ReplaceAll(dep, "-", "_")), "__init__.py", "")
	}

	for member, dep := range map[string]string{"member-a": "pulumi-onepassword", "member-b": "pulumi-random"} {
		writeFile(filepath.Join(root, "packages", member), "pyproject.toml", `[project]
name = "`+member+`"
version = "0.1.0"
requires-python = ">=3.9"
dependencies = ["`+dep+`"]

[tool.uv.sources]
`+dep+` = { workspace = true }
`)
	}

	cmd := exec.Command("uv", "lock", "--offline")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	for member, expected := range map[string]string{"member-a": "pulumi-onepassword", "member-b": "pulumi-random"} {
		uv, err := newUv(filepath.Join(root, "packages", member), "")
		require.NoError(t, err)

		packages, err := uv.ListPackages(t.Context(), true /* transitive */)
		require.NoError(t, err)

		names := packageNames(packages)
		require.Contains(t, names, expected)
		for _, other := range []string{"pulumi-onepassword", "pulumi-random"} {
			if other != expected {
				require.NotContains(t, names, other, "%s should not see %s", member, other)
			}
		}
	}
}
