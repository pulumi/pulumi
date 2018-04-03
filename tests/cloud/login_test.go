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

package cloud

import (
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/backend/cloud"
	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"
)

// requirePulumiAPISet will skip the test unless the PULUMI_API is set.
func requirePulumiAPISet(t *testing.T) {
	if os.Getenv("PULUMI_API") == "" {
		t.Skip("PULUMI_API environment variable not set. Skipping this test.")
	}
}

func TestRequireLogin(t *testing.T) {
	requirePulumiAPISet(t)

	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)

		// logout and confirm auth error.
		e.RunCommand("pulumi", "logout")

		out, err := e.RunCommandExpectError("pulumi", "stack", "init", "foo", "--remote")
		assert.Empty(t, out, "expected no stdout")
		assert.Contains(t, err, "error: you must be logged in to create stacks in the Pulumi Cloud.")
		assert.Contains(t, err, "Run `pulumi login` to log in.")

		// login and confirm things work.
		os.Setenv(cloud.AccessTokenEnvVar, integration.TestAccountAccessToken)
		e.RunCommand("pulumi", "login")

		e.RunCommand("pulumi", "stack", "init", "foo", "--remote")
		e.RunCommand("pulumi", "stack", "rm", "foo", "--yes")
	})
}
