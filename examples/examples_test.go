// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package examples

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

func TestExamples(t *testing.T) {
	cwd, err := os.Getwd()
	if !assert.NoError(t, err, "expected a valid working directory: %v", err) {
		return
	}
	examples := []integration.ProgramTestOptions{
		{
			Dir:          path.Join(cwd, "minimal"),
			Dependencies: []string{"pulumi"},
			Config: map[string]string{
				"name": "Pulumi",
			},
			Secrets: map[string]string{
				"secret": "this is my secret message",
			},
			ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
				// Simple runtime validation that just ensures the checkpoint was written and read.
				assert.Equal(t, integration.TestStackName, checkpoint.Target)
			},
		},
		{
			Dir:          path.Join(cwd, "dynamic-provider/simple"),
			Dependencies: []string{"pulumi"},
			Config: map[string]string{
				"simple:config:w": "1",
				"simple:config:x": "1",
				"simple:config:y": "1",
			},
		},
		{
			Dir:          path.Join(cwd, "dynamic-provider/multiple-turns"),
			Dependencies: []string{"pulumi"},
		},
	}
	for _, ex := range examples {
		example := ex
		t.Run(example.Dir, func(t *testing.T) {
			integration.ProgramTest(t, example)
		})
	}
}
