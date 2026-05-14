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

// AI Generated - needs human review

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
// response (or error) and records the call inputs for assertions.
type mockDeploymentLogClient struct {
	resp    *apitype.DeploymentLogs
	err     error
	gotID   string
	gotOpts client.GetDeploymentLogsOptions
}

func (m *mockDeploymentLogClient) GetDeploymentLogs(
	_ context.Context, _ client.StackIdentifier, id string,
	opts client.GetDeploymentLogsOptions,
) (*apitype.DeploymentLogs, error) {
	m.gotID = id
	m.gotOpts = opts
	if m.err != nil {
		return nil, m.err
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

	resp := &apitype.DeploymentLogs{
		Lines: []apitype.DeploymentLogLine{
			{Header: "pulumi up", Line: "running update"},
			{Line: "plain log line"},
		},
	}
	c := &mockDeploymentLogClient{resp: resp}

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1",
		defaultDeploymentLogArgs())
	require.NoError(t, err)

	assert.Equal(t, "[pulumi up] running update\nplain log line\n", buf.String())
}

func TestDeploymentLog_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}
	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1",
		defaultDeploymentLogArgs())
	require.NoError(t, err)

	assert.Equal(t, "No log lines available.\n", buf.String())
}

func TestDeploymentLog_DefaultOutput_NextTokenHint(t *testing.T) {
	t.Parallel()

	resp := &apitype.DeploymentLogs{
		Lines:     []apitype.DeploymentLogLine{{Line: "first line"}},
		NextToken: "tok-abc",
	}
	c := &mockDeploymentLogClient{resp: resp}

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1",
		defaultDeploymentLogArgs())
	require.NoError(t, err)

	assert.Equal(t,
		"first line\n\nMore log lines available. Re-run with --continuation-token \"tok-abc\" to continue.\n",
		buf.String())
}

func TestDeploymentLog_JSONOutput(t *testing.T) {
	t.Parallel()

	resp := &apitype.DeploymentLogs{
		Lines: []apitype.DeploymentLogLine{
			{Header: "h1", Line: "line a"},
			{Line: "line b"},
		},
		NextToken: "next-1",
	}
	c := &mockDeploymentLogClient{resp: resp}

	args := defaultDeploymentLogArgs()
	args.output = "json"

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1", args)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"lines": [
			{"header": "h1", "timestamp": "0001-01-01T00:00:00Z", "line": "line a"},
			{"timestamp": "0001-01-01T00:00:00Z", "line": "line b"}
		],
		"nextToken": "next-1"
	}`, buf.String())
}

func TestDeploymentLog_JSONOutput_NilLinesNormalized(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}
	args := defaultDeploymentLogArgs()
	args.output = "json"

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1", args)
	require.NoError(t, err)

	assert.JSONEq(t, `{"lines": [], "nextToken": ""}`, buf.String())
}

func TestDeploymentLog_InvalidOutput(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}
	args := defaultDeploymentLogArgs()
	args.output = "yaml"

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1", args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --output value "yaml"`)
}

func TestDeploymentLog_StepRequiresJob(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}
	args := defaultDeploymentLogArgs()
	args.step = 2 // job stays at -1

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-1", args)
	require.Error(t, err)
	assert.Equal(t, "--step requires --job to also be set (>= 0)", err.Error())
}

func TestDeploymentLog_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{err: errors.New("not found")}
	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-missing",
		defaultDeploymentLogArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting deployment logs")
	assert.Contains(t, err.Error(), "not found")
}

func TestDeploymentLog_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf,
		failingLogFactory(errors.New("not logged in")), "dep-1", defaultDeploymentLogArgs())
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
	args.continuationToken = "tok"

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-id", args)
	require.NoError(t, err)

	job, step, offset, count := 1, 2, 3, 4
	assert.Equal(t, "dep-id", c.gotID)
	assert.Equal(t, client.GetDeploymentLogsOptions{
		Job:               &job,
		Step:              &step,
		Offset:            &offset,
		Count:             &count,
		ContinuationToken: "tok",
	}, c.gotOpts)
}

func TestDeploymentLog_OptionsPropagation_AllUnset(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentLogClient{resp: &apitype.DeploymentLogs{}}

	var buf bytes.Buffer
	err := runDeploymentLog(t.Context(), &buf, stubLogFactory(c), "dep-id",
		defaultDeploymentLogArgs())
	require.NoError(t, err)

	assert.Equal(t, client.GetDeploymentLogsOptions{}, c.gotOpts)
}
