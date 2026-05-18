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
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDriftListClient struct {
	// pages maps page number to response.
	pages   map[int]apitype.ListDriftRunsResponse
	err     error
	gotPage []int
	gotSize []int
}

func (m *mockDriftListClient) ListDriftRuns(
	_ context.Context, _ client.StackIdentifier, page, pageSize int,
) (apitype.ListDriftRunsResponse, error) {
	m.gotPage = append(m.gotPage, page)
	m.gotSize = append(m.gotSize, pageSize)
	if m.err != nil {
		return apitype.ListDriftRunsResponse{}, m.err
	}
	if resp, ok := m.pages[page]; ok {
		return resp, nil
	}
	return apitype.ListDriftRunsResponse{}, nil
}

var driftTestStackID = client.StackIdentifier{
	Owner:   "my-org",
	Project: "my-project",
	Stack:   tokens.MustParseStackName("dev"),
}

func stubDriftListFactory(c driftListClient) driftListClientFactory {
	return func(_ context.Context, _ string) (driftListClient, client.StackIdentifier, error) {
		return c, driftTestStackID, nil
	}
}

func newTestDriftListCmd() (*driftListCmd, *bytes.Buffer) {
	var buf bytes.Buffer
	return &driftListCmd{
		output: outputflag.OutputFlag[driftListRender]{
			RenderForTerminal: (*driftListCmd).renderTable,
			RenderJSON:        (*driftListCmd).renderJSON,
		},
		w: &buf,
	}, &buf
}

func singlePageRuns() *mockDriftListClient {
	return &mockDriftListClient{
		pages: map[int]apitype.ListDriftRunsResponse{
			1: {
				DriftRuns: []apitype.DriftRun{
					{
						ID:            "run-1",
						DriftDetected: true,
						Created:       "2026-05-13T10:00:00Z",
						Status:        "succeeded",
						DetectUpdate: &apitype.DriftRunUpdate{
							UpdateID:        "u-1",
							ResourceChanges: map[string]int{"update": 2, "same": 5},
							Modified:        "2026-05-13T10:01:00Z",
							Status:          "succeeded",
						},
						RemediateUpdate: &apitype.DriftRunUpdate{
							UpdateID:        "u-2",
							ResourceChanges: map[string]int{"update": 2},
							Modified:        "2026-05-13T10:02:00Z",
							Status:          "succeeded",
						},
					},
					{
						ID:            "run-2",
						DriftDetected: false,
						Created:       "2026-05-12T10:00:00Z",
						Status:        "succeeded",
						DetectUpdate: &apitype.DriftRunUpdate{
							UpdateID: "u-3",
							Modified: "2026-05-12T10:01:00Z",
							Status:   "succeeded",
						},
					},
				},
				ItemsPerPage: 10,
				Total:        2,
			},
		},
	}
}

func TestDriftList_TableOutput(t *testing.T) {
	t.Parallel()

	c := singlePageRuns()
	dlcmd, buf := newTestDriftListCmd()
	err := dlcmd.run(t.Context(), stubDriftListFactory(c))
	require.NoError(t, err)

	out := buf.String()
	// Verify all data is present (exact layout depends on terminal width;
	// long values like "succeeded (2 update)" may wrap across lines).
	assert.Contains(t, out, "run-1")
	assert.Contains(t, out, "2026-05-13T10:00:00Z")
	assert.Contains(t, out, "succeeded")
	assert.Contains(t, out, "yes")
	assert.Contains(t, out, "run-2")
	assert.Contains(t, out, "no")
	assert.Contains(t, out, "Showing 2 of 2 drift run(s)")
}

func TestDriftList_TableOutput_Empty(t *testing.T) {
	t.Parallel()

	c := &mockDriftListClient{pages: map[int]apitype.ListDriftRunsResponse{
		1: {Total: 0},
	}}
	dlcmd, buf := newTestDriftListCmd()
	err := dlcmd.run(t.Context(), stubDriftListFactory(c))
	require.NoError(t, err)

	assert.Equal(t, "No drift detection runs found for this stack.\n", buf.String())
}

func TestDriftList_JSONOutput(t *testing.T) {
	t.Parallel()

	c := singlePageRuns()
	dlcmd, buf := newTestDriftListCmd()
	require.NoError(t, dlcmd.output.Set("json"))
	err := dlcmd.run(t.Context(), stubDriftListFactory(c))
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, `"driftRuns"`)
	assert.Contains(t, out, `"run-1"`)
	assert.Contains(t, out, `"total": 2`)
	assert.Contains(t, out, `"count": 2`)
}

func TestDriftList_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockDriftListClient{err: errors.New("server error")}
	dlcmd, _ := newTestDriftListCmd()
	err := dlcmd.run(t.Context(), stubDriftListFactory(c))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing drift runs")
}

func TestDriftList_CountFlag(t *testing.T) {
	t.Parallel()

	c := &mockDriftListClient{
		pages: map[int]apitype.ListDriftRunsResponse{
			1: {
				DriftRuns: []apitype.DriftRun{
					{ID: "r-1", Status: "succeeded"},
					{ID: "r-2", Status: "succeeded"},
				},
				ItemsPerPage: 2,
				Total:        3,
			},
			2: {
				DriftRuns:    []apitype.DriftRun{{ID: "r-3", Status: "succeeded"}},
				ItemsPerPage: 2,
				Total:        3,
			},
		},
	}
	dlcmd, buf := newTestDriftListCmd()
	dlcmd.count = 2
	err := dlcmd.run(t.Context(), stubDriftListFactory(c))
	require.NoError(t, err)

	assert.Equal(t, []int{1}, c.gotPage)
	assert.Contains(t, buf.String(), "r-1")
	assert.Contains(t, buf.String(), "r-2")
	assert.NotContains(t, buf.String(), "r-3")
	assert.Contains(t, buf.String(), "Showing 2 of 3")
}

func TestDriftList_AllFlag(t *testing.T) {
	t.Parallel()

	c := &mockDriftListClient{
		pages: map[int]apitype.ListDriftRunsResponse{
			1: {
				DriftRuns: []apitype.DriftRun{
					{ID: "r-1", Status: "succeeded"},
					{ID: "r-2", Status: "succeeded"},
				},
				ItemsPerPage: 2,
				Total:        3,
			},
			2: {
				DriftRuns:    []apitype.DriftRun{{ID: "r-3", Status: "succeeded"}},
				ItemsPerPage: 2,
				Total:        3,
			},
		},
	}
	dlcmd, buf := newTestDriftListCmd()
	dlcmd.all = true
	err := dlcmd.run(t.Context(), stubDriftListFactory(c))
	require.NoError(t, err)

	assert.Equal(t, []int{1, 2}, c.gotPage)
	assert.Contains(t, buf.String(), "r-1")
	assert.Contains(t, buf.String(), "r-3")
	assert.Contains(t, buf.String(), "Showing 3 of 3")
}

func TestDriftList_AllAndCountMutuallyExclusive(t *testing.T) {
	t.Parallel()

	c := singlePageRuns()
	cmd := newStackDriftListCmdWith(stubDriftListFactory(c))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--all", "--count", "5"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--all and --count are mutually exclusive")
}
