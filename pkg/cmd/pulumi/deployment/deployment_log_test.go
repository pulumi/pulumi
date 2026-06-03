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
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDeploymentLogClient stubs deploymentLogClient. It returns a fixed
// response (or error) and records the call inputs for assertions. When
// queuedResps is non-nil the mock returns the head of the queue on each
// call, allowing tests to exercise the --all pagination loop.
type mockDeploymentLogClient struct {
	resp        *apitype.DeploymentLogs
	queuedResps []*apitype.DeploymentLogs
	err         error
	gotID       string
	gotOpts     client.GetDeploymentLogsOptions
	gotOptsAll  []client.GetDeploymentLogsOptions

	// Version-resolution stubs. byVersionResp is returned for any
	// GetDeploymentByVersion call; byVersionErr forces a failure. gotVersion
	// records the version the command asked us to resolve.
	byVersionResp apitype.GetDeploymentResponse
	byVersionErr  error
	gotVersion    string
}

func (m *mockDeploymentLogClient) GetDeploymentByVersion(
	_ context.Context, _ client.StackIdentifier, version string,
) (apitype.GetDeploymentResponse, error) {
	m.gotVersion = version
	if m.byVersionErr != nil {
		return apitype.GetDeploymentResponse{}, m.byVersionErr
	}
	return m.byVersionResp, nil
}

func (m *mockDeploymentLogClient) GetDeploymentLogs(
	_ context.Context, _ client.StackIdentifier, id string,
	opts client.GetDeploymentLogsOptions,
) (*apitype.DeploymentLogs, error) {
	m.gotID = id
	m.gotOpts = opts
	m.gotOptsAll = append(m.gotOptsAll, opts)
	if m.err != nil {
		return nil, m.err
	}
	if len(m.queuedResps) > 0 {
		head := m.queuedResps[0]
		m.queuedResps = m.queuedResps[1:]
		return head, nil
	}
	return m.resp, nil
}

func stubLogFactory(c deploymentLogClient) deploymentLogClientFactory {
	return func(_ context.Context, _ string) (deploymentLogClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func failingLogFactory(err error) deploymentLogClientFactory {
	return func(_ context.Context, _ string) (deploymentLogClient, client.StackIdentifier, error) {
		return nil, client.StackIdentifier{}, err
	}
}

func TestDeploymentLog_DefaultOutput(t *testing.T) {
	t.Parallel()

	// Real server payloads put Header on its own row (no Line) and end
	// each body Line with "\n" — the workflow runner appends the newline
	// when it stores the log row. The renderer must trust that.
	resp := &apitype.DeploymentLogs{
		Lines: []apitype.DeploymentLogLine{
			{Header: "pulumi up"},
			{Line: "running update\n"},
			{Line: "plain log line\n"},
		},
	}
	c := &mockDeploymentLogClient{resp: resp}

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1",
		defaultDeploymentLogArgs(), renderDeploymentLogText)
	require.NoError(t, err)

	assert.Equal(t, "[pulumi up]\nrunning update\nplain log line\n", buf.String())
}

func TestDeploymentLog_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}
	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1",
		defaultDeploymentLogArgs(), renderDeploymentLogText)
	require.NoError(t, err)

	assert.Equal(t, "No log lines available.\n", buf.String())
}

func TestDeploymentLog_JSONOutput(t *testing.T) {
	t.Parallel()

	resp := &apitype.DeploymentLogs{
		Lines: []apitype.DeploymentLogLine{
			{Header: "h1", Line: "line a"},
			{Line: "line b"},
		},
	}
	c := &mockDeploymentLogClient{resp: resp}

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1",
		defaultDeploymentLogArgs(), renderDeploymentLogJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"lines": [
			{"header": "h1", "timestamp": "0001-01-01T00:00:00Z", "line": "line a"},
			{"timestamp": "0001-01-01T00:00:00Z", "line": "line b"}
		]
	}`, buf.String())
}

func TestDeploymentLog_JSONOutput_NilLinesNormalized(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1",
		defaultDeploymentLogArgs(), renderDeploymentLogJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{"lines": []}`, buf.String())
}

func TestDeploymentLog_StepRequiresJob(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}
	args := defaultDeploymentLogArgs()
	args.step = 2 // job stays at -1

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1", args, renderDeploymentLogText)
	require.Error(t, err)
	assert.Equal(t, "--step requires --job to also be set (>= 0)", err.Error())
}

func TestDeploymentLog_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{err: errors.New("not found")}
	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-missing",
		defaultDeploymentLogArgs(), renderDeploymentLogText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting deployment logs")
	assert.Contains(t, err.Error(), "not found")
}

func TestDeploymentLog_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf,
		failingLogFactory(errors.New("not logged in")), "dep-1", defaultDeploymentLogArgs(),
		renderDeploymentLogText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestDeploymentLog_OptionsPropagation(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}
	args := defaultDeploymentLogArgs()
	args.job = 1
	args.step = 2
	args.offset = 3
	args.count = 4

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-id", args, renderDeploymentLogText)
	require.NoError(t, err)

	job, step, offset, count := 1, 2, 3, 4
	assert.Equal(t, "dep-id", c.gotID)
	assert.Equal(t, client.GetDeploymentLogsOptions{
		Job:    &job,
		Step:   &step,
		Offset: &offset,
		Count:  &count,
	}, c.gotOpts)
}

func TestDeploymentLog_OptionsPropagation_AllUnset(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-id",
		defaultDeploymentLogArgs(), renderDeploymentLogText)
	require.NoError(t, err)

	assert.Equal(t, client.GetDeploymentLogsOptions{}, c.gotOpts)
}

func TestDeploymentLog_ResolvesVersionRef(t *testing.T) {
	t.Parallel()

	for _, ref := range []string{"#9410", "9410"} {
		t.Run(ref, func(t *testing.T) {
			t.Parallel()

			c := &mockDeploymentLogClient{
				resp:          &apitype.DeploymentLogs{},
				byVersionResp: apitype.GetDeploymentResponse{ID: "uuid-from-version"},
			}

			var buf bytes.Buffer
			err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), ref,
				defaultDeploymentLogArgs(), renderDeploymentLogText)
			require.NoError(t, err)

			assert.Equal(t, "9410", c.gotVersion)
			assert.Equal(t, "uuid-from-version", c.gotID)
		})
	}
}

func TestDeploymentLog_PassesUUIDThrough(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c),
		"0a1b2c3d-1111-2222-3333-444455556666", defaultDeploymentLogArgs(),
		renderDeploymentLogText)
	require.NoError(t, err)

	assert.Equal(t, "", c.gotVersion, "non-numeric ref must not trigger a version lookup")
	assert.Equal(t, "0a1b2c3d-1111-2222-3333-444455556666", c.gotID)
}

func TestDeploymentLog_VersionLookupError(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{
		resp:         &apitype.DeploymentLogs{},
		byVersionErr: errors.New("not found"),
	}

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "#9410",
		defaultDeploymentLogArgs(), renderDeploymentLogText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving deployment version 9410")
	assert.Contains(t, err.Error(), "not found")
	assert.Equal(t, "", c.gotID, "logs endpoint must not be called when resolution fails")
}

func TestDeploymentLog_AllFollowsContinuationToken(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{
		queuedResps: []*apitype.DeploymentLogs{
			{Lines: []apitype.DeploymentLogLine{{Line: "page-1\n"}}, NextToken: "tok-1"},
			{Lines: []apitype.DeploymentLogLine{{Line: "page-2\n"}}, NextToken: "tok-2"},
			{Lines: []apitype.DeploymentLogLine{{Line: "page-3\n"}}, NextToken: ""},
		},
	}

	args := defaultDeploymentLogArgs()
	args.all = true

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-id", args, renderDeploymentLogText)
	require.NoError(t, err)

	assert.Equal(t, "page-1\npage-2\npage-3\n", buf.String())
	require.Len(t, c.gotOptsAll, 3)
	assert.Equal(t, "", c.gotOptsAll[0].ContinuationToken)
	assert.Equal(t, "tok-1", c.gotOptsAll[1].ContinuationToken)
	assert.Equal(t, "tok-2", c.gotOptsAll[2].ContinuationToken)
}
