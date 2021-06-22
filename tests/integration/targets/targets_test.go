// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

func TestUntargetedCreateDuringTargetedUpdate(t *testing.T) {
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
