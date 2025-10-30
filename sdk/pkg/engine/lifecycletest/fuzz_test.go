// Copyright 2024, Pulumi Corporation.
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

package lifecycletest

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/fuzzing"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestFuzz is a harness for running fuzz tests against the lifecycle test framework. It will generate random lifecycle
// test fixtures and run them, hunting for possible snapshot integrity errors.
//
// Usage:
//
// This test will be skipped unless the `PULUMI_LIFECYCLE_TEST_FUZZ` environment variable is set. This is because
// fuzzing is by its nature potentially flaky, and so we want fine-grained control of when we run fuzzing, especially in
// CI where a red build can prevent a PR from being merged.
//
// If you want to customize the fuzzing that occurs, you can modify (but not commit!) the FixtureOptions passed to
// fuzzing.GeneratedFixture in this test to your needs.
func TestFuzz(t *testing.T) {
	t.Parallel()

	shouldFuzz := os.Getenv("PULUMI_LIFECYCLE_TEST_FUZZ")
	if shouldFuzz == "" {
		t.Skip("PULUMI_LIFECYCLE_TEST_FUZZ not set")
	}

	rapid.Check(t, fuzzing.GeneratedFixture(fuzzing.FixtureOptions{}))
}

// TestFuzzFromStateFile is a harness for running fuzz tests starting from a JSON state file such as that produced by a
// `pulumi stack export` command. It can be used to try and reproduce errors that have occurred in a user's stack
// without having access to the actual program that ran, or logs that might help infer the program that ran.
//
// Usage:
//
// This test will be skipped unless the `PULUMI_LIFECYCLE_TEST_FUZZ_FROM_STATE_FILE` environment variable is set. The
// variable should be a path to a suitable JSON file.
//
// If you want to customize the fuzzing that occurs, you can modify (but not commit!) the FixtureOptions passed to
// fuzzing.GeneratedFixture in this test to your needs.
func TestFuzzFromStateFile(t *testing.T) {
	t.Parallel()

	stateFile := os.Getenv("PULUMI_LIFECYCLE_TEST_FUZZ_FROM_STATE_FILE")
	if stateFile == "" {
		t.Skip("PULUMI_LIFECYCLE_TEST_FUZZ_FROM_STATE_FILE not set")
	}

	reader, err := os.Open(stateFile)
	require.NoError(t, err)

	var deployment apitype.UntypedDeployment
	err = json.NewDecoder(reader).Decode(&deployment)
	require.NoError(t, err)

	v3Deployment, err := stack.UnmarshalUntypedDeployment(context.Background(), &deployment)
	require.NoError(t, err)

	if len(v3Deployment.Resources) == 0 {
		t.Skip("No resources in state file")
	}

	first := v3Deployment.Resources[0]
	project := first.URN.Project()
	stack := first.URN.Stack()

	rapid.Check(t, fuzzing.GeneratedFixture(fuzzing.FixtureOptions{
		StackSpecOptions: fuzzing.StackSpecOptions{
			Project: string(project),
			Stack:   string(stack),
		},
		SnapshotSpecOptions: fuzzing.SnapshotSpecOptions{
			SourceDeploymentV3: v3Deployment,
		},
	}))
}
