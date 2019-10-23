// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"
)

// TestQuery creates a stack and runs a query over the stack's resource ouptputs.
func TestQuery(t *testing.T) {
	//
	// TODO[hausdorff, #3396]: Enable once we allow query against local backend stacks.
	//

	// integration.ProgramTest(t, &integration.ProgramTestOptions{
	// 	// Create Pulumi resources.
	// 	Dir:          "step1",
	// 	StackName:    "query-stack",
	// 	Dependencies: []string{"@pulumi/pulumi"},
	// 	EditDirs: []integration.EditDir{
	// 		// Try to create resources during `pulumi query`. This should fail.
	// 		{
	// 			Dir:           "step2",
	// 			Additive:      true,
	// 			QueryMode:     true,
	// 			ExpectFailure: true,
	// 		},
	// 		// Run a query during `pulumi query`. Should succeed.
	// 		{
	// 			Dir:           "step3",
	// 			Additive:      true,
	// 			QueryMode:     true,
	// 			ExpectFailure: false,
	// 		},
	// 	},
	// })
}
