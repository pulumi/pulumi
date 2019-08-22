// Copyright 2016-2018, Pulumi Corporation.
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

package tests

import (
	"os"
	"testing"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestTemplates(t *testing.T) {
	overrides, err := integration.DecodeMapString(os.Getenv("PULUMI_TEST_NODE_OVERRIDES"))
	if !assert.NoError(t, err, "expected valid override map: %v", err) {
		return
	}

	base := integration.ProgramTestOptions{
		Tracing:              "https://tracing.pulumi-engineering.com/collector/api/v1/spans",
		ExpectRefreshChanges: true,
		Overrides:            overrides,
		Quick:                true,
		SkipRefresh:          true,
	}

	tests := []string{"typescript"}
	for _, templateName := range tests {
		t.Run(templateName, func(t *testing.T) {
			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			e.RunCommand("pulumi", "new", templateName, "-f")

			path, err := workspace.DetectProjectPathFrom(e.RootPath)
			assert.NoError(t, err)
			_, err = workspace.LoadProject(path)
			assert.NoError(t, err)

			example := base.With(integration.ProgramTestOptions{
				Dir: e.RootPath,
			})

			integration.ProgramTest(t, &example)
		})
	}
}
