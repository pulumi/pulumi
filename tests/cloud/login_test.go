// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/backend/cloud"
	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"
)

// requirePulumiAPISet will skip the test unless the PULUMI_API is set.
func requirePulumiAPISet(t *testing.T) {
	if os.Getenv("PULUMI_API") == "" {
		t.Skip("PULUMI_API environment variable not set. Skipping this test.")
	}
}

func TestRequireLogin(t *testing.T) {
	requirePulumiAPISet(t)

	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)

		// logout and confirm auth error.
		e.RunCommand("pulumi", "logout")

		out, err := e.RunCommandExpectError("pulumi", "stack", "init", "foo", "--remote")
		assert.Empty(t, out, "expected no stdout")
		assert.Contains(t, err, "error: you must be logged in to create stacks in the Pulumi Cloud.")
		assert.Contains(t, err, "Run `pulumi login` to log in.")

		// login and confirm things work.
		os.Setenv(cloud.AccessTokenEnvVar, integration.TestAccountAccessToken)
		e.RunCommand("pulumi", "login")

		e.RunCommand("pulumi", "stack", "init", "foo", "--remote")
		e.RunCommand("pulumi", "stack", "rm", "foo", "--yes")
	})
}
