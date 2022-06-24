// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/stretchr/testify/assert"
)

func TestUntargetedCreateDuringTargetedUpdate(t *testing.T) {
	t.Skip() // TODO[pulumi/pulumi#4149]
	t.Parallel()

	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	stackName, err := resource.NewUniqueHex("test-", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	e.ImportDirectory("untargeted_create")
	e.RunCommand("pulumi", "stack", "init", stackName)
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview", "--yes")
	urn, _ := e.RunCommand("pulumi", "stack", "output", "urn")

	if err := fsutil.CopyFile(
		path.Join(e.RootPath, "untargeted_create", "index.ts"),
		path.Join("untargeted_create", "step1", "index.ts"), nil); err != nil {

		t.Fatalf("error copying index.ts file: %v", err)
	}

	e.RunCommand("pulumi", "up", "--target", strings.TrimSpace(urn), "--non-interactive", "--skip-preview", "--yes")
	e.RunCommand("pulumi", "refresh", "--non-interactive", "--yes")

	e.RunCommand("pulumi", "destroy", "--skip-preview", "--non-interactive", "--yes")
	e.RunCommand("pulumi", "stack", "rm", "--yes")
}

func TestDeleteManyTargets(t *testing.T) {
	t.Parallel()

	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	// First just spin up the project.
	projName := "delete_targets_many_deps"
	stackName, err := resource.NewUniqueHex("test-", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")
	e.ImportDirectory(projName)
	e.RunCommand("pulumi", "stack", "init", stackName)
	e.RunCommand("yarn", "install")
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview", "--yes")

	// Create a handy mkURN func to create URNs for dynamic resources in this project/stack.
	resourceType := tokens.Type("pulumi-nodejs:dynamic:Resource")
	mkURNStr := func(resourceName tokens.QName, parentType tokens.Type) string {
		return string(resource.NewURN(
			tokens.QName(stackName), tokens.PackageName(projName), parentType, resourceType, resourceName))
	}

	// Attempt to destroy the root-most node. It should fail and the error text should
	// mention every one of the nodes in the entire graph (since they all transitively depend on a).
	stdout, _ := e.RunCommandExpectError("pulumi", "destroy", "--skip-preview", "--yes", "--non-interactive",
		"--target", mkURNStr("a", ""))
	assert.Contains(t, stdout, mkURNStr("b", ""))
	assert.Contains(t, stdout, mkURNStr("c", ""))
	assert.Contains(t, stdout, mkURNStr("d", ""))
	assert.Contains(t, stdout, mkURNStr("e", ""))
	assert.Contains(t, stdout, mkURNStr("f", resourceType))
	assert.Contains(t, stdout, mkURNStr("g", resourceType))

	// Destroy the leaf-most node. This should work just fine.
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes", "--non-interactive",
		"--target", mkURNStr("h", tokens.Type(fmt.Sprintf("%[1]s$%[1]s", resourceType))))

	// Finally, go back and try to delete the root-most node, but clean up the transitive closure.
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes", "--non-interactive",
		"--target", mkURNStr("a", ""), "--target-dependents")

	// Finally clean up the entire stack.
	e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes", "--non-interactive")
	e.RunCommand("pulumi", "stack", "rm", "--yes")
}
