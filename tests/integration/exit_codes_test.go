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

//go:build !windows

package ints

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// TestExitCodeStackNotFound verifies that selecting a non-existent stack
// returns exit code 6 (ExitStackNotFound) instead of a generic error code.
func TestExitCodeStackNotFound(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	// Try to select a non-existent stack - this should fail with exit code 6
	_, _, err := e.GetCommandResults("pulumi", "stack", "select", "nonexistent-stack-xyz-12345")

	require.Error(t, err, "expected error when selecting non-existent stack")
	if exiterr, ok := err.(*exec.ExitError); ok {
		require.Equal(t, cmdutil.ExitStackNotFound, exiterr.ExitCode(),
			"expected exit code %d (ExitStackNotFound), got %d",
			cmdutil.ExitStackNotFound, exiterr.ExitCode())
	} else {
		require.Fail(t, "expected *exec.ExitError, got %T", err)
	}
}

// TestExitCodeStackSelectSuccess verifies that selecting an existing stack
// returns exit code 0 (success).
func TestExitCodeStackSelectSuccess(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "test-stack-for-select")

	// Selecting an existing stack should succeed with exit code 0
	// RunCommand fails the test if exit != 0, so reaching here means success
	e.RunCommand("pulumi", "stack", "select", "test-stack-for-select")
}

// TestExitCodeVersionSuccess verifies that pulumi version returns exit code 0.
func TestExitCodeVersionSuccess(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Version should always succeed
	e.RunCommand("pulumi", "version")
}
