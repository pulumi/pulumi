// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package examples

import (
	"bytes"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

func TestExamples(t *testing.T) {
	cwd, err := os.Getwd()
	if !assert.NoError(t, err, "expected a valid working directory: %v", err) {
		return
	}

	var formattableStdout, formattableStderr bytes.Buffer
	examples := []integration.ProgramTestOptions{
		{
			Dir:          path.Join(cwd, "minimal"),
			Dependencies: []string{"@pulumi/pulumi"},
			Config: map[string]string{
				"name": "Pulumi",
			},
			Secrets: map[string]string{
				"secret": "this is my secret message",
			},
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				// Simple runtime validation that just ensures the checkpoint was written and read.
				assert.NotNil(t, stackInfo.Deployment)
			},
			RunBuild: true,
		},
		{
			Dir:          path.Join(cwd, "dynamic-provider/simple"),
			Dependencies: []string{"@pulumi/pulumi"},
			Config: map[string]string{
				"simple:config:w": "1",
				"simple:config:x": "1",
				"simple:config:y": "1",
			},
		},
		{
			Dir:          path.Join(cwd, "dynamic-provider/multiple-turns"),
			Dependencies: []string{"@pulumi/pulumi"},
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				for _, res := range stackInfo.Deployment.Resources {
					if !providers.IsProviderType(res.Type) && res.Parent == "" {
						assert.Equal(t, stackInfo.RootResource.URN, res.URN,
							"every resource but the root resource should have a parent, but %v didn't", res.URN)
					}
				}
			},
		},
		{
			Dir:          path.Join(cwd, "dynamic-provider/derived-inputs"),
			Dependencies: []string{"@pulumi/pulumi"},
		},
		{
			Dir:          path.Join(cwd, "formattable"),
			Dependencies: []string{"@pulumi/pulumi"},
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				// Note that we're abusing this hook to validate stdout. We don't actually care about the checkpoint.
				stdout := formattableStdout.String()
				assert.False(t, strings.Contains(stdout, "MISSING"))
			},
			Stdout: &formattableStdout,
			Stderr: &formattableStderr,
		},
		{
			Dir:          path.Join(cwd, "dynamic-provider/multiple-turns-2"),
			Dependencies: []string{"@pulumi/pulumi"},
		},
		{
			Dir: path.Join(cwd, "compat/v0.10.0/minimal"),
			Config: map[string]string{
				"name": "Pulumi",
			},
			Secrets: map[string]string{
				"secret": "this is my secret message",
			},
			RunBuild: true,
		},
	}

	for _, example := range examples {
		ex := example
		t.Run(example.Dir, func(t *testing.T) {
			integration.ProgramTest(t, &ex)
		})
	}
}
