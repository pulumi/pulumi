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

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
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
		_ = args.output.Set(outputFormat)
	}
	return args
}

func TestDeploymentCancel_DefaultOutput(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentCancelClient{}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"),
		"dep-123", cancelArgs("", true, ""))
	require.NoError(t, err)
	assert.Equal(t, "Cancellation requested for deployment 'dep-123'.\n", buf.String())
}

func TestDeploymentCancel_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentCancelClient{}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"),
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
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"),
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
	err := runDeploymentCancel(t.Context(), &buf, factory,
		"dep-123", cancelArgs("acme/web/staging", true, ""))
	require.NoError(t, err)
	assert.Equal(t, "acme/web/staging", capturedStack)
}

func TestNewDeploymentCancelCmd_InvalidOutputRejected(t *testing.T) {
	t.Parallel()

	// Unsupported --output values are caught by the OutputFlag at flag-parse
	// time, so the API is never reached.
	cmd := newDeploymentCancelCmdWith(stubCancelFactory(&mockDeploymentCancelClient{}, "prod"))
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
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"),
		"dep-123", cancelArgs("", true, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canceling deployment")
	assert.Contains(t, err.Error(), "server boom")
}

func TestDeploymentCancel_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf,
		failingCancelFactory(errors.New("not logged in")),
		"dep-123", cancelArgs("", true, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestDeploymentCancel_NonInteractiveRequiresYes(t *testing.T) {
	t.Parallel()

	var captured capturedCancelCall
	c := &mockDeploymentCancelClient{captured: &captured}
	var buf bytes.Buffer
	err := runDeploymentCancel(t.Context(), &buf, stubCancelFactory(c, "prod"),
		"dep-123", cancelArgs("", false, ""))
	require.Error(t, err)
	assert.ErrorIs(t, err, backenderr.NonInteractiveRequiresYesError{})
	assert.Equal(t, capturedCancelCall{}, captured, "client must not be called when confirmation is refused")
}
