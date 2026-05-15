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

package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedCancelCall records the inputs to a single CancelStackDeployment call
// so tests can assert that the deployment ID and StackIdentifier reach the
// client as expected.
type capturedCancelCall struct {
	stack        client.StackIdentifier
	deploymentID string
}

// mockDeploymentCancelClient stubs deploymentCancelClient. It returns a fixed
// error (or nil) and records the most recent invocation.
type mockDeploymentCancelClient struct {
	err      error
	captured *capturedCancelCall
}

func (m *mockDeploymentCancelClient) CancelStackDeployment(
	_ context.Context, stack client.StackIdentifier, deploymentID string,
) error {
	if m.captured != nil {
		*m.captured = capturedCancelCall{stack: stack, deploymentID: deploymentID}
	}
	return m.err
}

func stubCancelFactory(c deploymentCancelClient, stackName string) deploymentCancelClientFactory {
	return func(_ context.Context, _ string) (deploymentCancelClient, client.StackIdentifier, string, error) {
		return c, testStackID, stackName, nil
	}
}

func failingCancelFactory(err error) deploymentCancelClientFactory {
	return func(_ context.Context, _ string) (deploymentCancelClient, client.StackIdentifier, string, error) {
		return nil, client.StackIdentifier{}, "", err
	}
}

// alwaysConfirm/alwaysDecline make the confirmation deterministic in tests.
// The default --yes flag bypasses the prompt entirely; these are used to cover
// the interactive paths.
func alwaysConfirm(string) bool { return true }

func alwaysDecline(string) bool { return false }

// cancelArgs builds a deploymentCancelArgs with the production OutputFlag
// pre-wired so tests don't have to repeat the boilerplate. Pass "" for
// outputFormat to keep the default (terminal) renderer.
func cancelArgs(stack string, yes bool, outputFormat string) deploymentCancelArgs {
	args := deploymentCancelArgs{
		stack:  stack,
		yes:    yes,
		output: defaultDeploymentCancelOutputFormat(),
	}
	if outputFormat != "" {
		// Errors from Set bubble up via cobra in production; the tests that
		// exercise an unsupported value drive cobra directly instead.
		_ = args.output.Set(outputFormat)
	}
	return args
}

func TestDeploymentCancel_DefaultOutput(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentCancelClient{}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"), alwaysConfirm,
		"dep-123", cancelArgs("", true, ""))
	require.NoError(t, err)
	assert.Equal(t, "Cancellation requested for deployment 'dep-123'.\n", buf.String())
}

func TestDeploymentCancel_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentCancelClient{}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"), alwaysConfirm,
		"dep-123", cancelArgs("", true, "json"))
	require.NoError(t, err)

	var env deploymentCancelEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, deploymentCancelEnvelope{
		DeploymentID: "dep-123",
		Stack:        "prod",
		Canceled:     true,
	}, env)
}

func TestDeploymentCancel_PropagatesIDAndStack(t *testing.T) {
	t.Parallel()

	var captured capturedCancelCall
	c := &mockDeploymentCancelClient{captured: &captured}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"), alwaysConfirm,
		"dep-xyz", cancelArgs("", true, ""))
	require.NoError(t, err)
	assert.Equal(t, capturedCancelCall{stack: testStackID, deploymentID: "dep-xyz"}, captured)
}

func TestDeploymentCancel_StackFlagPropagatesToFactory(t *testing.T) {
	t.Parallel()

	var capturedStack string
	factory := func(_ context.Context, stackFlag string) (deploymentCancelClient, client.StackIdentifier, string, error) {
		capturedStack = stackFlag
		return &mockDeploymentCancelClient{}, testStackID, "staging", nil
	}

	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, factory, alwaysConfirm,
		"dep-123", cancelArgs("acme/web/staging", true, ""))
	require.NoError(t, err)
	assert.Equal(t, "acme/web/staging", capturedStack)
}

func TestNewDeploymentCancelCmd_InvalidOutputRejected(t *testing.T) {
	t.Parallel()

	// Unsupported --output values are caught by the OutputFlag at flag-parse
	// time, so the API is never reached.
	cmd := newDeploymentCancelCmdWith(stubCancelFactory(&mockDeploymentCancelClient{}, "prod"), alwaysConfirm)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--output", "yaml", "--yes", "dep-123"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}

func TestDeploymentCancel_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentCancelClient{err: errors.New("server boom")}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"), alwaysConfirm,
		"dep-123", cancelArgs("", true, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canceling deployment")
	assert.Contains(t, err.Error(), "server boom")
}

func TestDeploymentCancel_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf,
		failingCancelFactory(errors.New("not logged in")), alwaysConfirm,
		"dep-123", cancelArgs("", true, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestDeploymentCancel_ConfirmDeclined(t *testing.T) {
	t.Parallel()

	// Without --yes the prompt fires; if the user declines, we must bail out
	// before touching the client.
	var captured capturedCancelCall
	c := &mockDeploymentCancelClient{captured: &captured}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"), alwaysDecline,
		"dep-123", cancelArgs("", false, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confirmation declined")
	assert.Equal(t, capturedCancelCall{}, captured, "client must not be called when confirmation declines")
}

func TestDeploymentCancel_ConfirmAccepted(t *testing.T) {
	t.Parallel()

	// Without --yes, an accepting confirmer must let the cancel through.
	var captured capturedCancelCall
	c := &mockDeploymentCancelClient{captured: &captured}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"), alwaysConfirm,
		"dep-123", cancelArgs("", false, ""))
	require.NoError(t, err)
	assert.Equal(t, capturedCancelCall{stack: testStackID, deploymentID: "dep-123"}, captured)
}

func TestDeploymentCancel_ConfirmPromptReceivesIDAndStack(t *testing.T) {
	t.Parallel()

	// The y/n prompt surfaces the deployment ID and stack to the user —
	// assert they propagate verbatim.
	var gotPrompt string
	confirm := func(prompt string) bool {
		gotPrompt = prompt
		return true
	}

	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf,
		stubCancelFactory(&mockDeploymentCancelClient{}, "prod"), confirm,
		"dep-abc123", cancelArgs("", false, ""))
	require.NoError(t, err)
	assert.Contains(t, gotPrompt, "dep-abc123")
	assert.Contains(t, gotPrompt, "prod")
}

func TestDeploymentCancel_NilConfirm_NonInteractiveAccepts(t *testing.T) {
	t.Parallel()

	// Passing nil for confirm installs the production prompt. Tests don't
	// have a TTY, so cmdutil.Interactive() returns false and the inline
	// non-interactive branch accepts without prompting — verify that the
	// cancel still reaches the client.
	var captured capturedCancelCall
	c := &mockDeploymentCancelClient{captured: &captured}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"), nil,
		"dep-123", cancelArgs("", false, ""))
	require.NoError(t, err)
	assert.Equal(t, capturedCancelCall{stack: testStackID, deploymentID: "dep-123"}, captured)
}

func TestDeploymentCancel_NilFactory_UsesDefaultAndPropagatesError(t *testing.T) {
	t.Parallel()

	// Passing nil for the factory installs defaultDeploymentCancelClientFactory,
	// which drives cmdStack.RequireStack — that fails when run outside a
	// Pulumi project. The deeper branches of the factory (cloud-backend type
	// assertion, success path) are left to integration tests; this guards the
	// nil-default wire-up plus the resolving-stack early-error path.
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, nil, alwaysConfirm,
		"dep-123", cancelArgs("", true, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving stack")
}

func TestDeploymentCancel_YesSkipsPrompt(t *testing.T) {
	t.Parallel()

	// With --yes the confirm callback must not be invoked; if it is, the
	// test fails because alwaysDecline would otherwise abort the run.
	var captured capturedCancelCall
	c := &mockDeploymentCancelClient{captured: &captured}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"), alwaysDecline,
		"dep-123", cancelArgs("", true, ""))
	require.NoError(t, err)
	assert.Equal(t, capturedCancelCall{stack: testStackID, deploymentID: "dep-123"}, captured)
}

func TestNewDeploymentCancelCmd_CobraFlagBinding(t *testing.T) {
	t.Parallel()

	var captured capturedCancelCall
	c := &mockDeploymentCancelClient{captured: &captured}
	cmd := newDeploymentCancelCmdWith(stubCancelFactory(c, "prod"), alwaysConfirm)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--stack", "acme/web/prod",
		"--yes",
		"--output", "json",
		"dep-123",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Equal(t, capturedCancelCall{stack: testStackID, deploymentID: "dep-123"}, captured)

	var env deploymentCancelEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, "dep-123", env.DeploymentID)
	assert.Equal(t, "prod", env.Stack)
	assert.True(t, env.Canceled)
}

func TestNewDeploymentCancelCmd_RequiresDeploymentID(t *testing.T) {
	t.Parallel()

	cmd := newDeploymentCancelCmdWith(stubCancelFactory(&mockDeploymentCancelClient{}, "prod"), alwaysConfirm)
	cmd.SetArgs([]string{})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
}

func TestNewDeploymentCancelCmd_Defaults(t *testing.T) {
	t.Parallel()

	// Passing nil installs the production factory and confirmer; assert the
	// command is well-formed without invoking it (the prod factory would
	// touch the real filesystem / cloud config).
	cmd := newDeploymentCancelCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "cancel <deployment-id>", cmd.Use)

	output := cmd.Flags().Lookup("output")
	require.NotNil(t, output)
	assert.Equal(t, "o", output.Shorthand)
	assert.Equal(t, "default", output.DefValue)

	stack := cmd.Flags().Lookup("stack")
	require.NotNil(t, stack)
	assert.Equal(t, "s", stack.Shorthand)

	yes := cmd.Flags().Lookup("yes")
	require.NotNil(t, yes)
	assert.Equal(t, "y", yes.Shorthand)
	assert.Equal(t, "false", yes.DefValue)
}
