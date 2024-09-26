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
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type typeChecker int

const (
	// TypeCheckerNone is the default typeChecker
	TypeCheckerNone typeChecker = iota
	// TypeCheckerMypy is the mypy typeChecker
	TypeCheckerMypy
	// TypeCheckerPyright is the pyright typeChecker
	TypeCheckerPyright
)

type toolchain int

const (
	Pip toolchain = iota
	Poetry
)

type PythonOptions struct {
	// The root directory of the project.
	Root string
	// The program directory of the project.
	ProgramDir string
	// Virtual environment to use, relative to `Root`.
	Virtualenv string
	// Use a typechecker to type check.
	Typechecker typeChecker
	// The package manager to use for managing dependencies.
	Toolchain toolchain
}

type PythonPackage struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Location string `json:"location"`
}

type Info struct {
	Executable string
	Version    string
}

type Toolchain interface {
	// InstallDependencies installs the dependencies of the project found in `cwd`.
	InstallDependencies(ctx context.Context, cwd string, useLanguageVersionTools,
		showOutput bool, infoWriter, errorWriter io.Writer) error
	// EnsureVenv validates virtual environment of the toolchain and creates it if it doesn't exist.
	EnsureVenv(ctx context.Context, cwd string, useLanguageVersionTools,
		showOutput bool, infoWriter, errorWriter io.Writer) error
	// ValidateVenv checks if the virtual environment of the toolchain is valid.
	ValidateVenv(ctx context.Context) error
	// ListPackages returns a list of Python packages installed in the toolchain.
	ListPackages(ctx context.Context, transitive bool) ([]PythonPackage, error)
	// Command returns an *exec.Cmd for running `python` using the configured toolchain.
	Command(ctx context.Context, args ...string) (*exec.Cmd, error)
	// ModuleCommand returns an *exec.Cmd for running an installed python module using the configured toolchain.
	// https://docs.python.org/3/using/cmdline.html#cmdoption-m
	ModuleCommand(ctx context.Context, module string, args ...string) (*exec.Cmd, error)
	// About returns information about the python executable of the toolchain.
	About(ctx context.Context) (Info, error)
}

func Name(tc toolchain) string {
	switch tc {
	case Pip:
		return "Pip"
	case Poetry:
		return "Poetry"
	default:
		return "Unknown"
	}
}

func ResolveToolchain(options PythonOptions) (Toolchain, error) {
	if options.Toolchain == Poetry {
		dir := options.ProgramDir
		if dir == "" {
			dir = options.Root
		}
		return newPoetry(dir)
	}
	return newPip(options.Root, options.Virtualenv)
}

func virtualEnvBinDirName() string {
	if runtime.GOOS == windows {
		return "Scripts"
	}
	return "bin"
}

// Determines if we should use pyenv. To use pyenv we need:
//   - pyenv installed
//   - .python-version file in the current directory or any of its parents
func usePyenv(cwd string) (bool, string, string, error) {
	versionFile, err := fsutil.Searchup(cwd, ".python-version")
	if err != nil {
		if !errors.Is(err, fsutil.ErrNotFound) {
			return false, "", "", fmt.Errorf("error while looking for .python-version %s", err)
		}
		// No .python-version file found
		return false, "", "", nil
	}
	logging.V(9).Infof("Python toolchain: found .python-version %s", versionFile)
	pyenvPath, err := exec.LookPath("pyenv")
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) {
			return false, "", "", fmt.Errorf("error while looking for pyenv %+v", err)
		}
		// No pyenv installed
		logging.V(9).Infof("Python toolchain: found .python-version file at %s, but could not find pyenv executable",
			versionFile)
		return false, "", "", nil
	}
	return true, pyenvPath, versionFile, nil
}

func installPython(ctx context.Context, cwd string, showOutput bool, infoWriter, errorWriter io.Writer) error {
	use, pyenv, versionFile, err := usePyenv(cwd)
	if err != nil {
		return err
	}
	if !use {
		return nil
	}

	if showOutput {
		_, err := infoWriter.Write([]byte(fmt.Sprintf("Installing python version from .python-version file at %s\n",
			versionFile)))
		if err != nil {
			return fmt.Errorf("error while writing to infoWriter %s", err)
		}
	}
	cmd := exec.CommandContext(ctx, pyenv, "install", "--skip-existing")
	cmd.Dir = cwd
	if showOutput {
		cmd.Stdout = infoWriter
		cmd.Stderr = errorWriter
	}
	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("error while running pyenv install: %s", string(exitErr.Stderr))
		}
		return fmt.Errorf("error while running pyenv install: %s", err)
	}
	return nil
}
