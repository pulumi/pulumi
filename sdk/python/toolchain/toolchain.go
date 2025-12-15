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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
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
	Uv
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
	// The path to the python executable that's being used
	PythonExecutable string
	// The version of python
	PythonVersion semver.Version
	// The version of pip, poetry, uv ...
	ToolchainVersion semver.Version
}

type Toolchain interface {
	// InstallDependencies installs the dependencies of the project found in `cwd`.
	InstallDependencies(ctx context.Context, cwd string, useLanguageVersionTools,
		showOutput bool, infoWriter, errorWriter io.Writer) error
	// PrepareProject prepares the python project for use with its toolchain. For example it will convert a
	// requirements.txt into an pyproject.toml for uv or poetry.
	PrepareProject(ctx context.Context, projectName, cwd string, showOutput bool, infoWriter,
		errorWriter io.Writer) error
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
	// VirtualEnvPath returns the path of the virtual env used by the toolchain.
	VirtualEnvPath(ctx context.Context) (string, error)
	// LinkPackages adds packages as dependencies to the Python program, updating the relevant dependency files.
	// (pyproject.toml, requirements.txt). The virtual environment is not updated with the new dependencies. Run
	// InstallDependencies to install the new dependencies if needed.
	// `packages` is a map python package names to paths.
	LinkPackages(ctx context.Context, packages map[string]string) error
}

func Name(tc toolchain) string {
	switch tc {
	case Pip:
		return "Pip"
	case Poetry:
		return "Poetry"
	case Uv:
		return "Uv"
	default:
		return "Unknown"
	}
}

func TypeCheckerName(tc typeChecker) string {
	switch tc {
	case TypeCheckerMypy:
		return "Mypy"
	case TypeCheckerPyright:
		return "Pyright"
	case TypeCheckerNone:
		fallthrough
	default:
		return "None"
	}
}

func ResolveToolchain(options PythonOptions) (Toolchain, error) {
	switch options.Toolchain { //nolint:exhaustive // golangci-lint v2 upgrade
	case Poetry:
		dir := options.ProgramDir
		if dir == "" {
			dir = options.Root
		}
		return newPoetry(dir)
	case Uv:
		return newUv(options.Root, options.Virtualenv)
	}
	return newPip(options.Root, options.Virtualenv)
}

// ActivateVirtualEnv takes an array of environment variables (same format as os.Environ()) and path to
// a virtual environment directory, and returns a new "activated" array with the virtual environment's
// "bin" dir ("Scripts" on Windows) prepended to the `PATH` environment variable, the `VIRTUAL_ENV`
// variable set to the path, and the `PYTHONHOME` variable removed.
func ActivateVirtualEnv(environ []string, virtualEnvDir string) []string {
	virtualEnvBin := filepath.Join(virtualEnvDir, virtualEnvBinDirName())
	var hasPath bool
	var result []string
	for _, env := range environ {
		split := strings.SplitN(env, "=", 2)
		contract.Assertf(len(split) == 2, "unexpected environment variable: %q", env)
		key, value := split[0], split[1]

		// Case-insensitive compare, as Windows will normally be "Path", not "PATH".
		if strings.EqualFold(key, "PATH") {
			hasPath = true
			// Prepend the virtual environment bin directory to PATH so any calls to run
			// python or pip will use the binaries in the virtual environment.
			path := fmt.Sprintf("%s=%s%s%s", key, virtualEnvBin, string(os.PathListSeparator), value)
			result = append(result, path)
		} else if strings.EqualFold(key, "PYTHONHOME") {
			// Skip PYTHONHOME to "unset" this value.
		} else if strings.EqualFold(key, "VIRTUAL_ENV") {
			// Skip VIRTUAL_ENV, we always set this to `virtualEnvDir`
		} else {
			result = append(result, env)
		}
	}
	if !hasPath {
		path := "PATH=" + virtualEnvBin
		result = append(result, path)
	}
	virtualEnv := "VIRTUAL_ENV=" + virtualEnvDir
	result = append(result, virtualEnv)
	return result
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
		_, err := fmt.Fprintf(infoWriter, "Installing python version from .python-version file at %s\n",
			versionFile)
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
		return errutil.ErrorWithStderr(err, "error while running pyenv install")
	}
	return nil
}

func searchup(currentDir, fileToFind string) (string, error) {
	if _, err := os.Stat(filepath.Join(currentDir, fileToFind)); err == nil {
		return currentDir, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	parentDir := filepath.Dir(currentDir)
	if currentDir == parentDir {
		// Reached the root directory, file not found
		return "", os.ErrNotExist
	}
	return searchup(parentDir, fileToFind)
}

func getPythonExecutablePath(ctx context.Context,
	commandFunc func(context.Context, ...string) (*exec.Cmd, error),
) (string, error) {
	cmd, err := commandFunc(ctx, "-c", "import sys; print(sys.executable)")
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get python executable path: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func ParsePythonVersion(versionString string) (semver.Version, error) {
	versionString = strings.TrimSpace(versionString)
	versionString = strings.TrimPrefix(versionString, "Python ")

	re := regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?(?:(a|b|rc|dev)(\d+))?`)
	matches := re.FindStringSubmatch(versionString)
	if len(matches) < 3 {
		return semver.Version{}, fmt.Errorf("invalid Python version format: %q", versionString)
	}

	major, minor := matches[1], matches[2]
	patch := matches[3]
	if patch == "" {
		patch = "0"
	}

	normalizedVersion := fmt.Sprintf("%s.%s.%s", major, minor, patch)

	// Convert Python pre-release naming to semver format
	if matches[4] != "" {
		preReleaseType := matches[4]
		preReleaseNum := matches[5]
		switch preReleaseType {
		case "a":
			normalizedVersion += "-alpha." + preReleaseNum
		case "b":
			normalizedVersion += "-beta." + preReleaseNum
		case "rc":
			normalizedVersion += "-rc." + preReleaseNum
		case "dev":
			normalizedVersion += "-dev." + preReleaseNum
		}
	}

	sem, err := semver.Parse(normalizedVersion)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to parse Python version %q: %w", normalizedVersion, err)
	}

	return sem, nil
}

func getPythonVersion(ctx context.Context,
	commandFunc func(context.Context, ...string) (*exec.Cmd, error),
) (semver.Version, error) {
	cmd, err := commandFunc(ctx, "--version")
	if err != nil {
		return semver.Version{}, err
	}
	out, err := cmd.Output()
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to get version: %w", err)
	}
	versionStr := strings.TrimSpace(string(out))
	pythonVersion, err := ParsePythonVersion(versionStr)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to parse python version %q: %w", versionStr, err)
	}
	return pythonVersion, nil
}
