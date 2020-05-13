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
func VirtualEnvCommand(virtualEnvDir string, name string, arg ...string) *exec.Cmd {
	if runtime.GOOS == windows {
		name = fmt.Sprintf("%s.exe", name)
	}
	cmdPath := filepath.Join(virtualEnvDir, virtualEnvBinDirName(), name)
	return exec.Command(cmdPath, arg...)
}

// IsVirtualEnv returns true if the specified directory contains python and pip binaries.
func IsVirtualEnv(dir string) bool {
	fileExists := func(file string) bool {
		if runtime.GOOS == windows {
			file = fmt.Sprintf("%s.exe", file)
		}
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			return true
		}
		return false
	}

	return fileExists(filepath.Join(dir, virtualEnvBinDirName(), "python")) &&
		fileExists(filepath.Join(dir, virtualEnvBinDirName(), "pip"))
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

func virtualEnvBinDirName() string {
	if runtime.GOOS == windows {
		return "Scripts"
	}
	return "bin"
}
