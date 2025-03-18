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

//go:build !xplatform_acceptance

package ints

import (
	"encoding/json"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
)

// ExcludeTargetsDetectionTest tests that the exclude-targets flag properly excludes
// resources from being destroyed.
func TestExcludeTargets(t *testing.T) {
	t.Parallel()

	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// First just spin up the project.
	projName := "exclude_targets_test"
	stackName, err := resource.NewUniqueHex("test-", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	// Create the test directory
	testDirPath := path.Join(e.RootPath, projName)
	err = os.MkdirAll(testDirPath, 0o755)
	contract.AssertNoErrorf(err, "os.MkdirAll should not have failed")

	// Create package.json
	packageJSON := `{
		"name": "exclude_targets_test",
		"version": "1.0.0",
		"dependencies": {
			"@pulumi/pulumi": "latest"
		}
	}`
	err = os.WriteFile(path.Join(testDirPath, "package.json"), []byte(packageJSON), 0o644)
	contract.AssertNoErrorf(err, "os.WriteFile should not have failed for package.json")

	// Create index.ts with three resources
	indexTS := `import * as pulumi from "@pulumi/pulumi";

// Create three dynamic resources
const provider = new pulumi.ProviderResource("test", "test-provider");

const resourceA = new pulumi.CustomResource("test:resource:A", "resourceA", {}, { provider });
const resourceB = new pulumi.CustomResource("test:resource:B", "resourceB", {}, { provider });
const resourceC = new pulumi.CustomResource("test:resource:C", "resourceC", {}, { provider });

// Export the URNs
export const resourceAURN = resourceA.urn;
export const resourceBURN = resourceB.urn;
export const resourceCURN = resourceC.urn;
`
	err = os.WriteFile(path.Join(testDirPath, "index.ts"), []byte(indexTS), 0o644)
	contract.AssertNoErrorf(err, "os.WriteFile should not have failed for index.ts")

	// Change to the test directory
	e.CWD = testDirPath

	// Initialize the stack and deploy it
	e.RunCommand("pulumi", "stack", "init", stackName)
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")
	e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview", "--yes")

	// Get the URNs of the created resources
	resourceAURNOutput, _ := e.RunCommand("pulumi", "stack", "output", "resourceAURN")
	resourceBURNOutput, _ := e.RunCommand("pulumi", "stack", "output", "resourceBURN")

	resourceAURN := strings.TrimSpace(resourceAURNOutput)
	resourceBURN := strings.TrimSpace(resourceBURNOutput)

	// Test 1: Destroy with --exclude-target to exclude resourceA and resourceB
	e.RunCommand("pulumi", "destroy", "--non-interactive", "--skip-preview", "--yes",
		"--exclude-target", resourceAURN, "--exclude-target", resourceBURN)

	// Verify that resourceA and resourceB were not destroyed
	stackJSON, _ := e.RunCommand("pulumi", "stack", "export")
	var stackData map[string]interface{}
	err = json.Unmarshal([]byte(stackJSON), &stackData)
	contract.AssertNoErrorf(err, "json.Unmarshal should not have failed for stack export")

	// Search through the stack deployment resources to find our resources
	resources := stackData["deployment"].(map[string]interface{})["resources"].([]interface{})

	// Count of each resource type
	var resourceACount, resourceBCount, resourceCCount int

	for _, res := range resources {
		urn := res.(map[string]interface{})["urn"].(string)
		if strings.Contains(urn, "resourceA") {
			resourceACount++
		} else if strings.Contains(urn, "resourceB") {
			resourceBCount++
		} else if strings.Contains(urn, "resourceC") {
			resourceCCount++
		}
	}

	// Assert that resourceA and resourceB are still in the stack, but resourceC is gone
	assert.Equal(t, 1, resourceACount, "resourceA should still be in the stack")
	assert.Equal(t, 1, resourceBCount, "resourceB should still be in the stack")
	assert.Equal(t, 0, resourceCCount, "resourceC should have been destroyed")

	// Now destroy everything to clean up
	e.RunCommand("pulumi", "destroy", "--non-interactive", "--skip-preview", "--yes")
	e.RunCommand("pulumi", "stack", "rm", "--yes")
}

// TestExcludeTargetDependents tests that the exclude-target-dependents flag
// properly excludes dependents of excluded resources from being destroyed.
func TestExcludeTargetDependents(t *testing.T) {
	t.Parallel()

	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// First just spin up the project.
	projName := "exclude_target_deps_test"
	stackName, err := resource.NewUniqueHex("test-", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	// Create the test directory
	testDirPath := path.Join(e.RootPath, projName)
	err = os.MkdirAll(testDirPath, 0o755)
	contract.AssertNoErrorf(err, "os.MkdirAll should not have failed")

	// Create package.json
	packageJSON := `{
		"name": "exclude_target_deps_test",
		"version": "1.0.0",
		"dependencies": {
			"@pulumi/pulumi": "latest"
		}
	}`
	err = os.WriteFile(path.Join(testDirPath, "package.json"), []byte(packageJSON), 0o644)
	contract.AssertNoErrorf(err, "os.WriteFile should not have failed for package.json")

	// Create index.ts with resources that have dependencies
	indexTS := `import * as pulumi from "@pulumi/pulumi";

// Create dynamic resources with dependencies
const provider = new pulumi.ProviderResource("test", "test-provider");

// Set up the dependency chain: C <- B <- A
// Where A depends on B, and B depends on C
const resourceC = new pulumi.CustomResource("test:resource:C", "resourceC", {}, { provider });

const resourceB = new pulumi.CustomResource("test:resource:B", "resourceB", {}, {
    provider,
    dependsOn: [resourceC]
});

const resourceA = new pulumi.CustomResource("test:resource:A", "resourceA", {}, {
    provider,
    dependsOn: [resourceB]
});

// Two independent resources
const resourceD = new pulumi.CustomResource("test:resource:D", "resourceD", {}, { provider });
const resourceE = new pulumi.CustomResource("test:resource:E", "resourceE", {}, { provider });

// Export the URNs
export const resourceAURN = resourceA.urn;
export const resourceBURN = resourceB.urn;
export const resourceCURN = resourceC.urn;
export const resourceDURN = resourceD.urn;
export const resourceEURN = resourceE.urn;
`
	err = os.WriteFile(path.Join(testDirPath, "index.ts"), []byte(indexTS), 0o644)
	contract.AssertNoErrorf(err, "os.WriteFile should not have failed for index.ts")

	// Change to the test directory
	e.CWD = testDirPath

	// Initialize the stack and deploy it
	e.RunCommand("pulumi", "stack", "init", stackName)
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")
	e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview", "--yes")

	// Get the URNs of the created resources
	resourceBURNOutput, _ := e.RunCommand("pulumi", "stack", "output", "resourceBURN")
	resourceBURN := strings.TrimSpace(resourceBURNOutput)

	// Test: Destroy with --exclude-target and --exclude-target-dependents
	e.RunCommand("pulumi", "destroy", "--non-interactive", "--skip-preview", "--yes",
		"--exclude-target", resourceBURN, "--exclude-target-dependents")

	// Verify that the resources have the expected state
	stackJSON, _ := e.RunCommand("pulumi", "stack", "export")
	var stackData map[string]interface{}
	err = json.Unmarshal([]byte(stackJSON), &stackData)
	contract.AssertNoErrorf(err, "json.Unmarshal should not have failed for stack export")

	// Resources in the stack
	resources := stackData["deployment"].(map[string]interface{})["resources"].([]interface{})

	// Count of each resource type
	resourceCounts := make(map[string]int)

	for _, res := range resources {
		urn := res.(map[string]interface{})["urn"].(string)
		for _, name := range []string{"resourceA", "resourceB", "resourceC", "resourceD", "resourceE"} {
			if strings.Contains(urn, name) {
				resourceCounts[name]++
			}
		}
	}

	// Assert that A, B, and C are still in the stack because they form a dependency chain
	// with B as the excluded target, but D and E should be gone
	assert.Equal(t, 1, resourceCounts["resourceA"], "resourceA should still be in the stack")
	assert.Equal(t, 1, resourceCounts["resourceB"], "resourceB should still be in the stack")
	assert.Equal(t, 1, resourceCounts["resourceC"], "resourceC should still be in the stack")
	assert.Equal(t, 0, resourceCounts["resourceD"], "resourceD should have been destroyed")
	assert.Equal(t, 0, resourceCounts["resourceE"], "resourceE should have been destroyed")

	// Now destroy everything to clean up
	e.RunCommand("pulumi", "destroy", "--non-interactive", "--skip-preview", "--yes")
	e.RunCommand("pulumi", "stack", "rm", "--yes")
}
