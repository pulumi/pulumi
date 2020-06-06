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
	if showOutput {
		fmt.Println("Creating virtual environment...")
		fmt.Println()
	}

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
	if err := saveProj("venv"); err != nil {
		return err
	}

	if showOutput {
		fmt.Println("Finished creating virtual environment")
		fmt.Println()
	}

	// If `requirements.txt` doesn't exist, just exit early.
	requirementsPath := filepath.Join(root, "requirements.txt")
	if _, err := os.Stat(requirementsPath); os.IsNotExist(err) {
		return nil
	}

	if showOutput {
		fmt.Println("Installing dependencies...")
		fmt.Println()
	}

	// Install dependencies by running `pip install -r requirements.txt` using the `pip`
	// in the virtual environment.
	pipCmd := VirtualEnvCommand(venvDir, "pip", "install", "-r", "requirements.txt")
	pipCmd.Dir = root
	pipCmd.Env = ActivateVirtualEnv(os.Environ(), venvDir)
	if showOutput {
		// Show stdout/stderr output.
		pipCmd.Stdout = os.Stdout
		pipCmd.Stderr = os.Stderr
		if err := pipCmd.Run(); err != nil {
			return errors.Wrap(err, "installing dependencies via `pip install -r requirements.txt`")
		}
	} else {
		// Otherwise, only show output if there is an error.
		if output, err := pipCmd.CombinedOutput(); err != nil {
			if len(output) > 0 {
				os.Stdout.Write(output)
				fmt.Println()
			}
			return errors.Wrap(err, "installing dependencies via `pip install -r requirements.txt`")
		}
	}

	if showOutput {
		fmt.Println("Finished installing dependencies")
		fmt.Println()
	}

	return nil
}

func virtualEnvBinDirName() string {
	if runtime.GOOS == windows {
		return "Scripts"
	}
	return "bin"
}
