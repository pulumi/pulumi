// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// TestRunTestInfrastructureAgainstService is a copy of the "TestStackOutputs" integration test, but configured to
// run against the Pulumi Service located at PULUMI_API (if set).
func TestRunTestInfrastructureAgainstService(t *testing.T) {
	requirePulumiAPISet(t)

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "../integration/stack_outputs",
		Dependencies: []string{"pulumi"},
		Quick:        true,

		// Options specific to testing against the service. All environments have a "default" PPC in the moolumi org.
		CloudURL: os.Getenv("PULUMI_API"),
		Owner:    "moolumi",
		Repo:     "pulumi",
		PPCName:  "default",
	})
}
