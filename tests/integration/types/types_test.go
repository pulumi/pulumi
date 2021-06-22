// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
// +build python all

package ints

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/assert"
)

func TestPythonTypes(t *testing.T) {
	for _, dir := range []string{"simple", "declared"} {
		d := filepath.Join("python", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: d,
				Dependencies: []string{
					filepath.Join("..", "..", "..", "sdk", "python", "env", "src"),
				},
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					for _, res := range []string{"", "2", "3", "4"} {
						assert.Equal(t, "hello", stack.Outputs[fmt.Sprintf("res%s_first_value", res)])
						assert.Equal(t, 42.0, stack.Outputs[fmt.Sprintf("res%s_second_value", res)])
					}
				},
			})
		})
	}
}
