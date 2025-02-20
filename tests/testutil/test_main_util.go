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
	"strings"
)

// If the PULUMI_INTEGRATION_REBUILD_BINARIES environment variable is set to "true", this function will rebuild the
// Pulumi CLI and the language runtime plugins into the `bin` directory of the repository root. It will then set up the
// $PATH environment variable to include this directory, so that when the tests run we will use the newly built binaries
// without polluting the global $PATH, where the integration tests usually expect to find the binaries.
func SetupBinaryRebuilding() {
	if os.Getenv("PULUMI_INTEGRATION_REBUILD_BINARIES") != "true" {
		return
	}
	// Find the root of the repository
	stdout, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		fmt.Printf("error finding repo root: %v\n", err)
		os.Exit(1)
	}
	repoRoot := strings.TrimSpace(string(stdout))
	cmd := exec.Command("make", "build_local")
	cmd.Dir = repoRoot
	stdout, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("error building plugin: %v.  Output: %v\n", err, string(stdout))
		os.Exit(1)
	}
	os.Setenv("PATH", fmt.Sprintf("%s:%s", repoRoot+"/bin", os.Getenv("PATH")))
}
