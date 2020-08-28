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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v2/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v2/go/common/testing"
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

// TestPulumiActionsImage simulates building and running Pulumi programs on the pulumi/actions image.
//
// The main codepath being tested is the entrypoint script of the container, which contains logic for
// downloading dependencies, honoring various environment variables, etc.
func TestPulumiActionsImage(t *testing.T) {
	const pulumiContainerToTest = "pulumi/actions:latest"

	if os.Getenv("RUN_CONTAINER_TESTS") == "" {
		t.Skip("Skipping container runtime tests because RUN_CONTAINER_TESTS not set.")
	}

	// Confirm we have credentials.
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Fatal("PULUMI_ACCESS_TOKEN not found, aborting tests.")
	}

	// MacOS workaround. os.TempDir returns a path under /var/, which isn't
	// bindable in default Docker installs. So we override the behavior to
	// use /tmp, which should work.
	if strings.HasPrefix(os.TempDir(), "/var/") {
		os.Setenv("TMPDIR", "/tmp")
	}

	// Confirm the container has been built, will emit no output if it isn't found.
	e := ptesting.NewEnvironment(t)
	stdout, _ := e.RunCommand("docker", "images", pulumiContainerToTest, "--quiet")
	if len(stdout) == 0 {
		t.Fatalf("It doesn't appear that the container image %s has been built.", pulumiContainerToTest)
	}
	e.DeleteEnvironment()

	t.Run("dotnet", func(t *testing.T) {
		testRuntimeWorksInContainer(t, "dotnet", pulumiContainerToTest)
	})

	t.Run("nodejs", func(t *testing.T) {
		testRuntimeWorksInContainer(t, "nodejs", pulumiContainerToTest)
	})

	t.Run("python", func(t *testing.T) {
		testRuntimeWorksInContainer(t, "python", pulumiContainerToTest)
	})

	t.Run("python_venv", func(t *testing.T) {
		testRuntimeWorksInContainer(t, "python_venv", pulumiContainerToTest)
	})
}

// testRuntimeWorksInContainer runs a test that attempts to run a Pulumi program in the given
// container. It is assumed that Pulumi program has a configuration key named "runtime" and
// will print "Hello from {{ runtime }}".
func testRuntimeWorksInContainer(t *testing.T, runtime, container string) {
	const stackOwner = "moolumi"

	stackName := fmt.Sprintf("%s/container-%s-%x", stackOwner, runtime, time.Now().UnixNano())

	e := ptesting.NewEnvironment(t)
	defer func() {
		e.RunCommand("pulumi", "stack", "rm", "--force", "--yes")
		// BUG: We mount /tmp/${e.CWD} into the container, which contains the Pulumi program,
		// and then the container executes and leaves new files there too. Since code in the
		// container ( by default) runs as root (at least that's the default behavior on Linux),
		// then on CI we don't have access to call e.DeleteEnvironment(). (Since files created
		// on the container are owned by "root root".) We should fix this, since as-is we
		// are leaving crap in the temp folder.
		t.Logf("NOTE: Skipping cleanup of test environment. Leaving files in %q", e.CWD)
	}()
	e.ImportDirectory(runtime)

	// Create the stack and set the required configuration.
	e.RunCommand("pulumi", "stack", "init", stackName)
	e.RunCommand("pulumi", "config", "set", "runtime", runtime)

	// Run `pulumi up` using the Pulumi container.
	stdout, _ := e.RunCommand("docker", "run",
		// Set the PULUMI_ACCESS_TOKEN environment variable, to authenticate.
		// BUG: This is why the these tests aren't configured to run as part of CI,
		// since the access token would get written to logs.
		"--env", fmt.Sprintf("PULUMI_ACCESS_TOKEN=%s", os.Getenv("PULUMI_ACCESS_TOKEN")),
		// Mount the stack's source code into the container.
		"--volume", fmt.Sprintf("%s:/src", e.CWD),
		// Set working directory when running the container.
		"--workdir", "/src",
		// Cleanup the container on shutdown.
		"--rm",
		// Container to run.
		container,
		// Flags to the container's entry point (`pulumi`).
		"up", "--stack", stackName, "--yes")

	assert.Contains(t, stdout, "Hello from "+runtime,
		"Looking for indication stack update was successful in container output.")
}
