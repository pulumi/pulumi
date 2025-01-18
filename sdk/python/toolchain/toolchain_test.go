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
	"context"
	"errors"
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
			err = tc.InstallDependencies(context.Background(), opts.Root, false, true, os.Stdout, os.Stderr)
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

	for _, test := range []struct {
		opts             PythonOptions
		expectedPackages []string
	}{
		{
			opts: PythonOptions{
				Toolchain:  Pip,
				Virtualenv: "venv",
			},
			// The virtualenv created by the pip toolchain always has pip
			// installed. Additionally it always installs setuptools and wheel
			// into the virtualenv.
			expectedPackages: []string{"pip", "setuptools", "wheel"},
		},
		{
			opts: PythonOptions{
				Toolchain: Poetry,
			},
			// Virtual environments created by Poetry always include pip.
			expectedPackages: []string{"pip"},
		},
		{
			opts: PythonOptions{
				Toolchain: Uv,
			},
			// Virtual environments created by uv are empty.
			expectedPackages: []string{},
		},
	} {
		test := test

		t.Run("empty/"+Name(test.opts.Toolchain), func(t *testing.T) {
			t.Parallel()
			opts := copyOptions(test.opts)
			opts.Root = t.TempDir()
			createVenv(t, opts)

			tc, err := ResolveToolchain(opts)
			require.NoError(t, err)

			packages, err := tc.ListPackages(context.Background(), false)
			require.NoError(t, err)
			require.Len(t, packages, len(test.expectedPackages))
			for i, pkg := range test.expectedPackages {
				require.Equal(t, pkg, packages[i].Name)
			}
		})

		t.Run("non-empty/"+Name(test.opts.Toolchain), func(t *testing.T) {
			t.Parallel()
			opts := copyOptions(test.opts)
			opts.Root = t.TempDir()
			createVenv(t, opts, "pulumi-random")

			tc, err := ResolveToolchain(opts)
			require.NoError(t, err)

			packages, err := tc.ListPackages(context.Background(), false)
			require.NoError(t, err)
			sort.Slice(packages, func(i, j int) bool {
				return packages[i].Name < packages[j].Name
			})
			// pulumi_random depends on pulumi, which has pip as a dependency.
			// We are looking for packages that are not dependencies of other
			// packages, so we have to exclude pip here.
			var expectedPackages []string
			for _, pkg := range append([]string{"pulumi_random"}, test.expectedPackages...) {
				if pkg != "pip" {
					expectedPackages = append(expectedPackages, pkg)
				}
			}
			sort.Strings(expectedPackages)

			require.Len(t, packages, len(expectedPackages))
			for i, pkg := range expectedPackages {
				require.Equal(t, pkg, packages[i].Name)
			}
		})

		t.Run("non-empty-with-pip/"+Name(test.opts.Toolchain), func(t *testing.T) {
			t.Parallel()
			opts := copyOptions(test.opts)
			opts.Root = t.TempDir()
			createVenv(t, opts, "pulumi-random", "pip")

			tc, err := ResolveToolchain(opts)
			require.NoError(t, err)

			packages, err := tc.ListPackages(context.Background(), false)
			require.NoError(t, err)
			sort.Slice(packages, func(i, j int) bool {
				return packages[i].Name < packages[j].Name
			})

			// pulumi_random depends on pulumi, which has pip as a dependency.
			// We are looking for packages that are not dependencies of other
			// packages, so we have to exclude pip here.
			var expectedPackages []string
			for _, pkg := range append([]string{"pulumi_random"}, test.expectedPackages...) {
				if pkg != "pip" {
					expectedPackages = append(expectedPackages, pkg)
				}
			}
			expectedPackages = unique(expectedPackages)
			sort.Strings(expectedPackages)

			require.Len(t, packages, len(expectedPackages))
			for i, pkg := range expectedPackages {
				require.Equal(t, pkg, packages[i].Name)
			}
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

//nolint:paralleltest // mutates environment variables
func TestPyenv(t *testing.T) {
	if runtime.GOOS == windows {
		t.Skip("pyenv is not supported on Windows")
	}
	tmpDir := t.TempDir()

	// Test without pyenv, a .python-version file
	use, _, _, err := usePyenv(tmpDir)
	require.NoError(t, err)
	require.False(t, use)

	// Add a fake pyenv binary to $tmp/bin and set $PATH to $tmp/bin
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "bin"), 0o755))
	//nolint:gosec // we want this file to be executable
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bin", "pyenv"), []byte("#!/bin/sh\nexit 0;\n"), 0o700))
	t.Setenv("PATH", filepath.Join(tmpDir, "bin"))

	// Test witbout .python-version file
	use, _, _, err = usePyenv(tmpDir)
	require.NoError(t, err)
	require.False(t, use)

	// Create a .python-version file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".python-version"), []byte("3.9.0"), 0o600))

	use, pyenvPath, versionFile, err := usePyenv(tmpDir)
	t.Log("X", use, pyenvPath, versionFile, err)
	require.NoError(t, err)
	require.True(t, use)
	require.Equal(t, filepath.Join(tmpDir, ".python-version"), versionFile)
	require.Equal(t, filepath.Join(tmpDir, "bin", "pyenv"), pyenvPath)
}

//nolint:paralleltest // mutates environment variables
func TestPyenvInstall(t *testing.T) {
	if runtime.GOOS == windows {
		t.Skip("pyenv is not supported on Windows")
	}
	tmpDir := t.TempDir()

	t.Log("tmpDir", tmpDir)

	// Add a fake pyenv binary to $tmp/bin and set $PATH to $tmp/bin.
	// The binary will write its arguments to a file, that we read back later to verify that it was called with the
	// correct arguments.
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "bin"), 0o755))
	outPath := filepath.Join(tmpDir, "out.txt")
	script := fmt.Sprintf("#!/bin/sh\necho $@ > %s\n", outPath)
	//nolint:gosec // we want this file to be executable
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bin", "pyenv"), []byte(script), 0o700))
	t.Setenv("PATH", filepath.Join(tmpDir, "bin"))

	// Create a .python-version file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".python-version"), []byte("3.9.0"), 0o600))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err := installPython(context.Background(), tmpDir, false, stdout, stderr)
	require.NoError(t, err)

	b, err := os.ReadFile(outPath)
	require.NoError(t, err)
	require.Equal(t, "install --skip-existing", strings.TrimSpace(string(b)))
}

func createVenv(t *testing.T, opts PythonOptions, packages ...string) {
	t.Helper()

	if opts.Toolchain == Pip {
		tc, err := ResolveToolchain(opts)
		require.NoError(t, err)
		err = tc.InstallDependencies(context.Background(), opts.Root, false, /*useLanguageVersionTools*/
			true /*showOutput */, os.Stdout, os.Stderr)
		require.NoError(t, err)

		for _, pkg := range packages {
			cmd, err := tc.Command(context.Background(), "-m", "pip", "install", pkg)
			require.NoError(t, err)
			require.NoError(t, cmd.Run())
		}
	} else if opts.Toolchain == Poetry {
		writePyprojectForPoetry(t, opts.Root)
		// Write poetry.toml file to enable in-project virtualenvs. This ensures we delete the
		// virtualenv with the tmp directory after the test is done.
		writePoetryToml(t, opts.Root)
		tc, err := ResolveToolchain(opts)
		require.NoError(t, err)
		err = tc.InstallDependencies(context.Background(), opts.Root, false, /*useLanguageVersionTools*/
			true /*showOutput */, os.Stdout, os.Stderr)
		require.NoError(t, err)

		for _, pkg := range packages {
			cmd := exec.Command("poetry", "add", pkg)
			cmd.Dir = opts.Root
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
		}
	} else if opts.Toolchain == Uv {
		writePyprojectForUv(t, opts.Root)
		tc, err := ResolveToolchain(opts)
		require.NoError(t, err)
		err = tc.InstallDependencies(context.Background(), opts.Root, false, /*useLanguageVersionTools*/
			true /*showOutput */, os.Stdout, os.Stderr)
		require.NoError(t, err)

		for _, pkg := range packages {
			cmd := exec.Command("uv", "add", pkg)
			cmd.Dir = opts.Root
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
		}
	}
}

func writePyprojectForUv(t *testing.T, root string) {
	t.Helper()

	f, err := os.OpenFile(filepath.Join(root, "pyproject.toml"), os.O_CREATE|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	fmt.Fprint(f, `
[project]
name = "list-packages-test"
version = "0.0.1"
requires-python = ">=3.9"
dependencies = []
`)
	err = f.Close()
	require.NoError(t, err)
}

func writePyprojectForPoetry(t *testing.T, root string) {
	t.Helper()

	f, err := os.OpenFile(filepath.Join(root, "pyproject.toml"), os.O_CREATE|os.O_WRONLY, 0o600)
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
python = "^3.9"
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
[virtualenvs.options]
no-setuptools = true
`)
	err = f.Close()
	require.NoError(t, err)
}

func copyOptions(opts PythonOptions) PythonOptions {
	return PythonOptions{
		Root:        opts.Root,
		ProgramDir:  opts.ProgramDir,
		Virtualenv:  opts.Virtualenv,
		Typechecker: opts.Typechecker,
		Toolchain:   opts.Toolchain,
	}
}

func unique(s []string) []string {
	u := make([]string, 0, len(s))
	m := make(map[string]bool)
	for _, val := range s {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}
	return u
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

type ProcessState struct{}

func (p *ProcessState) Pid() int {
	return 123
}

func (p *ProcessState) String() string {
	return "exit status 139 "
}

func TestErrorWithStderr(t *testing.T) {
	t.Parallel()

	err := errors.New("error")
	require.Equal(t, "the error message: error", errorWithStderr(err, "the error message").Error())

	exitErr := &exec.ExitError{ProcessState: &os.ProcessState{}, Stderr: []byte("command said something")}
	require.Equal(t, "the error message: exit status 0: command said something",
		errorWithStderr(exitErr, "the error message").Error())

	exitErrNoStderr := &exec.ExitError{ProcessState: &os.ProcessState{}}
	require.Equal(t, "the error message: exit status 0", errorWithStderr(exitErrNoStderr, "the error message").Error())
}
