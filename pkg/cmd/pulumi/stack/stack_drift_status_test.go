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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDriftStatusClient struct {
	status apitype.StackDriftStatus
	err    error
}

func (m *mockDriftStatusClient) GetDriftStatus(
	_ context.Context, _ client.StackIdentifier,
) (apitype.StackDriftStatus, error) {
	return m.status, m.err
}

func stubDriftStatusFactory(c driftStatusClient) driftStatusClientFactory {
	return func(_ context.Context, _ string) (driftStatusClient, client.StackIdentifier, error) {
		return c, driftTestStackID, nil
	}
}

func newTestDriftStatusCmd() (*driftStatusCmd, *bytes.Buffer) {
	var buf bytes.Buffer
	return &driftStatusCmd{
		output: outputflag.OutputFlag[driftStatusRender]{
			RenderForTerminal: (*driftStatusCmd).renderText,
			RenderJSON:        (*driftStatusCmd).renderJSON,
		},
		w: &buf,
	}, &buf
}

func TestDriftStatus_TextOutput_DriftDetected(t *testing.T) {
	t.Parallel()

	c := &mockDriftStatusClient{status: apitype.StackDriftStatus{
		DriftDetected:  true,
		LatestDriftRun: "run-abc-123",
		RunInProgress:  false,
	}}
	dscmd, buf := newTestDriftStatusCmd()
	err := dscmd.run(t.Context(), stubDriftStatusFactory(c))
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Drift detected:    yes")
	assert.Contains(t, out, "Latest drift run:  run-abc-123")
	assert.Contains(t, out, "Run in progress:   no")
}

func TestDriftStatus_TextOutput_NoDrift(t *testing.T) {
	t.Parallel()

	c := &mockDriftStatusClient{status: apitype.StackDriftStatus{
		DriftDetected:  false,
		LatestDriftRun: "",
		RunInProgress:  false,
	}}
	dscmd, buf := newTestDriftStatusCmd()
	err := dscmd.run(t.Context(), stubDriftStatusFactory(c))
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Drift detected:    no")
	assert.NotContains(t, out, "Latest drift run:")
	assert.Contains(t, out, "Run in progress:   no")
}

func TestDriftStatus_TextOutput_RunInProgress(t *testing.T) {
	t.Parallel()

	c := &mockDriftStatusClient{status: apitype.StackDriftStatus{
		DriftDetected:  false,
		LatestDriftRun: "run-xyz",
		RunInProgress:  true,
	}}
	dscmd, buf := newTestDriftStatusCmd()
	err := dscmd.run(t.Context(), stubDriftStatusFactory(c))
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Run in progress:   yes")
}

func TestDriftStatus_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockDriftStatusClient{status: apitype.StackDriftStatus{
		DriftDetected:  true,
		LatestDriftRun: "run-abc-123",
		RunInProgress:  false,
	}}
	dscmd, buf := newTestDriftStatusCmd()
	require.NoError(t, dscmd.output.Set("json"))
	err := dscmd.run(t.Context(), stubDriftStatusFactory(c))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"driftDetected": true,
		"latestDriftRun": "run-abc-123",
		"runInProgress": false
	}`, buf.String())
}

func TestDriftStatus_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockDriftStatusClient{err: errors.New("server error")}
	dscmd, _ := newTestDriftStatusCmd()
	err := dscmd.run(t.Context(), stubDriftStatusFactory(c))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting drift status")
}
