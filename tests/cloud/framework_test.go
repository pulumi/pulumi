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

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// TestRunTestInfrastructureAgainstService is a copy of the "TestStackOutputs" integration test, but configured to
// run against the Pulumi Service located at PULUMI_API (if set).
func TestRunTestInfrastructureAgainstService(t *testing.T) {
	requirePulumiAPISet(t)

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "../integration/stack_outputs",
		Dependencies: []string{"pulumi"},
		Quick:        true,

		// Options specific to testing against the service. All environments have a "default" PPC in the moolumi org.
		CloudURL: os.Getenv("PULUMI_API"),
		Owner:    "moolumi",
		Repo:     "pulumi",
		PPCName:  "default",
	})
}
