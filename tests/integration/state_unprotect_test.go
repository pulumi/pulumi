// Copyright 2016-2024, Pulumi Corporation.
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
	"runtime"
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
)

// TestUnprotectURN tests the functionality of unprotecting specific resources by URN.
func TestUnprotectURN(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Import the test directory with a protected resource
	e.ImportDirectory("protect_resources/step1")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "unprotect-urn-test")
	e.RunCommand("pulumi", "install")
	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Try to destroy - should fail because the resource is protected
	_, _, err := e.RunCommandReturnExpectedError("pulumi", "destroy", "--skip-preview", "--yes")
	assert.Error(t, err, "expect error from pulumi destroy")
	if runtime.GOOS == "windows" {
		assert.ErrorContains(t, err, "exit status 0xffffffff")
	} else {
		assert.ErrorContains(t, err, "exit status 255")
	}

	// Get the URN of the protected resource
	stdout, _ := e.RunCommand("pulumi", "stack", "--show-urns")
	lines := strings.Split(stdout, "\n")
	var urn string
	for _, line := range lines {
		if strings.Contains(line, "eternal") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				urn = fields[2]
				break
			}
		}
	}
	assert.NotEmpty(t, urn, "Could not find URN for 'eternal' resource")

	// Unprotect the specific resource by URN
	e.RunCommand("pulumi", "state", "unprotect", urn, "--yes")

	// Verify the resource is now unprotected by successfully destroying the stack
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes")
}

// TestUnprotectMultipleURNs tests the functionality of unprotecting multiple resources by URN.
func TestUnprotectMultipleURNs(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Create a custom Pulumi program with multiple protected resources
	const pulumiYaml = `name: unprotect-multiple-test
runtime: nodejs
`
	const indexTS = `
import { Resource } from "./resource";

// Allocate resources and protect them:
let a = new Resource("resource1", { state: 1 }, { protect: true });
let b = new Resource("resource2", { state: 2 }, { protect: true });
`

	// First import the protect_resources directory to get the resource.ts and package.json
	e.ImportDirectory("protect_resources/step1")

	// Then overwrite the specific files we need with our test content
	e.WriteTestFile("Pulumi.yaml", pulumiYaml)
	e.WriteTestFile("index.ts", indexTS)

	// Create the stack and deploy
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "multiple-unprotect-test")
	e.RunCommand("pulumi", "install")
	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Try to destroy - should fail because resources are protected
	_, _, err := e.RunCommandReturnExpectedError("pulumi", "destroy", "--skip-preview", "--yes")
	assert.Error(t, err, "expect error from pulumi destroy")

	// Get the URNs of the protected resources
	stdout, _ := e.RunCommand("pulumi", "stack", "--show-urns")
	lines := strings.Split(stdout, "\n")
	var urns []string
	for _, line := range lines {
		if strings.Contains(line, "resource1") || strings.Contains(line, "resource2") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				urns = append(urns, fields[2])
			}
		}
	}
	assert.Equal(t, 2, len(urns), "Expected to find 2 resource URNs")

	// Unprotect multiple resources in one command
	e.RunCommand("pulumi", "state", "unprotect", urns[0], urns[1], "--yes")

	// Verify resources are now unprotected by successfully destroying the stack
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes")
}
