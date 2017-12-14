// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package examples

import (
	"bytes"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

func TestExamples(t *testing.T) {
	cwd, err := os.Getwd()
	if !assert.NoError(t, err, "expected a valid working directory: %v", err) {
		return
	}

	var minimal integration.ProgramTestOptions
	minimal = integration.ProgramTestOptions{
		Dir:          path.Join(cwd, "minimal"),
		Dependencies: []string{"pulumi"},
		Config: map[string]string{
			"name": "Pulumi",
		},
		Secrets: map[string]string{
			"secret": "this is my secret message",
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Simple runtime validation that just ensures the checkpoint was written and read.
			assert.Equal(t, minimal.StackName(), stackInfo.Checkpoint.Stack)
		},
		ReportStats: integration.NewS3Reporter("us-west-2", "eng.pulumi.com", "testreports"),
	}

	var formattableStdout, formattableStderr bytes.Buffer
	examples := []integration.ProgramTestOptions{
		minimal,
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
		{
			Dir:          path.Join(cwd, "formattable"),
			Dependencies: []string{"pulumi"},
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				// Note that we're abusing this hook to validate stdout. We don't actually care about the checkpoint.
				stdout := formattableStdout.String()
				assert.False(t, strings.Contains(stdout, "MISSING"))
			},
			Stdout: &formattableStdout,
			Stderr: &formattableStderr,
		},
	}

	for _, ex := range examples {
		example := ex
		t.Run(example.Dir, func(t *testing.T) {
			integration.ProgramTest(t, &example)
		})
	}
}
