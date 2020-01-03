// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

package ints

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
)

// TestPolicy tests policy related commands work.
func TestPolicy(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	// Confirm we have credentials.
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Fatal("PULUMI_ACCESS_TOKEN not found, aborting tests.")
	}

	name, _ := e.RunCommand("pulumi", "whoami")
	orgName := strings.TrimSpace(name)

	// Pack and push a Policy Pack for the organization.
	policyPackName := fmt.Sprintf("%s-%x", "test-policy-pack", time.Now().UnixNano())
	e.ImportDirectory("test_policy_pack")
	e.RunCommand("yarn", "install")
	os.Setenv("TEST_POLICY_PACK", policyPackName)
	e.RunCommand("pulumi", "policy", "publish", orgName)

	// Enable, Disable and then Delete the Policy Pack.
	e.RunCommand("pulumi", "policy", "enable", fmt.Sprintf("%s/%s", orgName, policyPackName), "1")
	e.RunCommand("pulumi", "policy", "disable", fmt.Sprintf("%s/%s", orgName, policyPackName), "1")
	e.RunCommand("pulumi", "policy", "rm", fmt.Sprintf("%s/%s", orgName, policyPackName), "1")
}
