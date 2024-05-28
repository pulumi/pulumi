package toolchain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateVenvPip(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	tc, err := newPip(tmp, "venv")
	require.NoError(t, err)
	err = tc.ValidateVenv(context.Background())
	require.ErrorContains(t, err, "he 'virtualenv' option in Pulumi.yaml is set to \"venv\"")
}

func TestValidateVenvPoetry(t *testing.T) {
	t.Parallel()
	// TODO: @julienp
	t.Skip("Poetry is not supported yet")
	tmp := t.TempDir()
	tc, err := newPoetry(tmp)
	require.NoError(t, err)
	err = tc.ValidateVenv(context.Background())
	require.Error(t, err)
	// TODO: check error message
	// require.ErrorContains(t, err, "he 'virtualenv' option in Pulumi.yaml is set to \"venv\"")
}

func TestListPackages(t *testing.T) {
	t.Parallel()

	// TODO: @julienp add poetry to CI
	for _, tc := range []toolchain{Pip /*, Poetry*/} {
		tc := tc
		t.Run("empty/"+Name(tc), func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			createVenv(t, tc, tmp)

			tc, err := ResolveToolchain(PythonOptions{
				Toolchain:  tc,
				Virtualenv: filepath.Join(tmp, "venv"),
				Root:       tmp,
			})
			require.NoError(t, err)

			packages, err := tc.ListPackages(context.Background(), false)
			require.NoError(t, err)
			require.Len(t, packages, 1)
			require.Equal(t, "pip", packages[0].Name)
		})

		t.Run("non-empty/"+Name(tc), func(t *testing.T) {
			t.Parallel()
			tmp := t.TempDir()
			createVenv(t, tc, tmp, "pulumi-random")

			tc, err := ResolveToolchain(PythonOptions{
				Toolchain:  tc,
				Virtualenv: filepath.Join(tmp, "venv"),
				Root:       tmp,
			})
			require.NoError(t, err)

			packages, err := tc.ListPackages(context.Background(), false)
			sort.Slice(packages, func(i, j int) bool {
				return packages[i].Name < packages[j].Name
			})
			require.NoError(t, err)
			require.Len(t, packages, 2)
			require.Equal(t, "pip", packages[0].Name)
			require.Equal(t, "pulumi_random", packages[1].Name)
		})
	}
}

func createVenv(t *testing.T, toolchain toolchain, dir string, packages ...string) {
	t.Helper()

	if toolchain == Pip {
		tc, err := ResolveToolchain(PythonOptions{
			Toolchain: Pip,
		})
		require.NoError(t, err)
		cmd, err := tc.Command(context.Background(), "-m", "venv", filepath.Join(dir, "venv"))
		require.NoError(t, err)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		for _, pkg := range packages {
			tc, err := ResolveToolchain(PythonOptions{
				Toolchain:  Pip,
				Virtualenv: filepath.Join(dir, "venv"),
			})
			require.NoError(t, err)
			cmd, err := tc.Command(context.Background(), "-m", "pip", "install", pkg)
			require.NoError(t, err)
			cmd.Dir = dir
			require.NoError(t, cmd.Run())
		}
	} else if toolchain == Poetry {
		tc, err := ResolveToolchain(PythonOptions{
			Toolchain: Poetry,
			Root:      dir,
		})
		require.NoError(t, err)

		f, err := os.OpenFile(filepath.Join(dir, "pyproject.toml"), os.O_CREATE|os.O_WRONLY, 0o600)
		require.NoError(t, err)

		fmt.Printf("tmp: %s\n", dir)

		fmt.Fprint(f, `
[build-system]
requires = ["poetry-core"]
build-backend = "poetry.core.masonry.api"

[tool.poetry]
name = "test_pulumi_venv"
version = "0.1.0"
description = ""
authors = []
readme = "README.md"
package-mode = false
packages = [{include = "test_pulumi_venv"}]

[tool.poetry.dependencies]
python = "^3.8"
`)

		for _, pkg := range packages {
			fmt.Fprintf(f, "%s = \"*\"\n", pkg)
		}
		err = f.Close()
		require.NoError(t, err)

		err = tc.InstallDependenciesWithWriters(context.Background(), dir, true, os.Stdout, os.Stderr)
		require.NoError(t, err)
	}
}
