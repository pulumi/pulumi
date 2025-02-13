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

package ints

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("PULUMI_INTEGRATION_REBUILD_BINARIES") == "true" {
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

	os.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

	code := m.Run()
	os.Exit(code)
}
