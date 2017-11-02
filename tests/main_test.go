// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tests

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/pulumi/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

func TestMain(m *testing.M) {
	// If the Cloud-enabled versions of CLI commands will be used, log in with a test account.
	if os.Getenv("PULUMI_API") != "" {
		// Use different location for storing credentials, as to not impact the developer's box.
		if err := os.Setenv(cmd.UseAltCredentialsLocationEnvVar, "1"); err != nil {
			fmt.Printf("error setting env var '%s': %v\n", cmd.UseAltCredentialsLocationEnvVar, err)
			os.Exit(1)
		}

		os.Setenv(cmd.PulumiAccessTokenEnvVar, integration.TestAccountAccessToken)
		output, err := exec.Command("pulumi", "login").Output()
		if err != nil {
			fmt.Printf("Error logging in (output '%v'): %v\n", string(output), err)
			os.Exit(1)
		}
	}

	code := m.Run()
	os.Exit(code)
}
