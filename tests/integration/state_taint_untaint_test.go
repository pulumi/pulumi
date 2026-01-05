// Copyright 2025, Pulumi Corporation.
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

package ints

import (
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaintReplaceResource tests the taint/untaint workflow
func TestTaintReplaceResource(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.WriteTestFile("index.js", `
const pulumi = require("@pulumi/pulumi");

// Resource is a dynamic provider resource that can be used for testing
class DynamicResource extends pulumi.dynamic.Resource {
    constructor(name, opts) {
        super({
            create: (inputs) => Promise.resolve({ id: name, outs: { output: "output" } }),
            update: (id, inputs) => Promise.resolve({ outs: { output: "output" } }),
            delete: (id, props) => Promise.resolve(),
        }, name, { output: undefined }, opts);
    }
}

const resource1 = new DynamicResource("myresource");

exports.resource1Id = resource1.id;
`)

	e.WriteTestFile("package.json", `{
		"name": "taint-test",
		"main": "index.js",
		"dependencies": {
			"@pulumi/pulumi": "^3.0.0"
		}
	}`)

	e.WriteTestFile("Pulumi.yaml", `name: taint-test
runtime: nodejs
description: Test taint functionality
`)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "dev")
	e.RunCommand("pulumi", "install")

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Get the URN of the test resource
	var resourceURN string
	stdout, _ := e.RunCommand("pulumi", "stack", "--show-urns")
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "pulumi-nodejs:dynamic:Resource") {
			// This is our dynamic resource - extract the URN
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "urn:pulumi:") {
					resourceURN = field
					break
				}
			}
		}
	}
	require.NotEmpty(t, resourceURN, "Resource URN not found")

	// Taint the resource
	stdout, _ = e.RunCommand("pulumi", "state", "taint", resourceURN, "--yes")
	assert.Contains(t, stdout, "1 resources tainted", "Expected success message")

	// Run preview to see the resource will be replaced
	stdout, _ = e.RunCommand("pulumi", "preview", "--json")
	assert.Contains(t, stdout, "replace", "Preview should show resource will be replaced")

	// Run update to replace the tainted resource
	stdout, _ = e.RunCommand("pulumi", "up", "--yes", "--skip-preview")
	assert.Contains(t, stdout, "replaced", "Update should show resource was replaced")

	// Verify the resource is no longer tainted by running another preview
	stdout, _ = e.RunCommand("pulumi", "preview")
	assert.Contains(t, stdout, "2 unchanged", "Should have no changes after replacing tainted resource")
}

// TestTaintMultipleResources tests tainting multiple resources at once
func TestTaintMultipleResources(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.WriteTestFile("index.js", `
const pulumi = require("@pulumi/pulumi");

// Resource is a dynamic provider resource that can be used for testing
class DynamicResource extends pulumi.dynamic.Resource {
    constructor(name, opts) {
        super({
            create: (inputs) => Promise.resolve({ id: name, outs: { output: "output" } }),
            update: (id, inputs) => Promise.resolve({ outs: { output: "output" } }),
            delete: (id, props) => Promise.resolve(),
        }, name, { output: undefined }, opts);
    }
}

// Create multiple resources for testing
const resource1 = new DynamicResource("resource1");
const resource2 = new DynamicResource("resource2");
const resource3 = new DynamicResource("resource3");

exports.resource1Id = resource1.id;
exports.resource2Id = resource2.id;
exports.resource3Id = resource3.id;
`)

	e.WriteTestFile("package.json", `{
		"name": "taint-multiple-test",
		"main": "index.js",
		"dependencies": {
			"@pulumi/pulumi": "latest"
		}
	}`)

	e.WriteTestFile("Pulumi.yaml", `name: taint-multiple-test
runtime: nodejs
description: Test taint multiple resources
`)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "dev")
	e.RunCommand("pulumi", "install")

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Get URNs of resources
	var resource1URN, resource2URN string
	stdout, _ := e.RunCommand("pulumi", "stack", "--show-urns")
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "pulumi-nodejs:dynamic:Resource") {
			// This is our dynamic resource - extract the URN
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "urn:pulumi:") {
					if strings.Contains(field, "resource1") {
						resource1URN = field
					} else if strings.Contains(field, "resource2") {
						resource2URN = field
					}
				}
			}
		}
	}
	require.NotEmpty(t, resource1URN, "Resource1 URN not found")
	require.NotEmpty(t, resource2URN, "Resource2 URN not found")

	// Taint multiple resources
	stdout, _ = e.RunCommand("pulumi", "state", "taint", resource1URN, resource2URN, "--yes")
	assert.Contains(t, stdout, "2 resources tainted", "Expected 2 resources tainted")

	// Run update to replace both tainted resources
	stdout, _ = e.RunCommand("pulumi", "up", "--yes", "--skip-preview")
	// Should show 2 resources replaced
	assert.Contains(t, stdout, "replaced", "Update should show resources were replaced")
}

// TestUntaintResource tests untainting a tainted resource
func TestUntaintResource(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.WriteTestFile("index.js", `
const pulumi = require("@pulumi/pulumi");

// Resource is a dynamic provider resource that can be used for testing
class DynamicResource extends pulumi.dynamic.Resource {
    constructor(name, opts) {
        super({
            create: (inputs) => Promise.resolve({ id: name, outs: { output: "output" } }),
            update: (id, inputs) => Promise.resolve({ outs: { output: "output" } }),
            delete: (id, props) => Promise.resolve(),
        }, name, { output: undefined }, opts);
    }
}

const resource1 = new DynamicResource("myresource");

exports.resource1Id = resource1.id;
`)

	e.WriteTestFile("package.json", `{
		"name": "untaint-test",
		"main": "index.js",
		"dependencies": {
			"@pulumi/pulumi": "latest"
		}
	}`)

	e.WriteTestFile("Pulumi.yaml", `name: untaint-test
runtime: nodejs
description: Test untaint functionality
`)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "dev")
	e.RunCommand("pulumi", "install")

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Get the URN of the test resource
	var resourceURN string
	stdout, _ := e.RunCommand("pulumi", "stack", "--show-urns")
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "pulumi-nodejs:dynamic:Resource") {
			// This is our dynamic resource - extract the URN
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "urn:pulumi:") {
					resourceURN = field
					break
				}
			}
		}
	}
	require.NotEmpty(t, resourceURN, "Resource URN not found")

	// Taint the resource
	stdout, _ = e.RunCommand("pulumi", "state", "taint", resourceURN, "--yes")
	assert.Contains(t, stdout, "1 resources tainted", "Expected success message")

	// Preview should show it will be replaced
	stdout, _ = e.RunCommand("pulumi", "preview", "--json")
	assert.Contains(t, stdout, "replace", "Preview should show resource will be replaced")

	// Untaint the resource
	stdout, _ = e.RunCommand("pulumi", "state", "untaint", resourceURN, "--yes")
	assert.Contains(t, stdout, "1 resources untainted", "Expected success message")

	// Preview should now show no changes
	stdout, _ = e.RunCommand("pulumi", "preview")
	assert.Contains(t, strings.ToLower(stdout), "2 unchanged", "Should have no changes after untainting resource")
}

// TestUntaintAll tests the --all flag for untaint
func TestUntaintAll(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.WriteTestFile("index.js", `
const pulumi = require("@pulumi/pulumi");

// Resource is a dynamic provider resource that can be used for testing
class DynamicResource extends pulumi.dynamic.Resource {
    constructor(name, opts) {
        super({
            create: (inputs) => Promise.resolve({ id: name, outs: { output: "output" } }),
            update: (id, inputs) => Promise.resolve({ outs: { output: "output" } }),
            delete: (id, props) => Promise.resolve(),
        }, name, { output: undefined }, opts);
    }
}

// Create multiple resources for testing
const resource1 = new DynamicResource("resource1");
const resource2 = new DynamicResource("resource2");
const resource3 = new DynamicResource("resource3");

exports.resource1Id = resource1.id;
exports.resource2Id = resource2.id;
exports.resource3Id = resource3.id;
`)

	e.WriteTestFile("package.json", `{
		"name": "untaint-all-test",
		"main": "index.js",
		"dependencies": {
			"@pulumi/pulumi": "latest"
		}
	}`)

	e.WriteTestFile("Pulumi.yaml", `name: untaint-all-test
runtime: nodejs
description: Test untaint all functionality
`)

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "dev")
	e.RunCommand("pulumi", "install")

	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	// Get URNs of all resources
	var urns []string
	stdout, _ := e.RunCommand("pulumi", "stack", "--show-urns")
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "pulumi-nodejs:dynamic:Resource") {
			// This is our dynamic resource - extract the URN
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "urn:pulumi:") {
					urns = append(urns, field)
					break
				}
			}
		}
	}
	require.Len(t, urns, 3, "Should have 3 resources")

	// Taint all resources
	args := append([]string{"state", "taint", "--yes"}, urns...)
	stdout, _ = e.RunCommand("pulumi", args...)
	assert.Contains(t, stdout, "3 resources tainted", "Expected 3 resources tainted")

	// Untaint all resources
	stdout, _ = e.RunCommand("pulumi", "state", "untaint", "--all", "--yes")
	assert.Contains(t, stdout, "All resources untainted", "Expected all resources untainted")

	// Preview should show no changes
	stdout, _ = e.RunCommand("pulumi", "preview")
	assert.Contains(t, stdout, "4 unchanged", "Should have no changes after untainting all")
}
