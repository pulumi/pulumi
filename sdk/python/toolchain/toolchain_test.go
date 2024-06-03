package toolchain

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateVenv(t *testing.T) {
	t.Parallel()

	for _, opts := range []PythonOptions{
		{
			Toolchain:  Pip,
			Virtualenv: "venv",
		},
		{
			Toolchain: Poetry,
		},
	} {
		opts := opts
		t.Run("Doesnt-exist-"+Name(opts.Toolchain), func(t *testing.T) {
			t.Parallel()
			opts := copyOptions(opts)

			opts.Root = t.TempDir()
			tc, err := ResolveToolchain(opts)
			require.NoError(t, err)

			err = tc.ValidateVenv(context.Background())
			require.Error(t, err)
		})
		t.Run("Exists-"+Name(opts.Toolchain), func(t *testing.T) {
			t.Parallel()
			opts := copyOptions(opts)
			opts.Root = t.TempDir()
			createVenv(t, opts)

			tc, err := ResolveToolchain(opts)
			require.NoError(t, err)
			err = tc.InstallDependencies(context.Background(), opts.Root, true, os.Stdout, os.Stderr)
			require.NoError(t, err)
			err = tc.ValidateVenv(context.Background())
			require.NoError(t, err)
		})
	}
}

//nolint:paralleltest // modifies environment variables
func TestCommand(t *testing.T) {
	// Poetry with `in-project = true` uses `.venv` as the default virtualenv directory.
	// Use the same for pip to keep the tests consistent.
	venvDir := ".venv"

	for _, opts := range []PythonOptions{
		{
			Toolchain:  Pip,
			Virtualenv: venvDir,
		},
		{
			Toolchain: Poetry,
		},
	} {
		opts := opts
		t.Run("empty/"+Name(opts.Toolchain), func(t *testing.T) {
			opts := copyOptions(opts)
			opts.Root = t.TempDir()
			createVenv(t, opts)

			t.Setenv("MY_ENV_VAR", "HELLO")

			tc, err := ResolveToolchain(opts)
			require.NoError(t, err)

			cmd, err := tc.Command(context.Background())
			require.NoError(t, err)

			var venvBin string
			if runtime.GOOS == windows {
				venvBin = filepath.Join(opts.Root, venvDir, "Scripts")
				require.Equal(t, filepath.Join("python.exe"), cmd.Path)
			} else {
				venvBin = filepath.Join(opts.Root, venvDir, "bin")
				cmdPath := normalizePath(t, cmd.Path)
				require.Equal(t, filepath.Join(normalizePath(t, venvBin), "python"), cmdPath)
			}

			foundPath := false
			foundMyEnvVar := false
			for _, env := range cmd.Env {
				if strings.HasPrefix(env, "PATH=") {
					require.Contains(t, env, venvBin, "venv binary directory should in PATH")
					foundPath = true
				}
				if strings.HasPrefix(env, "MY_ENV_VAR=") {
					require.Equal(t, "MY_ENV_VAR=HELLO", env, "Env variables should be passed through")
					foundMyEnvVar = true
				}
			}
			require.True(t, foundPath, "PATH was not found in cmd.Env")
			require.True(t, foundMyEnvVar, "MY_ENV_VAR was not found in cmd.Env")
		})
	}
}

func TestListPackages(t *testing.T) {
	t.Parallel()

	for _, opts := range []PythonOptions{
		{
			Toolchain:  Pip,
			Virtualenv: "venv",
		},
		{
			Toolchain: Poetry,
		},
	} {
		opts := opts

		t.Run("empty/"+Name(opts.Toolchain), func(t *testing.T) {
			t.Parallel()
			opts := copyOptions(opts)
			opts.Root = t.TempDir()
			createVenv(t, opts)

			tc, err := ResolveToolchain(opts)
			require.NoError(t, err)

			packages, err := tc.ListPackages(context.Background(), false)
			require.NoError(t, err)
			require.Len(t, packages, 3)
			require.Equal(t, "pip", packages[0].Name)
			require.Equal(t, "setuptools", packages[1].Name)
			require.Equal(t, "wheel", packages[2].Name)
		})

		t.Run("non-empty/"+Name(opts.Toolchain), func(t *testing.T) {
			t.Parallel()
			opts := copyOptions(opts)
			opts.Root = t.TempDir()
			createVenv(t, opts, "pulumi-random")

			tc, err := ResolveToolchain(opts)
			require.NoError(t, err)

			packages, err := tc.ListPackages(context.Background(), false)
			sort.Slice(packages, func(i, j int) bool {
				return packages[i].Name < packages[j].Name
			})
			require.NoError(t, err)
			require.Len(t, packages, 4)
			require.Equal(t, "pip", packages[0].Name)
			require.Equal(t, "pulumi_random", packages[1].Name)
			require.Equal(t, "setuptools", packages[2].Name)
			require.Equal(t, "wheel", packages[3].Name)
		})
	}
}

func TestAbout(t *testing.T) {
	t.Parallel()

	for _, opts := range []PythonOptions{
		{
			Toolchain:  Pip,
			Virtualenv: "venv",
		},
		{
			Toolchain: Poetry,
		},
	} {
		opts := opts
		t.Run(Name(opts.Toolchain), func(t *testing.T) {
			t.Parallel()
			opts := copyOptions(opts)
			opts.Root = t.TempDir()
			createVenv(t, opts)

			tc, err := ResolveToolchain(opts)
			require.NoError(t, err)
			info, err := tc.About(context.Background())
			require.NoError(t, err)
			require.Regexp(t, "[0-9]+\\.[0-9]+\\.[0-9]+", info.Version)
			require.Regexp(t, "python$", info.Executable)
		})
	}
}

func createVenv(t *testing.T, opts PythonOptions, packages ...string) {
	t.Helper()

	if opts.Toolchain == Pip {
		tc, err := ResolveToolchain(opts)
		require.NoError(t, err)
		err = tc.InstallDependencies(context.Background(), opts.Root, true, os.Stdout, os.Stderr)
		require.NoError(t, err)

		for _, pkg := range packages {
			cmd, err := tc.Command(context.Background(), "-m", "pip", "install", pkg)
			require.NoError(t, err)
			require.NoError(t, cmd.Run())
		}
	} else if opts.Toolchain == Poetry {
		writePyproject(t, opts)
		// Write poetry.toml file to enable in-project virtualenvs. This ensures we delete the
		// virtualenv with the tmp directory after the test is done.
		writePoetryToml(t, opts.Root)
		tc, err := ResolveToolchain(opts)
		require.NoError(t, err)
		err = tc.InstallDependencies(context.Background(), opts.Root, true, os.Stdout, os.Stderr)
		require.NoError(t, err)

		for _, pkg := range packages {
			cmd := exec.Command("poetry", "add", pkg)
			cmd.Dir = opts.Root
			err := cmd.Run()
			require.NoError(t, err)
		}
	}
}

func writePyproject(t *testing.T, opts PythonOptions) {
	t.Helper()

	f, err := os.OpenFile(filepath.Join(opts.Root, "pyproject.toml"), os.O_CREATE|os.O_WRONLY, 0o600)
	require.NoError(t, err)
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
pip = "*"
setuptools = "*"
wheel = "*"
`)
	err = f.Close()
	require.NoError(t, err)
}

func writePoetryToml(t *testing.T, path string) {
	t.Helper()

	f, err := os.OpenFile(filepath.Join(path, "poetry.toml"), os.O_CREATE|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	fmt.Fprint(f, `[virtualenvs]
in-project = true
`)
	err = f.Close()
	require.NoError(t, err)
}

func copyOptions(opts PythonOptions) PythonOptions {
	return PythonOptions{
		Root:        opts.Root,
		Virtualenv:  opts.Virtualenv,
		Typechecker: opts.Typechecker,
		Toolchain:   opts.Toolchain,
	}
}

// normalizePath resolves symlinks within the directory part of the given path.
// This helps avoid test issues on macOS where for example /var -> /private/var.
// Importantly, we do not evaluate the symlink of the whole path, as that would
// resolve the python binary to the system python, since within in a virtualenv
// bin/python points to the system python.
func normalizePath(t *testing.T, path string) string {
	t.Helper()
	dir, bin := filepath.Split(path)
	normDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return filepath.Join(normDir, bin)
}
