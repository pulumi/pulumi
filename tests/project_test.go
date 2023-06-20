// Copyright 2016-2023, Pulumi Corporation.
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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

// https://github.com/pulumi/pulumi/issues/11264
func TestProjectBadName(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	pulumiProject := `
name: pulumi/proj
runtime: yaml
`

	integration.CreatePulumiRepo(e, pulumiProject)
	e.SetBackend(e.LocalURL())
	_, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "dev")
	assert.Contains(t, stderr, "project names may only contain alphanumerics, hyphens, underscores, and periods")
}
