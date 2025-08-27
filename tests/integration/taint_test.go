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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

// TestTaintReplaceResource tests the taint/untaint workflow
func TestTaintReplaceResource(t *testing.T) {
	t.Parallel()

	// Create a simple Node.js program that creates a resource
	dir := t.TempDir()

	// Write a simple Pulumi program
	program := `
const pulumi = require("@pulumi/pulumi");

class MyResource extends pulumi.CustomResource {
    constructor(name, args, opts) {
        super("test:index:MyResource", name, args, opts);
    }
}

const resource1 = new MyResource("myresource", {
    prop1: "value1",
});

exports.resource1Id = resource1.id;
`

	err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(program), 0600)
	require.NoError(t, err)

	// Write package.json
	packageJSON := `{
  "name": "taint-test",
  "main": "index.js",
  "dependencies": {
    "@pulumi/pulumi": "^3.0.0"
  }
}
`
	err = os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0600)
	require.NoError(t, err)

	// Write Pulumi.yaml
	pulumiYAML := `name: taint-test
runtime: nodejs
description: Test taint functionality
`
	err = os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(pulumiYAML), 0600)
	require.NoError(t, err)

	e := &integration.ProgramTestOptions{
		Dir:             dir,
		Dependencies:    []string{"@pulumi/pulumi"},
		Quick:           true,
		SkipRefresh:     true,
		NoParallel:      true,
		PrepareProject:  func(*engine.Projinfo) error { return nil },

		// Run the workflow
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			// Step 1: Get the URN of the resource
			var resourceURN string
			for _, res := range stack.Deployment.Resources {
				if res.Type == "test:index:MyResource" {
					resourceURN = string(res.URN)
					break
				}
			}
			require.NotEmpty(t, resourceURN, "MyResource URN not found")

			// Step 2: Taint the resource
			stdout, stderr, err := stack.RunCommand("pulumi", "state", "taint", resourceURN, "--yes")
			require.NoError(t, err, "Failed to taint resource: %s\n%s", stdout, stderr)
			assert.Contains(t, stdout, "1 resources tainted", "Expected success message")

			// Step 3: Run preview to see the resource will be replaced
			stdout, stderr, err = stack.RunCommand("pulumi", "preview", "--json")
			require.NoError(t, err, "Failed to preview: %s\n%s", stdout, stderr)
			// The preview should show the resource will be replaced
			assert.Contains(t, stdout, "replace", "Preview should show resource will be replaced")

			// Step 4: Run update to replace the tainted resource
			stdout, stderr, err = stack.RunCommand("pulumi", "up", "--yes", "--skip-preview")
			require.NoError(t, err, "Failed to update: %s\n%s", stdout, stderr)
			assert.Contains(t, stdout, "replaced", "Update should show resource was replaced")

			// Step 5: Verify the resource is no longer tainted by running another preview
			stdout, stderr, err = stack.RunCommand("pulumi", "preview")
			require.NoError(t, err, "Failed to preview after update: %s\n%s", stdout, stderr)
			// Should show no changes
			assert.Contains(t, strings.ToLower(stdout), "no changes", "Should have no changes after replacing tainted resource")
		},
	}

	integration.ProgramTest(t, e)
}

// TestTaintMultipleResources tests tainting multiple resources at once
func TestTaintMultipleResources(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write a program with multiple resources
	program := `
const pulumi = require("@pulumi/pulumi");

class MyResource extends pulumi.CustomResource {
    constructor(name, args, opts) {
        super("test:index:MyResource", name, args, opts);
    }
}

const resource1 = new MyResource("resource1", { prop: "value1" });
const resource2 = new MyResource("resource2", { prop: "value2" });
const resource3 = new MyResource("resource3", { prop: "value3" });

exports.resource1Id = resource1.id;
exports.resource2Id = resource2.id;
exports.resource3Id = resource3.id;
`

	err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(program), 0600)
	require.NoError(t, err)

	// Write package.json
	packageJSON := `{
  "name": "taint-multiple-test",
  "main": "index.js",
  "dependencies": {
    "@pulumi/pulumi": "^3.0.0"
  }
}
`
	err = os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0600)
	require.NoError(t, err)

	// Write Pulumi.yaml
	pulumiYAML := `name: taint-multiple-test
runtime: nodejs
description: Test taint multiple resources
`
	err = os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(pulumiYAML), 0600)
	require.NoError(t, err)

	e := &integration.ProgramTestOptions{
		Dir:             dir,
		Dependencies:    []string{"@pulumi/pulumi"},
		Quick:           true,
		SkipRefresh:     true,
		NoParallel:      true,

		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			// Get URNs of resources
			var resource1URN, resource2URN string
			for _, res := range stack.Deployment.Resources {
				if res.Type == "test:index:MyResource" {
					if strings.Contains(string(res.URN), "resource1") {
						resource1URN = string(res.URN)
					} else if strings.Contains(string(res.URN), "resource2") {
						resource2URN = string(res.URN)
					}
				}
			}
			require.NotEmpty(t, resource1URN, "resource1 URN not found")
			require.NotEmpty(t, resource2URN, "resource2 URN not found")

			// Taint multiple resources
			stdout, stderr, err := stack.RunCommand("pulumi", "state", "taint", resource1URN, resource2URN, "--yes")
			require.NoError(t, err, "Failed to taint resources: %s\n%s", stdout, stderr)
			assert.Contains(t, stdout, "2 resources tainted", "Expected 2 resources tainted")

			// Run update to replace both tainted resources
			stdout, stderr, err = stack.RunCommand("pulumi", "up", "--yes", "--skip-preview")
			require.NoError(t, err, "Failed to update: %s\n%s", stdout, stderr)
			// Should show 2 resources replaced
			assert.Contains(t, stdout, "replaced", "Update should show resources were replaced")
		},
	}

	integration.ProgramTest(t, e)
}

// TestUntaintResource tests untainting a tainted resource
func TestUntaintResource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write a simple Pulumi program
	program := `
const pulumi = require("@pulumi/pulumi");

class MyResource extends pulumi.CustomResource {
    constructor(name, args, opts) {
        super("test:index:MyResource", name, args, opts);
    }
}

const resource1 = new MyResource("myresource", {
    prop1: "value1",
});

exports.resource1Id = resource1.id;
`

	err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(program), 0600)
	require.NoError(t, err)

	// Write package.json
	packageJSON := `{
  "name": "untaint-test",
  "main": "index.js",
  "dependencies": {
    "@pulumi/pulumi": "^3.0.0"
  }
}
`
	err = os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0600)
	require.NoError(t, err)

	// Write Pulumi.yaml
	pulumiYAML := `name: untaint-test
runtime: nodejs
description: Test untaint functionality
`
	err = os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(pulumiYAML), 0600)
	require.NoError(t, err)

	e := &integration.ProgramTestOptions{
		Dir:             dir,
		Dependencies:    []string{"@pulumi/pulumi"},
		Quick:           true,
		SkipRefresh:     true,
		NoParallel:      true,

		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			// Get the URN of the resource
			var resourceURN string
			for _, res := range stack.Deployment.Resources {
				if res.Type == "test:index:MyResource" {
					resourceURN = string(res.URN)
					break
				}
			}
			require.NotEmpty(t, resourceURN, "MyResource URN not found")

			// Taint the resource
			stdout, stderr, err := stack.RunCommand("pulumi", "state", "taint", resourceURN, "--yes")
			require.NoError(t, err, "Failed to taint resource: %s\n%s", stdout, stderr)

			// Preview should show it will be replaced
			stdout, stderr, err = stack.RunCommand("pulumi", "preview", "--json")
			require.NoError(t, err, "Failed to preview: %s\n%s", stdout, stderr)
			assert.Contains(t, stdout, "replace", "Preview should show resource will be replaced")

			// Untaint the resource
			stdout, stderr, err = stack.RunCommand("pulumi", "state", "untaint", resourceURN, "--yes")
			require.NoError(t, err, "Failed to untaint resource: %s\n%s", stdout, stderr)
			assert.Contains(t, stdout, "1 resources untainted", "Expected success message")

			// Preview should now show no changes
			stdout, stderr, err = stack.RunCommand("pulumi", "preview")
			require.NoError(t, err, "Failed to preview after untaint: %s\n%s", stdout, stderr)
			assert.Contains(t, strings.ToLower(stdout), "no changes", "Should have no changes after untainting resource")
		},
	}

	integration.ProgramTest(t, e)
}

// TestUntaintAll tests the --all flag for untaint
func TestUntaintAll(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write a program with multiple resources
	program := `
const pulumi = require("@pulumi/pulumi");

class MyResource extends pulumi.CustomResource {
    constructor(name, args, opts) {
        super("test:index:MyResource", name, args, opts);
    }
}

const resource1 = new MyResource("resource1", { prop: "value1" });
const resource2 = new MyResource("resource2", { prop: "value2" });
const resource3 = new MyResource("resource3", { prop: "value3" });

exports.resource1Id = resource1.id;
exports.resource2Id = resource2.id;
exports.resource3Id = resource3.id;
`

	err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(program), 0600)
	require.NoError(t, err)

	// Write package.json
	packageJSON := `{
  "name": "untaint-all-test",
  "main": "index.js",
  "dependencies": {
    "@pulumi/pulumi": "^3.0.0"
  }
}
`
	err = os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0600)
	require.NoError(t, err)

	// Write Pulumi.yaml
	pulumiYAML := `name: untaint-all-test
runtime: nodejs
description: Test untaint all functionality
`
	err = os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(pulumiYAML), 0600)
	require.NoError(t, err)

	e := &integration.ProgramTestOptions{
		Dir:             dir,
		Dependencies:    []string{"@pulumi/pulumi"},
		Quick:           true,
		SkipRefresh:     true,
		NoParallel:      true,

		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			// Get URNs of all resources
			var resourceURNs []string
			for _, res := range stack.Deployment.Resources {
				if res.Type == "test:index:MyResource" {
					resourceURNs = append(resourceURNs, string(res.URN))
				}
			}
			require.Len(t, resourceURNs, 3, "Should have 3 resources")

			// Taint all resources
			args := append([]string{"state", "taint", "--yes"}, resourceURNs...)
			stdout, stderr, err := stack.RunCommand("pulumi", args...)
			require.NoError(t, err, "Failed to taint resources: %s\n%s", stdout, stderr)
			assert.Contains(t, stdout, "3 resources tainted", "Expected 3 resources tainted")

			// Untaint all resources
			stdout, stderr, err = stack.RunCommand("pulumi", "state", "untaint", "--all", "--yes")
			require.NoError(t, err, "Failed to untaint all: %s\n%s", stdout, stderr)
			assert.Contains(t, stdout, "All resources untainted", "Expected all resources untainted")

			// Preview should show no changes
			stdout, stderr, err = stack.RunCommand("pulumi", "preview")
			require.NoError(t, err, "Failed to preview: %s\n%s", stdout, stderr)
			assert.Contains(t, strings.ToLower(stdout), "no changes", "Should have no changes after untainting all")
		},
	}

	integration.ProgramTest(t, e)
}