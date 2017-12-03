// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tests

import (
	"os"
	"testing"

	"github.com/pulumi/pulumi/cmd"
	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"
)

func TestRequireLogin(t *testing.T) {
	if os.Getenv("PULUMI_API") == "" {
		t.Skip("PULUMI_API environment variable not set. This means the Cloud-variant Pulumi commands won't run.")
	}
	// Use the alt path for credentials as to not impact the local deverloper's machine.
	if err := os.Setenv(cmd.UseAltCredentialsLocationEnvVar, "1"); err != nil {
		t.Fatalf("error setting env var '%s': %v", cmd.UseAltCredentialsLocationEnvVar, err)
	}

	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteEnvironment()

		integration.CreateBasicPulumiRepo(e)

		// logout and confirm auth error.
		e.RunCommand("pulumi", "logout")

		out, err := e.RunCommandExpectError("pulumi", "stack", "init", "--local", "foo")
		assert.Empty(t, out, "expected no stdout")
		assert.Contains(t, err, "error: getting stored credentials: credentials file not found")

		// login and confirm things work.
		os.Setenv(cmd.PulumiAccessTokenEnvVar, integration.TestAccountAccessToken)
		e.RunCommand("pulumi", "login")

		e.RunCommand("pulumi", "stack", "init", "--local", "foo")
		e.RunCommand("pulumi", "stack", "rm", "foo", "--yes")
	})
}
