// Copyright 2017-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		Dependencies: []string{"@pulumi/pulumi"},
		Config: map[string]string{
			"name": "Pulumi",
		},
		Secrets: map[string]string{
			"secret": "this is my secret message",
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Simple runtime validation that just ensures the checkpoint was written and read.
			assert.Equal(t, minimal.GetStackName(), stackInfo.Checkpoint.Stack)
		},
		ReportStats: integration.NewS3Reporter("us-west-2", "eng.pulumi.com", "testreports"),
	}

	var formattableStdout, formattableStderr bytes.Buffer
	examples := []integration.ProgramTestOptions{
		minimal,
		{
			Dir:          path.Join(cwd, "dynamic-provider/simple"),
			Dependencies: []string{"@pulumi/pulumi"},
			Config: map[string]string{
				"simple:config:w": "1",
				"simple:config:x": "1",
				"simple:config:y": "1",
			},
			Verbose:       true,
			DebugUpdates:  true,
			DebugLogLevel: 12,
		},
		{
			Dir:          path.Join(cwd, "dynamic-provider/multiple-turns"),
			Dependencies: []string{"@pulumi/pulumi"},
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				for _, res := range stackInfo.Snapshot.Resources {
					if res.Parent == "" {
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
			Dir:          path.Join(cwd, "compat/v0.10.0/minimal"),
			Dependencies: []string{"@pulumi/pulumi"},
			Config: map[string]string{
				"name": "Pulumi",
			},
			Secrets: map[string]string{
				"secret": "this is my secret message",
			},
		},
	}

	for _, ex := range examples {
		example := ex
		t.Run(example.Dir, func(t *testing.T) {
			integration.ProgramTest(t, &example)
		})
	}
}
