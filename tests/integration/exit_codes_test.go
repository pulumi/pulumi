// Copyright 2026, Pulumi Corporation.
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
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

func TestExitCode_StackNotFound(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Create a minimal project so the DIY file backend can infer the
	// current project name when parsing stack references.
	e.WriteTestFile("Pulumi.yaml", `
name: exit-code-stack-not-found
runtime: nodejs
`)

	// Use a local file backend so we don't depend on external services.
	e.Backend = e.LocalURL()

	cmd := e.SetupCommandIn(t.Context(), e.CWD, "pulumi", "stack", "select", "does-not-exist")
	err := cmd.Run()

	require.Error(t, err)
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok)
	require.Equal(t, 6, exitErr.ExitCode())
}
