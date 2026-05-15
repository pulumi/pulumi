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
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedListCall records the inputs to a single ListStackDeployments call.
type capturedListCall struct {
	stack client.StackIdentifier
	opts  client.ListStackDeploymentsOptions
}

// mockDeploymentListClient stubs deploymentListClient. It returns a fixed
// response (or error) and records the most recent invocation so tests can
// assert on the flag-to-query propagation.
type mockDeploymentListClient struct {
	resp     apitype.ListDeploymentResponseV2
	err      error
	captured *capturedListCall
}

func (m *mockDeploymentListClient) ListStackDeployments(
	_ context.Context, stack client.StackIdentifier, opts client.ListStackDeploymentsOptions,
) (apitype.ListDeploymentResponseV2, error) {
	if m.captured != nil {
		*m.captured = capturedListCall{stack: stack, opts: opts}
	}
	if m.err != nil {
		return apitype.ListDeploymentResponseV2{}, m.err
	}
	return m.resp, nil
}

var testStackID = client.StackIdentifier{
	Owner:   "acme",
	Project: "web",
	Stack:   tokens.MustParseStackName("prod"),
}

func stubListFactory(c deploymentListClient) deploymentListClientFactory {
	return func(_ context.Context, _ string) (deploymentListClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func failingListFactory(err error) deploymentListClientFactory {
	return func(_ context.Context, _ string) (deploymentListClient, client.StackIdentifier, error) {
		return nil, client.StackIdentifier{}, err
	}
}

func sampleListResponse() apitype.ListDeploymentResponseV2 {
	return apitype.ListDeploymentResponseV2{
		ItemsPerPage: 10,
		Total:        2,
		Deployments: []apitype.ListDeploymentSnapshot{
			{
				ID:              "dep-1",
				Created:         "2026-05-01T12:00:00Z",
				Modified:        "2026-05-01T12:05:00Z",
				Status:          "succeeded",
				Version:         42,
				RequestedBy:     apitype.UserInfo{Name: "Alice", GitHubLogin: "alice", AvatarURL: "https://x/a.png"},
				PulumiOperation: apitype.Update,
				Initiator:       "cli",
				Updates:         []apitype.DeploymentNestedUpdate{},
				Jobs: []apitype.DeploymentJob{{
					Status:      "succeeded",
					Started:     time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
					LastUpdated: time.Date(2026, 5, 1, 12, 5, 0, 0, time.UTC),
					Steps: []apitype.DeploymentStepRun{
						{Name: "pulumi-up", Status: "succeeded"},
					},
				}},
			},
			{
				ID:              "dep-2",
				Created:         "2026-04-30T08:00:00Z",
				Modified:        "2026-04-30T08:02:00Z",
				Status:          "failed",
				Version:         41,
				RequestedBy:     apitype.UserInfo{Name: "Bob", GitHubLogin: "bob"},
				PulumiOperation: apitype.Preview,
				Updates:         []apitype.DeploymentNestedUpdate{},
				Jobs:            []apitype.DeploymentJob{},
			},
		},
	}
}

func TestDeploymentList_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentListClient{resp: sampleListResponse()}
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c), deploymentListArgs{page: 1})
	require.NoError(t, err)

	out := buf.String()
	// Table headers.
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "OPERATION")
	assert.Contains(t, out, "VERSION")
	assert.Contains(t, out, "STATUS")
	assert.Contains(t, out, "INITIATED BY")
	assert.Contains(t, out, "MODIFIED")

	// Rows.
	assert.Contains(t, out, "dep-1")
	assert.Contains(t, out, "update")
	assert.Contains(t, out, "succeeded")
	assert.Contains(t, out, "alice")
	assert.Contains(t, out, "2026-05-01T12:05:00Z")

	assert.Contains(t, out, "dep-2")
	assert.Contains(t, out, "preview")
	assert.Contains(t, out, "failed")
	assert.Contains(t, out, "bob")

	// Footer pagination summary.
	assert.Contains(t, out, "Showing 2 of 2 deployment(s) (page 1)")
}

func TestDeploymentList_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentListClient{resp: apitype.ListDeploymentResponseV2{}}
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c), deploymentListArgs{page: 1})
	require.NoError(t, err)
	assert.Equal(t, "No deployments found for this stack.\n", buf.String())
}

func TestDeploymentList_DefaultOutput_FallsBackToNameWhenGithubLoginMissing(t *testing.T) {
	t.Parallel()

	resp := apitype.ListDeploymentResponseV2{
		ItemsPerPage: 10,
		Total:        1,
		Deployments: []apitype.ListDeploymentSnapshot{{
			ID:              "dep-x",
			Status:          "succeeded",
			Version:         1,
			RequestedBy:     apitype.UserInfo{Name: "Display Name", GitHubLogin: ""},
			PulumiOperation: apitype.Update,
			Updates:         []apitype.DeploymentNestedUpdate{},
			Jobs:            []apitype.DeploymentJob{},
			Modified:        "2026-05-01T12:00:00Z",
		}},
	}

	var buf bytes.Buffer
	c := &mockDeploymentListClient{resp: resp}
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c), deploymentListArgs{page: 1})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Display Name")
}

func TestDeploymentList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentListClient{resp: sampleListResponse()}
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c),
		deploymentListArgs{output: "json", page: 1})
	require.NoError(t, err)

	// Decode and check structural fields rather than comparing the whole
	// document string-for-string: the encoder may reorder nothing but the
	// indentation/whitespace assertions are brittle.
	var env deploymentListEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))

	assert.Equal(t, int64(1), env.Page)
	assert.Equal(t, int64(10), env.ItemsPerPage)
	assert.Equal(t, int64(2), env.Total)
	require.Len(t, env.Deployments, 2)
	assert.Equal(t, "dep-1", env.Deployments[0].ID)
	assert.Equal(t, apitype.Update, env.Deployments[0].PulumiOperation)
	assert.Equal(t, "alice", env.Deployments[0].RequestedBy.GitHubLogin)
}

func TestDeploymentList_JSONOutput_EmptyArray(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentListClient{resp: apitype.ListDeploymentResponseV2{}}
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c),
		deploymentListArgs{output: "json", page: 1})
	require.NoError(t, err)

	assert.JSONEq(t,
		`{"deployments":[],"page":1,"itemsPerPage":0,"total":0}`, buf.String())
}

func TestDeploymentList_PropagatesFlagsToClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args deploymentListArgs
		want client.ListStackDeploymentsOptions
	}{
		{
			name: "defaults flow through",
			args: deploymentListArgs{page: 1, pageSize: 10},
			want: client.ListStackDeploymentsOptions{Page: 1, PageSize: 10},
		},
		{
			name: "all options",
			args: deploymentListArgs{page: 3, pageSize: 25, sort: "modified", asc: true},
			want: client.ListStackDeploymentsOptions{Page: 3, PageSize: 25, Sort: "modified", Asc: true},
		},
		{
			name: "zero values flow through",
			args: deploymentListArgs{},
			want: client.ListStackDeploymentsOptions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured capturedListCall
			c := &mockDeploymentListClient{resp: apitype.ListDeploymentResponseV2{}, captured: &captured}
			var buf bytes.Buffer
			err := runDeploymentList(t.Context(), &buf, stubListFactory(c), tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, captured.opts)
			assert.Equal(t, testStackID, captured.stack)
		})
	}
}

func TestDeploymentList_StackFlagPropagatesToFactory(t *testing.T) {
	t.Parallel()

	var capturedStack string
	factory := func(_ context.Context, stackFlag string) (deploymentListClient, client.StackIdentifier, error) {
		capturedStack = stackFlag
		return &mockDeploymentListClient{resp: apitype.ListDeploymentResponseV2{}}, testStackID, nil
	}

	var buf bytes.Buffer
	err := runDeploymentList(t.Context(), &buf, factory,
		deploymentListArgs{stack: "acme/web/staging", page: 1})
	require.NoError(t, err)
	assert.Equal(t, "acme/web/staging", capturedStack)
}

func TestDeploymentList_InvalidOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentListClient{resp: sampleListResponse()}
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c),
		deploymentListArgs{output: "yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --output value "yaml"`)
}

func TestDeploymentList_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentListClient{err: errors.New("server boom")}
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c), deploymentListArgs{page: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing stack deployments")
	assert.Contains(t, err.Error(), "server boom")
}

func TestDeploymentList_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runDeploymentList(t.Context(), &buf, failingListFactory(errors.New("not logged in")),
		deploymentListArgs{page: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}
