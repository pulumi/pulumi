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
	"fmt"
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

// mockDeploymentListClient stubs deploymentListClient. It returns either a
// fixed response on every call (resp) or page-keyed responses (pages, indexed
// by 1-based page number); records every call so tests can assert on the
// pagination loop.
type mockDeploymentListClient struct {
	resp     apitype.ListDeploymentResponseV2
	pages    map[int64]apitype.ListDeploymentResponseV2
	err      error
	captured *[]capturedListCall
}

func (m *mockDeploymentListClient) ListStackDeployments(
	_ context.Context, stack client.StackIdentifier, opts client.ListStackDeploymentsOptions,
) (apitype.ListDeploymentResponseV2, error) {
	if m.captured != nil {
		*m.captured = append(*m.captured, capturedListCall{stack: stack, opts: opts})
	}
	if m.err != nil {
		return apitype.ListDeploymentResponseV2{}, m.err
	}
	if m.pages != nil {
		return m.pages[opts.Page], nil
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

func defaultDeploymentListArgs() deploymentListArgs {
	return deploymentListArgs{renderOutput: renderDeploymentListTable}
}

func jsonDeploymentListArgs() deploymentListArgs {
	a := defaultDeploymentListArgs()
	a.renderOutput = renderDeploymentListJSON
	return a
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
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c), defaultDeploymentListArgs())
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

	// Footer summary.
	assert.Contains(t, out, "Showing 2 of 2 deployment(s)")
}

func TestDeploymentList_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentListClient{resp: apitype.ListDeploymentResponseV2{}}
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c), defaultDeploymentListArgs())
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
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c), defaultDeploymentListArgs())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Display Name")
}

func TestDeploymentList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentListClient{resp: sampleListResponse()}
	err := runDeploymentList(t.Context(), &buf, stubListFactory(c),
		jsonDeploymentListArgs())
	require.NoError(t, err)

	// Decode and check structural fields rather than comparing the whole
	// document string-for-string: the encoder may reorder nothing but the
	// indentation/whitespace assertions are brittle.
	var env deploymentListEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))

	assert.Equal(t, 2, env.Count)
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
		jsonDeploymentListArgs())
	require.NoError(t, err)

	assert.JSONEq(t,
		`{"deployments":[],"count":0,"total":0}`, buf.String())
}

func TestDeploymentList_PropagatesFlagsToClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args deploymentListArgs
		want client.ListStackDeploymentsOptions
	}{
		{
			name: "default asks for one page worth",
			args: deploymentListArgs{},
			want: client.ListStackDeploymentsOptions{Page: 1, PageSize: 10},
		},
		{
			name: "--count narrows the first page",
			args: deploymentListArgs{count: 25},
			want: client.ListStackDeploymentsOptions{Page: 1, PageSize: 25},
		},
		{
			name: "--all uses the default page size",
			args: deploymentListArgs{all: true},
			want: client.ListStackDeploymentsOptions{Page: 1, PageSize: defaultPageSize},
		},
		{
			name: "sort flags pass through",
			args: deploymentListArgs{sort: "modified", asc: true},
			want: client.ListStackDeploymentsOptions{Page: 1, PageSize: 10, Sort: "modified", Asc: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured []capturedListCall
			c := &mockDeploymentListClient{resp: apitype.ListDeploymentResponseV2{}, captured: &captured}
			var buf bytes.Buffer
			tt.args.renderOutput = renderDeploymentListTable
			err := runDeploymentList(t.Context(), &buf, stubListFactory(c), tt.args)
			require.NoError(t, err)
			require.NotEmpty(t, captured, "expected at least one client call")
			assert.Equal(t, tt.want, captured[0].opts)
			assert.Equal(t, testStackID, captured[0].stack)
		})
	}
}

// TestDeploymentList_AutoPaginates verifies the loop stitches multiple pages
// together to satisfy --count and --all without the caller having to ask.
func TestDeploymentList_AutoPaginates(t *testing.T) {
	t.Parallel()

	// 250 deployments total, served three pages deep.
	page := func(start, n int) apitype.ListDeploymentResponseV2 {
		ds := make([]apitype.ListDeploymentSnapshot, n)
		for i := range ds {
			ds[i] = apitype.ListDeploymentSnapshot{ID: fmt.Sprintf("dep-%d", start+i)}
		}
		return apitype.ListDeploymentResponseV2{Deployments: ds, Total: 250, ItemsPerPage: 100}
	}
	pages := map[int64]apitype.ListDeploymentResponseV2{
		1: page(1, 100),
		2: page(101, 100),
		3: page(201, 50),
	}

	t.Run("--all walks every page", func(t *testing.T) {
		t.Parallel()
		var captured []capturedListCall
		c := &mockDeploymentListClient{pages: pages, captured: &captured}
		var buf bytes.Buffer
		args := jsonDeploymentListArgs()
		args.all = true
		err := runDeploymentList(t.Context(), &buf, stubListFactory(c), args)
		require.NoError(t, err)

		var env deploymentListEnvelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
		assert.Equal(t, 250, env.Count)
		require.Len(t, captured, 3)
		assert.Equal(t, int64(1), captured[0].opts.Page)
		assert.Equal(t, int64(2), captured[1].opts.Page)
		assert.Equal(t, int64(3), captured[2].opts.Page)
	})

	t.Run("--count stops once enough collected", func(t *testing.T) {
		t.Parallel()
		var captured []capturedListCall
		c := &mockDeploymentListClient{pages: pages, captured: &captured}
		var buf bytes.Buffer
		args := jsonDeploymentListArgs()
		args.count = 150
		err := runDeploymentList(t.Context(), &buf, stubListFactory(c), args)
		require.NoError(t, err)

		var env deploymentListEnvelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
		assert.Equal(t, 150, env.Count)
		require.Len(t, captured, 2)
		// First call grabs the full default page; the second is sized to the
		// remainder rather than another full chunk.
		assert.Equal(t, int64(defaultPageSize), captured[0].opts.PageSize)
		assert.Equal(t, int64(50), captured[1].opts.PageSize)
	})
}

func TestDeploymentList_AllAndCountMutuallyExclusive(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentListClient{resp: apitype.ListDeploymentResponseV2{}}
	cmd := newDeploymentListCmdWith(stubListFactory(c))
	cmd.SetArgs([]string{"--all", "--count", "10"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--all and --count are mutually exclusive")
}
