// Copyright 2025, Pulumi Corporation.
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

package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// If the PULUMI_INTEGRATION_REBUILD_BINARIES environment variable is set to "true", this function will rebuild the
// Pulumi CLI and the language runtime plugins into the `bin` directory of the repository root. It will then set up the
// $PATH environment variable to include this directory, so that when the tests run we will use the newly built binaries
// without polluting the global $PATH, where the integration tests usually expect to find the binaries.
func SetupPulumiBinary() {
	// Find the root of the repository
	stdout, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		fmt.Printf("error finding repo root: %v\n", err)
		os.Exit(1)
	}
	repoRoot := strings.TrimSpace(string(stdout))
	if os.Getenv("PULUMI_INTEGRATION_REBUILD_BINARIES") == "true" {
		cmd := exec.Command("make", "build_local")
		cmd.Dir = repoRoot
		stdout, err = cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("error building plugin: %v.  Output: %v\n", err, string(stdout))
			os.Exit(1)
		}
	}
	repoBin := filepath.Join(repoRoot, "bin")
	os.Setenv("PATH", fmt.Sprintf("%s:%s", repoBin, os.Getenv("PATH")))
	if os.Getenv("PULUMI_INTEGRATION_BINARY_PATH") == "" {
		pulumiBinPath := filepath.Join(repoBin, "pulumi")
		// Disable in CI to avoid breaking the matrix calculation which uses the output from `go test`
		if _, isCI := os.LookupEnv("CI"); !isCI {
			if _, err := os.Stat(pulumiBinPath); os.IsNotExist(err) {
				fmt.Printf("WARNING: pulumi binary not found at %s. "+
					"Falling back to searching the $PATH. "+
					"Run `make build_local` or set `PULUMI_INTEGRATION_REBUILD_BINARIES=true`.\n", pulumiBinPath)
				return
			}
		}
		os.Setenv("PULUMI_INTEGRATION_BINARY_PATH", pulumiBinPath)
	}
}

// This runs pulumi install on the python provider so it's venv is setup for running.
func InstallPythonProvider() {
	// Find the root of the repository
	stdout, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		fmt.Printf("error finding repo root: %v\n", err)
		os.Exit(1)
	}
	repoRoot := strings.TrimSpace(string(stdout))

	// TODO: Would be good if this was just `pulumi install`
	cmd := exec.Command("python", "-m", "venv", "venv")
	providerRoot := filepath.Join(repoRoot, "tests", "testprovider-py")
	cmd.Dir = providerRoot
	stdout, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("error setup venv for plugin: %v.  Output: %v\n", err, string(stdout))
		os.Exit(1)
	}

	virtualenvBinPath := filepath.Join("venv", "bin", "python")
	if runtime.GOOS == "windows" {
		virtualenvBinPath = filepath.Join("venv", "Scripts", "python.exe")
	}
	cmd = exec.Command( //nolint:gosec
		virtualenvBinPath,
		"-m", "pip", "install", "-r", "requirements.txt")
	cmd.Dir = providerRoot
	stdout, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("error install requirements for plugin: %v.  Output: %v\n", err, string(stdout))
		os.Exit(1)
	}
}
