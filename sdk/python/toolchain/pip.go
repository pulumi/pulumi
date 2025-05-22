// Copyright 2016-2020, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

const (
	windows = "windows"
)

type pip struct {
	// The absolute path to the virtual env. Empty if not using a virtual env.
	virtualenvPath string
	// The virtual option as set in Pulumi.yaml.
	virtualenvOption string
	// The root directory of the project.
	root string
}

var _ Toolchain = &pip{}

func newPip(root, virtualenv string) (*pip, error) {
	virtualenvPath := resolveVirtualEnvironmentPath(root, virtualenv)
	logging.V(9).Infof("Python toolchain: using pip at %s", virtualenvPath)
	return &pip{virtualenvPath, virtualenv, root}, nil
}

func (p *pip) InstallDependencies(ctx context.Context, cwd string, useLanguageVersionTools, showOutput bool,
	infoWriter io.Writer, errorWriter io.Writer,
) error {
	return InstallDependencies(
		ctx,
		cwd,
		p.virtualenvPath,
		useLanguageVersionTools,
		showOutput,
		infoWriter,
		errorWriter)
}

func (p *pip) ListPackages(ctx context.Context, transitive bool) ([]PythonPackage, error) {
	args := []string{"list", "-v", "--format", "json"}
	if !transitive {
		args = append(args, "--not-required")
	}

	cmd, err := p.ModuleCommand(ctx, "pip", args...)
	if err != nil {
		return nil, err
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("calling `python %s`: %w", strings.Join(args, " "), err)
	}

	var packages []PythonPackage
	jsonDecoder := json.NewDecoder(bytes.NewBuffer(output))
	if err := jsonDecoder.Decode(&packages); err != nil {
		return nil, fmt.Errorf("parsing `python %s` output: %w", strings.Join(args, " "), err)
	}

	return packages, nil
}

func (p *pip) Command(ctx context.Context, arg ...string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	if p.virtualenvPath == "" {
		return Command(ctx, arg...)
	}
	name := "python"
	if runtime.GOOS == windows {
		name = name + ".exe"
	}
	cmdPath := filepath.Join(p.virtualenvPath, virtualEnvBinDirName(), name)
	cmd = exec.Command(cmdPath, arg...)

	cmd.Env = ActivateVirtualEnv(os.Environ(), p.virtualenvPath)

	return cmd, nil
}

func (p *pip) ModuleCommand(ctx context.Context, module string, args ...string) (*exec.Cmd, error) {
	moduleArgs := append([]string{"-m", module}, args...)
	return p.Command(ctx, moduleArgs...)
}

func (p *pip) About(ctx context.Context) (Info, error) {
	var cmd *exec.Cmd
	cmd, err := p.Command(ctx, "--version")
	if err != nil {
		return Info{}, err
	}
	var out []byte
	if out, err = cmd.Output(); err != nil {
		return Info{}, fmt.Errorf("failed to get version: %w", err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(string(out), "Python "))

	return Info{
		Executable: cmd.Path,
		Version:    version,
	}, nil
}

func (p *pip) ValidateVenv(ctx context.Context) error {
	if p.virtualenvOption != "" && !IsVirtualEnv(p.virtualenvPath) {
		return NewVirtualEnvError(p.virtualenvOption, p.virtualenvPath)
	}
	return nil
}

func (p *pip) EnsureVenv(ctx context.Context, cwd string, useLanguageVersionTools,
	showOutput bool, infoWriter, errorWriter io.Writer,
) error {
	// If we are using global/ambient Python, do nothing.
	if p.virtualenvOption == "" {
		return nil
	}

	if IsVirtualEnv(p.virtualenvPath) {
		return nil
	}

	var createVirtualEnv bool
	info, err := os.Stat(p.virtualenvPath)
	if err != nil {
		if os.IsNotExist(err) {
			createVirtualEnv = true
		} else {
			return err
		}
	} else if !info.IsDir() {
		return fmt.Errorf("the 'virtualenv' option in Pulumi.yaml is set to %q but it is not a directory", p.virtualenvPath)
	}

	// If the virtual environment directory exists, but is empty, it needs to be created.
	if !createVirtualEnv {
		empty, err := fsutil.IsDirEmpty(p.virtualenvPath)
		if err != nil {
			return err
		}
		createVirtualEnv = empty
	}

	if createVirtualEnv {
		return p.InstallDependencies(ctx, cwd, useLanguageVersionTools, showOutput, infoWriter, errorWriter)
	}

	return nil
}

func (p *pip) VirtualEnvPath(_ context.Context) (string, error) {
	return p.virtualenvPath, nil
}

// IsVirtualEnv returns true if the specified directory contains a python binary.
func IsVirtualEnv(dir string) bool {
	pyBin := filepath.Join(dir, virtualEnvBinDirName(), "python")
	if runtime.GOOS == windows {
		pyBin = pyBin + ".exe"
	}
	if info, err := os.Stat(pyBin); err == nil && !info.IsDir() {
		return true
	}
	return false
}

// CommandPath finds the correct path and command for Python. If the `PULUMI_PYTHON_CMD`
// variable is set it will be looked for on `PATH`, otherwise, `python3` and
// `python` will be looked for.
func CommandPath() (string /*pythonPath*/, string /*pythonCmd*/, error) {
	var err error
	var pythonCmds []string

	if pythonCmd := os.Getenv("PULUMI_PYTHON_CMD"); pythonCmd != "" {
		pythonCmds = []string{pythonCmd}
	} else {
		// Look for `python3` by default, but fallback to `python` if not found, except on Windows
		// where we look for these in the reverse order because the default python.org Windows
		// installation does not include a `python3` binary, and the existence of a `python3.exe`
		// symlink to `python.exe` on some systems does not work correctly with the Python `venv`
		// module.
		pythonCmds = []string{"python3", "python"}
		if runtime.GOOS == windows {
			pythonCmds = []string{"python", "python3"}
		}
	}

	var pythonCmd, pythonPath string
	for _, pythonCmd = range pythonCmds {
		pythonPath, err = exec.LookPath(pythonCmd)
		// Break on the first cmd we find on the path (if any)
		if err == nil {
			break
		}
	}
	if err != nil {
		// second-chance on windows for python being installed through the Windows app store.
		if runtime.GOOS == windows {
			pythonCmd, pythonPath, err = resolveWindowsExecutionAlias(pythonCmds)
		}
		if err != nil {
			return "", "", fmt.Errorf(
				"failed to locate any of %q on your PATH. Have you installed Python 3.6 or greater?",
				pythonCmds)
		}
	}
	return pythonPath, pythonCmd, nil
}

// Command returns an *exec.Cmd for running `python`. Uses `ComandPath`
// internally to find the correct executable.
func Command(ctx context.Context, arg ...string) (*exec.Cmd, error) {
	pythonPath, _, err := CommandPath()
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, pythonPath, arg...), nil
}

// resolveWindowsExecutionAlias performs a lookup for python among UWP
// application execution aliases which exec.LookPath() can't handle.
// Windows 10 supports execution aliases for UWP applications. If python
// is installed using the Windows store app, the installer will drop an alias
// in %LOCALAPPDATA%\Microsoft\WindowsApps which is a zero-length file - also
// called an execution alias. This directory is also added to the PATH.
// See https://www.tiraniddo.dev/2019/09/overview-of-windows-execution-aliases.html
// for an overview.
// Most of this code is a replacement of the windows version of exec.LookPath
// but uses os.Lstat instead of an os.Stat which fails with a
// "CreateFile <path>: The file cannot be accessed by the system".
func resolveWindowsExecutionAlias(pythonCmds []string) (string, string, error) {
	exts := []string{""}
	x := os.Getenv(`PATHEXT`)
	if x != "" {
		for _, e := range strings.Split(strings.ToLower(x), `;`) {
			if e == "" {
				continue
			}
			if e[0] != '.' {
				e = "." + e
			}
			exts = append(exts, e)
		}
	} else {
		exts = append(exts, ".com", ".exe", ".bat", ".cmd")
	}

	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if !strings.Contains(strings.ToLower(dir), filepath.Join("microsoft", "windowsapps")) {
			continue
		}
		for _, pythonCmd := range pythonCmds {
			for _, ext := range exts {
				path := filepath.Join(dir, pythonCmd+ext)
				_, err := os.Lstat(path)
				if err != nil && !os.IsNotExist(err) {
					return "", "", fmt.Errorf("evaluating python execution alias: %w", err)
				}
				if os.IsNotExist(err) {
					continue
				}
				return pythonCmd, path, nil
			}
		}
	}

	return "", "", errors.New("no python execution alias found")
}

// VirtualEnvCommand returns an *exec.Cmd for running a command from the specified virtual environment
// directory.
func VirtualEnvCommand(virtualEnvDir, name string, arg ...string) *exec.Cmd {
	if runtime.GOOS == windows {
		name = name + ".exe"
	}
	cmdPath := filepath.Join(virtualEnvDir, virtualEnvBinDirName(), name)
	return exec.Command(cmdPath, arg...)
}

// NewVirtualEnvError creates an error about the virtual environment with more info on how to resolve the issue.
func NewVirtualEnvError(dir, fullPath string) error {
	pythonBin := "python3"
	if runtime.GOOS == windows {
		pythonBin = "python"
	}
	venvPythonBin := filepath.Join(fullPath, virtualEnvBinDirName(), "python")

	message := "doesn't appear to be a virtual environment"
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		message = "doesn't exist"
	}

	commandsText := fmt.Sprintf("    1. %s -m venv %s\n", pythonBin, fullPath) +
		fmt.Sprintf("    2. %s -m pip install --upgrade pip setuptools wheel\n", venvPythonBin) +
		fmt.Sprintf("    3. %s -m pip install -r requirements.txt\n", venvPythonBin)

	return fmt.Errorf("The 'virtualenv' option in Pulumi.yaml is set to %q, but %q %s; "+
		"run the following commands to create the virtual environment and install dependencies into it:\n\n%s\n\n"+
		"For more information see: https://www.pulumi.com/docs/intro/languages/python/#virtual-environments",
		dir, fullPath, message, commandsText)
}

// InstallDependencies will create a new virtual environment and install dependencies in the root directory.
func InstallDependencies(ctx context.Context, cwd, venvDir string, useLanguageVersionTools, showOutput bool,
	infoWriter, errorWriter io.Writer,
) error {
	printmsg := func(message string) {
		if showOutput {
			fmt.Fprintf(infoWriter, "%s\n", message)
		}
	}

	if useLanguageVersionTools {
		if err := installPython(ctx, cwd, showOutput, infoWriter, errorWriter); err != nil {
			return err
		}
	}

	if venvDir != "" {
		printmsg("Creating virtual environment...")

		// Create the virtual environment by running `python -m venv <venvDir>`.
		if !filepath.IsAbs(venvDir) {
			return fmt.Errorf("virtual environment path must be absolute: %s", venvDir)
		}

		cmd, err := Command(ctx, "-m", "venv", venvDir)
		if err != nil {
			return err
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			if len(output) > 0 {
				fmt.Fprintf(errorWriter, "%s\n", string(output))
			}
			return fmt.Errorf("creating virtual environment at '%s': %w", venvDir, err)
		}

		printmsg("Finished creating virtual environment")
	}

	runPipInstall := func(errorMsg string, arg ...string) error {
		args := append([]string{"-m", "pip", "install"}, arg...)

		var pipCmd *exec.Cmd
		if venvDir == "" {
			var err error
			pipCmd, err = Command(ctx, args...)
			if err != nil {
				return err
			}
		} else {
			pipCmd = VirtualEnvCommand(venvDir, "python", args...)
		}
		pipCmd.Dir = cwd
		pipCmd.Env = ActivateVirtualEnv(os.Environ(), venvDir)

		wrapError := func(err error) error {
			return fmt.Errorf("%s via '%s': %w", errorMsg, strings.Join(pipCmd.Args, " "), err)
		}

		if showOutput {
			// Show stdout/stderr output.
			pipCmd.Stdout = infoWriter
			pipCmd.Stderr = errorWriter
			if err := pipCmd.Run(); err != nil {
				return wrapError(err)
			}
		} else {
			// Otherwise, only show output if there is an error.
			if output, err := pipCmd.CombinedOutput(); err != nil {
				if len(output) > 0 {
					fmt.Fprintf(errorWriter, "%s\n", string(output))
				}
				return wrapError(err)
			}
		}
		return nil
	}

	printmsg("Updating pip, setuptools, and wheel in virtual environment...")

	// activate virtual environment

	err := runPipInstall("updating pip, setuptools, and wheel", "--upgrade", "pip", "setuptools", "wheel")
	if err != nil {
		return err
	}

	printmsg("Finished updating")

	// If `requirements.txt` doesn't exist, exit early.
	requirementsPath := filepath.Join(cwd, "requirements.txt")
	if _, err := os.Stat(requirementsPath); os.IsNotExist(err) {
		return nil
	}

	printmsg("Installing dependencies in virtual environment...")

	err = runPipInstall("installing dependencies", "-r", "requirements.txt")
	if err != nil {
		return err
	}

	printmsg("Finished installing dependencies")

	return nil
}

func resolveVirtualEnvironmentPath(root, virtualenv string) string {
	if virtualenv == "" {
		return ""
	}
	if !filepath.IsAbs(virtualenv) {
		return filepath.Join(root, virtualenv)
	}
	return virtualenv
}
