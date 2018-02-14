// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
