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

package stack

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStackGetClient implements stackGetClient for tests.
type mockStackGetClient struct {
	stack apitype.Stack
	err   error
}

func (m *mockStackGetClient) GetStack(_ context.Context, _ client.StackIdentifier) (apitype.Stack, error) {
	return m.stack, m.err
}

func stackGetStubFactory(c stackGetClient) stackGetClientFactory {
	return func(_ context.Context, _ string) (stackGetClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func stackGetFailingFactory(err error) stackGetClientFactory {
	return func(_ context.Context, _ string) (stackGetClient, client.StackIdentifier, error) {
		return nil, client.StackIdentifier{}, err
	}
}

func sampleStack() apitype.Stack {
	return apitype.Stack{
		ID:           "abc123",
		OrgName:      "my-org",
		ProjectName:  "my-project",
		StackName:    "dev",
		ActiveUpdate: "11111111-2222-3333-4444-555555555555",
		Version:      42,
		Tags: map[apitype.StackTagName]string{
			"environment":    "production",
			"pulumi:project": "my-project",
			"pulumi:runtime": "nodejs",
			"vcs:owner":      "pulumi",
		},
		CurrentOperation: &apitype.OperationStatus{
			Kind:    apitype.UpdateUpdate,
			Author:  "alice",
			Started: 1747142400, // 2025-05-13T13:20:00Z
		},
	}
}

func TestStackGet_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockStackGetClient{stack: sampleStack()}
	err := runStackGet(t.Context(), &buf, stackGetStubFactory(c), "", "default")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Organization:   my-org")
	assert.Contains(t, out, "Project:        my-project")
	assert.Contains(t, out, "Stack:          dev")
	assert.Contains(t, out, "Version:        42")
	assert.Contains(t, out, "Active update:  11111111-2222-3333-4444-555555555555")
	assert.Contains(t, out, "Update in progress:")
	assert.Contains(t, out, "Kind:    update")
	assert.Contains(t, out, "Author:  alice")
	assert.Contains(t, out, "Tags:")
	assert.Contains(t, out, "environment")
	assert.Contains(t, out, "production")
	assert.Contains(t, out, "pulumi:project")
	assert.Contains(t, out, "pulumi:runtime")
	assert.Contains(t, out, "nodejs")
}

func TestStackGet_DefaultOutput_NoOperation(t *testing.T) {
	t.Parallel()

	stack := sampleStack()
	stack.CurrentOperation = nil

	var buf bytes.Buffer
	c := &mockStackGetClient{stack: stack}
	err := runStackGet(t.Context(), &buf, stackGetStubFactory(c), "", "default")
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, "Update in progress")
	assert.NotContains(t, out, "Kind:")
	assert.NotContains(t, out, "Author:")
}

func TestStackGet_DefaultOutput_NoActiveUpdate(t *testing.T) {
	t.Parallel()

	stack := sampleStack()
	stack.ActiveUpdate = ""

	var buf bytes.Buffer
	c := &mockStackGetClient{stack: stack}
	err := runStackGet(t.Context(), &buf, stackGetStubFactory(c), "", "default")
	require.NoError(t, err)

	assert.NotContains(t, buf.String(), "Active update:")
}

func TestStackGet_DefaultOutput_NoTags(t *testing.T) {
	t.Parallel()

	stack := sampleStack()
	stack.Tags = nil

	var buf bytes.Buffer
	c := &mockStackGetClient{stack: stack}
	err := runStackGet(t.Context(), &buf, stackGetStubFactory(c), "", "default")
	require.NoError(t, err)

	assert.NotContains(t, buf.String(), "Tags:")
}

func TestStackGet_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockStackGetClient{stack: sampleStack()}
	err := runStackGet(t.Context(), &buf, stackGetStubFactory(c), "", "json")
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"organization": "my-org",
		"project": "my-project",
		"stack": "dev",
		"version": 42,
		"activeUpdate": "11111111-2222-3333-4444-555555555555",
		"currentOperation": {
			"kind": "update",
			"author": "alice",
			"started": "2025-05-13T13:20:00Z"
		},
		"tags": {
			"environment":    "production",
			"pulumi:project": "my-project",
			"pulumi:runtime": "nodejs",
			"vcs:owner":      "pulumi"
		}
	}`, buf.String())
}

func TestStackGet_JSONOutput_NoOperation(t *testing.T) {
	t.Parallel()

	stack := sampleStack()
	stack.CurrentOperation = nil
	stack.Tags = nil

	var buf bytes.Buffer
	c := &mockStackGetClient{stack: stack}
	err := runStackGet(t.Context(), &buf, stackGetStubFactory(c), "", "json")
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"organization": "my-org",
		"project": "my-project",
		"stack": "dev",
		"version": 42,
		"activeUpdate": "11111111-2222-3333-4444-555555555555",
		"tags": {}
	}`, buf.String())
}

func TestStackGet_InvalidOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockStackGetClient{stack: sampleStack()}
	err := runStackGet(t.Context(), &buf, stackGetStubFactory(c), "", "xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --output value")
	assert.Contains(t, err.Error(), "xml")
}

func TestStackGet_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockStackGetClient{err: errors.New("server error")}
	err := runStackGet(t.Context(), &buf, stackGetStubFactory(c), "", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting stack")
	assert.Contains(t, err.Error(), "server error")
}

func TestStackGet_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runStackGet(t.Context(), &buf, stackGetFailingFactory(errors.New("not logged in")), "", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestStackGet_StackFlagPropagation(t *testing.T) {
	t.Parallel()

	var capturedStack string
	factory := func(_ context.Context, stackFlag string) (stackGetClient, client.StackIdentifier, error) {
		capturedStack = stackFlag
		return &mockStackGetClient{stack: sampleStack()}, testStackID, nil
	}

	var buf bytes.Buffer
	err := runStackGet(t.Context(), &buf, factory, "org/proj/my-stack", "default")
	require.NoError(t, err)
	assert.Equal(t, "org/proj/my-stack", capturedStack)
}

func TestStackGet_CobraFlagBinding(t *testing.T) {
	t.Parallel()

	c := &mockStackGetClient{stack: sampleStack()}
	cmd := newStackGetCmdWith(stackGetStubFactory(c))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--output", "json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"organization": "my-org"`)
}

func TestStackGet_DefaultCmd(t *testing.T) {
	t.Parallel()

	cmd := newStackGetCmd()
	assert.Equal(t, "get", cmd.Use)
	require.NotNil(t, cmd.RunE)

	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f)
	assert.Equal(t, "o", f.Shorthand)
	assert.Equal(t, "default", f.DefValue)

	sf := cmd.Flags().Lookup("stack")
	require.NotNil(t, sf)
	assert.Equal(t, "s", sf.Shorthand)
}
