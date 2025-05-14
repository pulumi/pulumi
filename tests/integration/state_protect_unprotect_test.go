// Copyright 2016-2025, Pulumi Corporation.
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

//go:build all

package ints

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
)

// TestProtectAndUnprotectMultipleURNs tests the functionality of protecting and unprotecting multiple resources by URN.
func TestProtectAndUnprotectMultipleURNs(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Create a custom Pulumi program with multiple initially unprotected resources
	const pulumiYaml = `name: protect-unprotect-test
runtime: nodejs
`
	const indexTS = `
import { Resource } from "./resource";

// Allocate resources (initially unprotected):
let a = new Resource("resource1", { state: 1 });
let b = new Resource("resource2", { state: 2 });
let c = new Resource("resource3", { state: 3 });
`

	// First import the protect_resources directory to get the resource.ts and package.json
	e.ImportDirectory("protect_resources/step1")

	// Then overwrite the specific files we need with our test content
	e.WriteTestFile("Pulumi.yaml", pulumiYaml)
	e.WriteTestFile("index.ts", indexTS)

	// Create the stack and deploy
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "protect-unprotect-test")
	e.RunCommand("pulumi", "install")
	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Verify resources are initially unprotected by trying to destroy
	// But cancel the destroy to keep testing
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes=false")

	// Get the URNs of all resources
	stdout, _ := e.RunCommand("pulumi", "stack", "--show-urns")
	lines := strings.Split(stdout, "\n")
	var urns []string

	// Look for resources created by our test - find dynamic resources from our provider
	for _, line := range lines {
		if strings.Contains(line, "pulumi:dynamic:Resource::") {
			// This is our custom dynamic resource - extract the URN
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "urn:pulumi:") {
					urns = append(urns, field)
					break
				}
			}
		}
	}

	// Log the output to help debug
	t.Logf("Stack output: %s", stdout)
	t.Logf("Found %d URNs: %v", len(urns), urns)
	assert.Equal(t, 3, len(urns), "Expected to find 3 resource URNs")

	// Add safety check to prevent panic if we don't have enough URNs
	if len(urns) < 2 {
		t.Fatalf("Not enough resource URNs found, expected at least 2, got %d", len(urns))
	}

	// STEP 1: Protect multiple resources in one command by passing all URNs
	protectArgs := append([]string{"pulumi", "state", "protect", "--yes"}, urns[:2]...)
	protectOutput, _ := e.RunCommand(protectArgs[0], protectArgs[1:]...)

	// Verify the protect output shows the correct count
	assert.Contains(t, protectOutput, "2 resources protected",
		"Protect command should report the correct number of resources")

	// Try to destroy - should fail because resources are now protected
	_, _, err := e.RunCommandReturnExpectedError("pulumi", "destroy", "--skip-preview", "--yes")
	assert.Error(t, err, "expect error from pulumi destroy after protect")
	if runtime.GOOS == "windows" {
		assert.ErrorContains(t, err, "exit status 0xffffffff")
	} else {
		assert.ErrorContains(t, err, "exit status 255")
	}

	// STEP 2: Unprotect a subset of resources to verify partial unprotect works
	unprotectSubsetArgs := append([]string{"pulumi", "state", "unprotect", "--yes"}, urns[0])
	unprotectOutput, _ := e.RunCommand(unprotectSubsetArgs[0], unprotectSubsetArgs[1:]...)

	// Verify the unprotect output shows the correct count
	assert.Contains(t, unprotectOutput, "1 resources unprotected",
		"Unprotect command for a single resource should report count of 1")

	// Try to destroy - should still fail because one resource is still protected
	_, _, err = e.RunCommandReturnExpectedError("pulumi", "destroy", "--skip-preview", "--yes")
	assert.Error(t, err, "expect error from pulumi destroy after partial unprotect")

	// STEP 3: Unprotect the remaining protected resource
	unprotectArgs := append([]string{"pulumi", "state", "unprotect", "--yes"}, urns[1])
	unprotectOutput, _ = e.RunCommand(unprotectArgs[0], unprotectArgs[1:]...)

	// Verify the unprotect output shows the correct count
	assert.Contains(t, unprotectOutput, "1 resources unprotected",
		"Unprotect command for a single resource should report count of 1")

	// Verify resources are now unprotected by successfully destroying the stack
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes")
}

// TestProtectAndUnprotectAllResources tests the functionality of protecting and unprotecting all resources
// using the --all flag.
func TestProtectAndUnprotectAllResources(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Create a custom Pulumi program with multiple initially unprotected resources
	const pulumiYaml = `name: protect-all-test
runtime: nodejs
`
	const indexTS = `
import { Resource } from "./resource";

// Allocate multiple resources (initially unprotected):
let a = new Resource("resource1", { state: 1 });
let b = new Resource("resource2", { state: 2 });
let c = new Resource("resource3", { state: 3 });
let d = new Resource("resource4", { state: 4 });
`

	// First import the protect_resources directory to get the resource.ts and package.json
	e.ImportDirectory("protect_resources/step1")

	// Then overwrite the specific files we need with our test content
	e.WriteTestFile("Pulumi.yaml", pulumiYaml)
	e.WriteTestFile("index.ts", indexTS)

	// Create the stack and deploy
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "protect-all-test")
	e.RunCommand("pulumi", "install")
	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Get resources count before the test
	stdout, _ := e.RunCommand("pulumi", "stack", "--show-urns")
	lines := strings.Split(stdout, "\n")
	var resourceCount int
	for _, line := range lines {
		if strings.Contains(line, "Resource::resource") {
			resourceCount++
		}
	}
	assert.True(t, resourceCount >= 4, fmt.Sprintf("Expected at least 4 resources, got %d", resourceCount))

	// STEP 1: Protect all resources using the --all flag
	protectOutput, _ := e.RunCommand("pulumi", "state", "protect", "--all", "--yes")
	assert.Contains(t, protectOutput, "All resources protected")

	// Try to destroy - should fail because all resources are now protected
	_, _, err := e.RunCommandReturnExpectedError("pulumi", "destroy", "--skip-preview", "--yes")
	assert.Error(t, err, "expect error from pulumi destroy after protect all")

	// STEP 2: Unprotect all resources using the --all flag
	unprotectOutput, _ := e.RunCommand("pulumi", "state", "unprotect", "--all", "--yes")
	assert.Contains(t, unprotectOutput, "All resources unprotected")

	// Verify resources are now unprotected by successfully destroying the stack
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes")
}

// TestProtectAndUnprotectIndividualURN tests protecting and unprotecting a single resource by URN.
func TestProtectAndUnprotectIndividualURN(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Import the test directory with a single unprotected resource
	e.ImportDirectory("protect_resources/step1")
	// Modify the index.ts to create an unprotected resource initially
	const indexTS = `
import { Resource } from "./resource";

// Allocate resource (initially unprotected):
let eternal = new Resource("eternal", { state: 42 });
`
	e.WriteTestFile("index.ts", indexTS)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "single-protect-unprotect-test")
	e.RunCommand("pulumi", "install")
	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Get the URN of the resource
	stdout, _ := e.RunCommand("pulumi", "stack", "--show-urns")
	lines := strings.Split(stdout, "\n")
	var urn string
	for _, line := range lines {
		if strings.Contains(line, "eternal") {
			fields := strings.Fields(line)
			for _, field := range fields {
				// Look for the URN format
				if strings.HasPrefix(field, "urn:pulumi:") {
					urn = field
					break
				}
			}
		}
	}
	assert.NotEmpty(t, urn, "Could not find URN for 'eternal' resource")

	// STEP 1: Protect the specific resource by URN
	e.RunCommand("pulumi", "state", "protect", urn, "--yes")

	// Try to destroy - should fail because the resource is protected
	_, _, err := e.RunCommandReturnExpectedError("pulumi", "destroy", "--skip-preview", "--yes")
	assert.Error(t, err, "expect error from pulumi destroy after protect")

	// STEP 2: Unprotect the specific resource by URN
	e.RunCommand("pulumi", "state", "unprotect", urn, "--yes")

	// Verify the resource is now unprotected by successfully destroying the stack
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes")
}
