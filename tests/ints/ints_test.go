// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// TestProjectMain tests out the ability to override the main entrypoint.
func TestProjectMain(t *testing.T) {
	var test integration.ProgramTestOptions
	test = integration.ProgramTestOptions{
		Dir:          "project_main",
		Dependencies: []string{"pulumi"},
		ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
			// Simple runtime validation that just ensures the checkpoint was written and read.
			assert.Equal(t, test.StackName(), checkpoint.Target)
		},
	}
	integration.ProgramTest(t, test)
}

// TestStackProjectName ensures we can read the Pulumi stack and project name from within the program.
func TestStackProjectName(t *testing.T) {
	var test integration.ProgramTestOptions
	test = integration.ProgramTestOptions{
		Dir:          "stack_project_name",
		Dependencies: []string{"pulumi"},
		Quick:        true,
	}
	integration.ProgramTest(t, test)
}
