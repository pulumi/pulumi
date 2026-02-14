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
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
)

func TestDependenciesFromRequirementsTxt(t *testing.T) {
	t.Parallel()

	b := `
pulumi>=3.0.0,<4.0.0
requests>1

# Comment
setuptools    # comment here

	spaces-before  ==   1.2.3
`
	r := strings.NewReader(b)
	deps, err := dependenciesFromRequirementsTxt(r, ".")
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"pulumi":        ">=3.0.0,<4.0.0",
		"requests":      ">1",
		"python":        "^3.10",
		"setuptools":    "*",
		"spaces-before": "1.2.3",
	}, deps)
}

func TestGeneratePyProjectTOML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p, err := newPoetry(dir)
	require.NoError(t, err)
	deps := map[string]any{
		"pulumi":        ">=3.0.0,<4.0.0",
		"requests":      ">1",
		"setuptools":    "*",
		"spaces-before": "1.2.3",
	}
	s, err := p.generatePyProjectTOML("project-name-here", deps)
	require.NoError(t, err)
	require.Equal(t, `[project]
name = "project-name-here"

[build-system]
requires = ["poetry-core"]
build-backend = "poetry.core.masonry.api"

[tool]
[tool.poetry]
package-mode = false
[tool.poetry.dependencies]
pulumi = ">=3.0.0,<4.0.0"
requests = ">1"
setuptools = "*"
spaces-before = "1.2.3"
`, s)
}

func TestCheckVersion(t *testing.T) {
	t.Parallel()
	version, err := validateVersion("Poetry (version 1.8.3)")
	require.NoError(t, err)
	require.Equal(t, semver.MustParse("1.8.3"), version)

	version, err = validateVersion("Poetry (version 2.1.2)")
	require.NoError(t, err)
	require.Equal(t, semver.MustParse("2.1.2"), version)

	version, err = validateVersion("Poetry (version 3.0)")
	require.NoError(t, err)
	require.Equal(t, semver.MustParse("3.0.0"), version)

	version, err = validateVersion("Poetry (version 1.9.0.dev0)")
	require.NoError(t, err)
	require.Equal(t, semver.MustParse("1.9.0"), version)

	_, err = validateVersion("Poetry (version 1.7.0)")
	require.ErrorContains(t, err, "is less than the minimum required version")

	_, err = validateVersion("invalid version string")
	require.ErrorContains(t, err, "unexpected output from poetry --version")

	_, err = validateVersion("")
	require.ErrorContains(t, err, "unexpected output from poetry --version")
}

// Test that we show the underlying error from `poetry` when linking fails
func TestPoetryLinkPackagesError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pyproject := `name = "my-project"
[tool.poetry]
package-mode = false
[tool.poetry.dependencies]
`
	err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0o600)
	require.NoError(t, err)
	poetry, err := newPoetry(root)
	require.NoError(t, err)

	err = poetry.LinkPackages(t.Context(), map[string]string{"nope": "." + string(filepath.Separator) + "nope"})

	require.Regexp(t, "Could not find a matching version of package .*nope", err.Error())
}

// Test that we show the underlying error from `poetry` when dependency installation fails
func TestPoetryInstallDependenciesError(t *testing.T) {
	t.Parallel()

	pyproject := `name = "my-project"
[tool.poetry]
package-mode = false
[tool.poetry.dependencies]
fail-to-install = {path = "./fail-to-install"}
`

	t.Run("show output false", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0o600)
		require.NoError(t, err)
		poetry, err := newPoetry(root)
		require.NoError(t, err)

		err = poetry.InstallDependencies(t.Context(), root, false, false /* showOutput */, nil, nil)
		require.Regexp(t, "fail-to-install does not exist", err.Error())
	})

	t.Run("show output true", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0o600)
		require.NoError(t, err)
		poetry, err := newPoetry(root)
		require.NoError(t, err)

		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		err = poetry.InstallDependencies(t.Context(), root, false, true /* showOutput */, stdout, stderr)
		require.ErrorContains(t, err, "exit status")
		require.Regexp(t, "fail-to-install does not exist", stderr)
	})
}
