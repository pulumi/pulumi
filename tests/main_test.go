// Copyright 2017-2018, Pulumi Corporation.
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

package tests

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func TestMain(m *testing.M) {
	// If the Cloud-enabled versions of CLI commands will be used, log in with a test account.
	if os.Getenv("PULUMI_API") != "" {
		// Use different location for storing credentials, as to not impact the developer's box.
		if err := os.Setenv(workspace.UseAltCredentialsLocationEnvVar, "1"); err != nil {
			fmt.Printf("error setting env var '%s': %v\n", workspace.UseAltCredentialsLocationEnvVar, err)
			os.Exit(1)
		}

		os.Setenv(cloud.AccessTokenEnvVar, integration.TestAccountAccessToken)
		output, err := exec.Command("pulumi", "login").Output()
		if err != nil {
			fmt.Printf("Error logging in (output '%v'): %v\n", string(output), err)
			os.Exit(1)
		}
	}

	// Disable stack backups for tests to avoid filling up ~/.pulumi/backups with unnecessary
	// backups of test stacks.
	if err := os.Setenv(local.DisableCheckpointBackupsEnvVar, "1"); err != nil {
		fmt.Printf("error setting env var '%s': %v\n", local.DisableCheckpointBackupsEnvVar, err)
		os.Exit(1)
	}

	code := m.Run()
	os.Exit(code)
}
