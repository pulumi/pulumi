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

package python

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

const windows = "windows"

// Command returns an *exec.Cmd for running `python`. If the `PULUMI_PYTHON_CMD` variable is set
// it will be looked for on `PATH`, otherwise, `python3` and `python` will be looked for.
func Command(arg ...string) (*exec.Cmd, error) {
	var err error
	var pythonCmds []string
	var pythonPath string

	if pythonCmd := os.Getenv("PULUMI_PYTHON_CMD"); pythonCmd != "" {
		pythonCmds = []string{pythonCmd}
	} else {
		// Look for "python3" by default, but fallback to `python` if not found as some Python 3
		// distributions (in particular the default python.org Windows installation) do not include
		// a `python3` binary.
		pythonCmds = []string{"python3", "python"}
	}

	for _, pythonCmd := range pythonCmds {
		pythonPath, err = exec.LookPath(pythonCmd)
		// Break on the first cmd we find on the path (if any)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to locate any of %q on your PATH.  Have you installed Python 3.6 or greater?",
			pythonCmds)
	}

	return exec.Command(pythonPath, arg...), nil
}

// VirtualEnvCommand returns an *exec.Cmd for running a command from the specified virtual environment
// directory.
func VirtualEnvCommand(virtualEnvDir, name string, arg ...string) *exec.Cmd {
	if runtime.GOOS == windows {
		name = fmt.Sprintf("%s.exe", name)
	}
	cmdPath := filepath.Join(virtualEnvDir, virtualEnvBinDirName(), name)
	return exec.Command(cmdPath, arg...)
}

// IsVirtualEnv returns true if the specified directory contains a python binary.
func IsVirtualEnv(dir string) bool {
	pyBin := filepath.Join(dir, virtualEnvBinDirName(), "python")
	if runtime.GOOS == windows {
		pyBin = fmt.Sprintf("%s.exe", pyBin)
	}
	if info, err := os.Stat(pyBin); err == nil && !info.IsDir() {
		return true
	}
	return false
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

	return errors.Errorf("The 'virtualenv' option in Pulumi.yaml is set to %q, but %q %s; "+
		"run the following commands to create the virtual environment and install dependencies into it:\n\n%s\n\n"+
		"For more information see: https://www.pulumi.com/docs/intro/languages/python/#virtual-environments",
		dir, fullPath, message, commandsText)
}

// ActivateVirtualEnv takes an array of environment variables (same format as os.Environ()) and path to
// a virtual environment directory, and returns a new "activated" array with the virtual environment's
// "bin" dir ("Scripts" on Windows) prepended to the `PATH` environment variable and `PYTHONHOME` variable
// removed.
func ActivateVirtualEnv(environ []string, virtualEnvDir string) []string {
	virtualEnvBin := filepath.Join(virtualEnvDir, virtualEnvBinDirName())
	var hasPath bool
	var result []string
	for _, env := range environ {
		if strings.HasPrefix(env, "PATH=") {
			hasPath = true
			// Prepend the virtual environment bin directory to PATH so any calls to run
			// python or pip will use the binaries in the virtual environment.
			originalValue := env[len("PATH="):]
			path := fmt.Sprintf("PATH=%s%s%s", virtualEnvBin, string(os.PathListSeparator), originalValue)
			result = append(result, path)
		} else if strings.HasPrefix(env, "PYTHONHOME=") {
			// Skip PYTHONHOME to "unset" this value.
		} else {
			result = append(result, env)
		}
	}
	if !hasPath {
		path := fmt.Sprintf("PATH=%s", virtualEnvBin)
		result = append(result, path)
	}
	return result
}

// InstallDependencies will create a new virtual environment and install dependencies in the root directory.
func InstallDependencies(root string, showOutput bool, saveProj func(virtualenv string) error) error {
	print := func(message string) {
		if showOutput {
			fmt.Println(message)
			fmt.Println()
		}
	}

	print("Creating virtual environment...")

	// Create the virtual environment by running `python -m venv venv`.
	venvDir := filepath.Join(root, "venv")
	cmd, err := Command("-m", "venv", venvDir)
	if err != nil {
		return err
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		if len(output) > 0 {
			os.Stdout.Write(output)
			fmt.Println()
		}
		return errors.Wrapf(err, "creating virtual environment at %s", venvDir)
	}

	// Save project with venv info.
	if saveProj != nil {
		if err := saveProj("venv"); err != nil {
			return err
		}
	}

	print("Finished creating virtual environment")

	runPipInstall := func(errorMsg string, arg ...string) error {
		pipCmd := VirtualEnvCommand(venvDir, "python", append([]string{"-m", "pip", "install"}, arg...)...)
		pipCmd.Dir = root
		pipCmd.Env = ActivateVirtualEnv(os.Environ(), venvDir)

		wrapError := func(err error) error {
			return errors.Wrapf(err, "%s via '%s'", errorMsg, strings.Join(pipCmd.Args, " "))
		}

		if showOutput {
			// Show stdout/stderr output.
			pipCmd.Stdout = os.Stdout
			pipCmd.Stderr = os.Stderr
			if err := pipCmd.Run(); err != nil {
				return wrapError(err)
			}
		} else {
			// Otherwise, only show output if there is an error.
			if output, err := pipCmd.CombinedOutput(); err != nil {
				if len(output) > 0 {
					os.Stdout.Write(output)
					fmt.Println()
				}
				return wrapError(err)
			}
		}
		return nil
	}

	print("Updating pip, setuptools, and wheel in virtual environment...")

	err = runPipInstall("updating pip, setuptools, and wheel", "--upgrade", "pip", "setuptools", "wheel")
	if err != nil {
		return err
	}

	print("Finished updating")

	// If `requirements.txt` doesn't exist, exit early.
	requirementsPath := filepath.Join(root, "requirements.txt")
	if _, err := os.Stat(requirementsPath); os.IsNotExist(err) {
		return nil
	}

	print("Installing dependencies in virtual environment...")

	err = runPipInstall("installing dependencies", "-r", "requirements.txt")
	if err != nil {
		return err
	}

	print("Finished installing dependencies")

	return nil
}

func virtualEnvBinDirName() string {
	if runtime.GOOS == windows {
		return "Scripts"
	}
	return "bin"
}
