// Copyright 2018, Pulumi Corporation.
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
package tests

import (
	"testing"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"
)

// assertHasNoHistory runs `pulumi history` and confirms an error that the stack has not
// ever been updated.
func assertHasNoHistory(e *ptesting.Environment) {
	// NOTE: pulumi returns with exit code 0 in this scenario.
	out, err := e.RunCommand("pulumi", "history")
	assert.Equal(e.T, "", err)
	assert.Equal(e.T, "Stack has never been updated\n", out)
}
func TestHistoryCommand(t *testing.T) {
	// We fail if no stack is selected.
	t.Run("NoStackSelected", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)
		integration.CreateBasicPulumiRepo(e)
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		out, err := e.RunCommandExpectError("pulumi", "history")
		assert.Equal(t, "", out)
		assert.Contains(t, err, "error: no stack selected")
	})

	// We don't display any history for a stack that has never been updated.
	t.Run("NoUpdates", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)
		integration.CreateBasicPulumiRepo(e)
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "no-updates-test")
		assertHasNoHistory(e)
	})

	// The "history" command uses the currently selected stack.
	t.Run("CurrentlySelectedStack", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)
		integration.CreateBasicPulumiRepo(e)
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.ImportDirectory("integration/stack_outputs")
		e.RunCommand("pulumi", "stack", "init", "stack-without-updates")
		e.RunCommand("pulumi", "stack", "init", "history-test")
		e.RunCommand("yarn", "install")
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		// Update the history-test stack.
		e.RunCommand("pulumi", "update", "--non-interactive", "--skip-preview", "-m", "this is an updated stack")
		// Confirm we see the update message in thie history output.
		out, err := e.RunCommand("pulumi", "history")
		assert.Equal(t, "", err)
		assert.Contains(t, out, "this is an updated stack")
		// Change stack and confirm the history command honors the selected stack.
		e.RunCommand("pulumi", "stack", "select", "stack-without-updates")
		assertHasNoHistory(e)
		// Change stack back, and confirm still has history.
		e.RunCommand("pulumi", "stack", "select", "history-test")
		out, err = e.RunCommand("pulumi", "history")
		assert.Equal(t, "", err)
		assert.Contains(t, out, "this is an updated stack")
	})
}
