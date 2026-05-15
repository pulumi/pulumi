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
	"encoding/json"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockScheduleEditClient struct {
	existing apitype.ScheduledAction
	err      error

	gotRawReq   *apitype.CreateScheduledDeploymentRequest
	gotDriftReq *apitype.CreateScheduledDriftDeploymentRequest
	gotTTLReq   *apitype.CreateScheduledTTLDeploymentRequest
}

func (m *mockScheduleEditClient) GetStackSchedule(
	_ context.Context, _ client.StackIdentifier, _ string,
) (apitype.ScheduledAction, error) {
	return m.existing, m.err
}

func (m *mockScheduleEditClient) UpdateStackSchedule(
	_ context.Context, _ client.StackIdentifier, _ string,
	req apitype.CreateScheduledDeploymentRequest,
) (apitype.ScheduledAction, error) {
	m.gotRawReq = &req
	return m.existing, nil
}

func (m *mockScheduleEditClient) UpdateStackDriftSchedule(
	_ context.Context, _ client.StackIdentifier, _ string,
	req apitype.CreateScheduledDriftDeploymentRequest,
) (apitype.ScheduledAction, error) {
	m.gotDriftReq = &req
	return m.existing, nil
}

func (m *mockScheduleEditClient) UpdateStackTTLSchedule(
	_ context.Context, _ client.StackIdentifier, _ string,
	req apitype.CreateScheduledTTLDeploymentRequest,
) (apitype.ScheduledAction, error) {
	m.gotTTLReq = &req
	return m.existing, nil
}

func editClientFactory(c stackScheduleEditClient) stackScheduleEditClientFactory {
	return func(_ context.Context, _ string) (stackScheduleEditClient, client.StackIdentifier, error) {
		return c, testScheduleStackID, nil
	}
}

func defWithOpts(t *testing.T, op apitype.PulumiOperation, opts *apitype.OperationContextOptions) json.RawMessage {
	t.Helper()
	req := &apitype.CreateDeploymentRequest{Op: op, InheritSettings: true}
	if opts != nil {
		req.Operation = &apitype.OperationContext{Options: opts}
	}
	def := apitype.ScheduledDeploymentDefinition{
		ProgramID: "5f337707",
		Request:   req,
	}
	b, err := json.Marshal(def)
	require.NoError(t, err)
	return b
}

func rawSchedule(t *testing.T) apitype.ScheduledAction {
	t.Helper()
	return apitype.ScheduledAction{
		ID:           "raw-id",
		ScheduleCron: "0 */4 * * *",
		Kind:         apitype.ScheduledActionKindDeployment,
		Definition:   defWithOpts(t, apitype.Refresh, nil),
	}
}

func driftSchedule(t *testing.T) apitype.ScheduledAction {
	t.Helper()
	return apitype.ScheduledAction{
		ID:           "drift-id",
		ScheduleCron: "0 */4 * * *",
		Kind:         apitype.ScheduledActionKindDeployment,
		Definition:   defWithOpts(t, apitype.DetectDrift, &apitype.OperationContextOptions{RemediateIfDriftDetected: true}),
	}
}

func ttlSchedule(t *testing.T) apitype.ScheduledAction {
	t.Helper()
	return apitype.ScheduledAction{
		ID:           "ttl-id",
		ScheduleOnce: "2026-12-31T23:59:00Z",
		Kind:         apitype.ScheduledActionKindDeployment,
		Definition:   defWithOpts(t, apitype.Destroy, &apitype.OperationContextOptions{DeleteAfterDestroy: true}),
	}
}

func TestStackScheduleEdit_Raw_ChangesCron(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: rawSchedule(t)}
	flags := stackScheduleEditFlags{cron: "0 */6 * * *", cronChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "raw-id", flags, renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotRawReq)
	assert.Equal(t, "0 */6 * * *", c.gotRawReq.ScheduleCron)
	assert.Empty(t, c.gotRawReq.ScheduleOnce)
	require.NotNil(t, c.gotRawReq.Request)
	assert.Equal(t, apitype.Refresh, c.gotRawReq.Request.Op) // preserved
	assert.Nil(t, c.gotDriftReq)
	assert.Nil(t, c.gotTTLReq)
}

func TestStackScheduleEdit_Raw_SwapCronForOnce(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: rawSchedule(t)}
	flags := stackScheduleEditFlags{once: "2026-12-31T23:59:00Z", onceChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "raw-id", flags, renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotRawReq)
	assert.Empty(t, c.gotRawReq.ScheduleCron)
	assert.Equal(t, "2026-12-31T23:59:00Z", c.gotRawReq.ScheduleOnce)
}

func TestStackScheduleEdit_Raw_ChangesOperation(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: rawSchedule(t)}
	flags := stackScheduleEditFlags{operation: "destroy", operationChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "raw-id", flags, renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotRawReq.Request)
	assert.Equal(t, apitype.Destroy, c.gotRawReq.Request.Op)
	// Cron preserved when not changed.
	assert.Equal(t, "0 */4 * * *", c.gotRawReq.ScheduleCron)
}

func TestStackScheduleEdit_Raw_InvalidOperation(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: rawSchedule(t)}
	flags := stackScheduleEditFlags{operation: "detect-drift", operationChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "raw-id", flags, renderScheduleGetText,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --operation")
}

func TestStackScheduleEdit_Drift_PreservesAutoRemediate(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: driftSchedule(t)}
	// User only changes the cron; auto-remediate must be preserved as true.
	flags := stackScheduleEditFlags{cron: "0 */6 * * *", cronChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "drift-id", flags, renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotDriftReq)
	assert.Equal(t, "0 */6 * * *", c.gotDriftReq.ScheduleCron)
	assert.True(t, c.gotDriftReq.AutoRemediate) // preserved
}

func TestStackScheduleEdit_Drift_DisablesAutoRemediate(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: driftSchedule(t)}
	// --auto-remediate=false explicitly disables it.
	flags := stackScheduleEditFlags{autoRemediate: false, autoRemediateChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "drift-id", flags, renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotDriftReq)
	assert.False(t, c.gotDriftReq.AutoRemediate)
	assert.Equal(t, "0 */4 * * *", c.gotDriftReq.ScheduleCron) // preserved
}

func TestStackScheduleEdit_TTL_PreservesDeleteAfterDestroy(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: ttlSchedule(t)}
	// Changing the timestamp must preserve delete-after-destroy.
	flags := stackScheduleEditFlags{once: "2027-01-01T00:00:00Z", onceChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "ttl-id", flags, renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotTTLReq)
	assert.Equal(t, "2027-01-01T00:00:00Z", c.gotTTLReq.Timestamp)
	assert.True(t, c.gotTTLReq.DeleteAfterDestroy) // preserved
}

func TestStackScheduleEdit_TTL_DisablesDeleteAfterDestroy(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: ttlSchedule(t)}
	flags := stackScheduleEditFlags{deleteAfterDestroy: false, deleteAfterDestroyChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "ttl-id", flags, renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotTTLReq)
	assert.False(t, c.gotTTLReq.DeleteAfterDestroy)
	assert.Equal(t, "2026-12-31T23:59:00Z", c.gotTTLReq.Timestamp) // preserved
}

func TestStackScheduleEdit_RejectsWrongKindFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		existing apitype.ScheduledAction
		flags    stackScheduleEditFlags
		want     string
	}{
		{
			name:     "raw rejects auto-remediate",
			existing: rawSchedule(t),
			flags:    stackScheduleEditFlags{autoRemediate: true, autoRemediateChanged: true},
			want:     "--auto-remediate is only valid for drift",
		},
		{
			name:     "raw rejects delete-after-destroy",
			existing: rawSchedule(t),
			flags:    stackScheduleEditFlags{deleteAfterDestroy: true, deleteAfterDestroyChanged: true},
			want:     "--delete-after-destroy is only valid for ttl",
		},
		{
			name:     "drift rejects once",
			existing: driftSchedule(t),
			flags:    stackScheduleEditFlags{once: "2026-12-31T23:59:00Z", onceChanged: true},
			want:     "--once is not valid for drift",
		},
		{
			name:     "drift rejects operation",
			existing: driftSchedule(t),
			flags:    stackScheduleEditFlags{operation: "update", operationChanged: true},
			want:     "--operation is not valid for drift",
		},
		{
			name:     "drift rejects delete-after-destroy",
			existing: driftSchedule(t),
			flags:    stackScheduleEditFlags{deleteAfterDestroy: true, deleteAfterDestroyChanged: true},
			want:     "--delete-after-destroy is only valid for ttl",
		},
		{
			name:     "ttl rejects cron",
			existing: ttlSchedule(t),
			flags:    stackScheduleEditFlags{cron: "0 */4 * * *", cronChanged: true},
			want:     "--cron is not valid for ttl",
		},
		{
			name:     "ttl rejects operation",
			existing: ttlSchedule(t),
			flags:    stackScheduleEditFlags{operation: "update", operationChanged: true},
			want:     "--operation is not valid for ttl",
		},
		{
			name:     "ttl rejects auto-remediate",
			existing: ttlSchedule(t),
			flags:    stackScheduleEditFlags{autoRemediate: true, autoRemediateChanged: true},
			want:     "--auto-remediate is only valid for drift",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &mockScheduleEditClient{existing: tc.existing}
			err := runStackScheduleEdit(
				t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "id", tc.flags, renderScheduleGetText,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestStackScheduleEdit_GetError(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{err: errors.New("not found")}
	flags := stackScheduleEditFlags{cron: "0 */6 * * *", cronChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "no-such",
		flags, renderScheduleGetText,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading stack schedule")
	assert.Contains(t, err.Error(), "not found")
}

func TestStackScheduleEdit_TTL_NormalizesExistingTimestamp(t *testing.T) {
	t.Parallel()

	existing := ttlSchedule(t)
	existing.ScheduleOnce = "2026-12-31 23:59:00.000"
	c := &mockScheduleEditClient{existing: existing}

	flags := stackScheduleEditFlags{deleteAfterDestroy: false, deleteAfterDestroyChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "ttl-id", flags, renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotTTLReq)
	assert.Equal(t, "2026-12-31T23:59:00Z", c.gotTTLReq.Timestamp)
}

func TestStackScheduleEdit_Raw_NormalizesExistingTimestamp(t *testing.T) {
	t.Parallel()

	existing := apitype.ScheduledAction{
		ID:           "raw-id",
		ScheduleOnce: "2026-12-31 23:59:00.000",
		Kind:         apitype.ScheduledActionKindDeployment,
		Definition:   defWithOpts(t, apitype.Refresh, nil),
	}
	c := &mockScheduleEditClient{existing: existing}

	flags := stackScheduleEditFlags{operation: "update", operationChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "raw-id", flags, renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotRawReq)
	assert.Equal(t, "2026-12-31T23:59:00Z", c.gotRawReq.ScheduleOnce)
}

func TestStackScheduleEdit_TTL_RejectsInvalidOnce(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: ttlSchedule(t)}
	flags := stackScheduleEditFlags{once: "not-a-timestamp", onceChanged: true}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "ttl-id", flags, renderScheduleGetText,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--once invalid timestamp")
	assert.Nil(t, c.gotTTLReq)
}

func TestStackScheduleEdit_NoFlags(t *testing.T) {
	t.Parallel()

	c := &mockScheduleEditClient{existing: rawSchedule(t)}
	err := runStackScheduleEdit(
		t.Context(), &bytes.Buffer{}, editClientFactory(c), "", "raw-id",
		stackScheduleEditFlags{}, renderScheduleGetText,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of")
	// No request should have been issued.
	assert.Nil(t, c.gotRawReq)
	assert.Nil(t, c.gotDriftReq)
	assert.Nil(t, c.gotTTLReq)
}
