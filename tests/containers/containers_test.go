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

package containers

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

// TestPulumiDockerImage simulates building and running Pulumi programs on the pulumi/pulumi Docker image.
//
// NOTE: This test is intended to be run inside the aforementioned container, unlike the actions test below.
func TestPulumiDockerImage(t *testing.T) {
	const stackOwner = "moolumi"

	if os.Getenv("RUN_CONTAINER_TESTS") == "" {
		t.Skip("Skipping container runtime tests because RUN_CONTAINER_TESTS not set.")
	}

	// Confirm we have credentials.
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Fatal("PULUMI_ACCESS_TOKEN not found, aborting tests.")
	}

	base := integration.ProgramTestOptions{
		Tracing:              "https://tracing.pulumi-engineering.com/collector/api/v1/spans",
		ExpectRefreshChanges: true,
		Quick:                true,
		SkipRefresh:          true,
		NoParallel:           true, // we mark tests as Parallel manually when instantiating
	}

	for _, template := range []string{"csharp", "python", "typescript"} {
		t.Run(template, func(t *testing.T) {
			t.Parallel()

			e := ptesting.NewEnvironment(t)
			defer func() {
				e.RunCommand("pulumi", "stack", "rm", "--force", "--yes")
				e.DeleteEnvironment()
			}()

			stackName := fmt.Sprintf("%s/container-%s-%x", stackOwner, template, time.Now().UnixNano())
			e.RunCommand("pulumi", "new", template, "-y", "-f", "-s", stackName)

			example := base.With(integration.ProgramTestOptions{
				Dir: e.RootPath,
			})

			integration.ProgramTest(t, &example)
		})
	}
}
