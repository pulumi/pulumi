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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockScheduleNewClient struct {
	schedule      apitype.ScheduledAction
	err           error
	gotRawReq     *apitype.CreateScheduledDeploymentRequest
	gotDriftReq   *apitype.CreateScheduledDriftDeploymentRequest
	gotTTLReq     *apitype.CreateScheduledTTLDeploymentRequest
	createdViaTTL bool
}

func (m *mockScheduleNewClient) CreateStackSchedule(
	_ context.Context, _ client.StackIdentifier, req apitype.CreateScheduledDeploymentRequest,
) (apitype.ScheduledAction, error) {
	m.gotRawReq = &req
	return m.schedule, m.err
}

func (m *mockScheduleNewClient) CreateStackDriftSchedule(
	_ context.Context, _ client.StackIdentifier, req apitype.CreateScheduledDriftDeploymentRequest,
) (apitype.ScheduledAction, error) {
	m.gotDriftReq = &req
	return m.schedule, m.err
}

func (m *mockScheduleNewClient) CreateStackTTLSchedule(
	_ context.Context, _ client.StackIdentifier, req apitype.CreateScheduledTTLDeploymentRequest,
) (apitype.ScheduledAction, error) {
	m.gotTTLReq = &req
	m.createdViaTTL = true
	return m.schedule, m.err
}

func newClientFactory(c stackScheduleNewClient) stackScheduleNewClientFactory {
	return func(_ context.Context, _ string) (stackScheduleNewClient, client.StackIdentifier, error) {
		return c, testScheduleStackID, nil
	}
}

func rawArgs(operation, cron, once string) stackScheduleNewArgs {
	return stackScheduleNewArgs{
		kind:      scheduleKindRaw,
		cron:      cron,
		once:      once,
		operation: operation,
	}
}

func TestStackScheduleNew_Raw_Cron(t *testing.T) {
	t.Parallel()

	c := &mockScheduleNewClient{
		schedule: apitype.ScheduledAction{
			ID:           "bb61b60a",
			ScheduleCron: "0 */4 * * *",
			Kind:         apitype.ScheduledActionKindDeployment,
			Definition:   deploymentDefinitionJSON(t, apitype.Update),
		},
	}
	var buf bytes.Buffer
	err := runStackScheduleNew(
		t.Context(), &buf, newClientFactory(c), rawArgs("update", "0 */4 * * *", ""), renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotRawReq)
	assert.Equal(t, "0 */4 * * *", c.gotRawReq.ScheduleCron)
	assert.Empty(t, c.gotRawReq.ScheduleOnce)
	require.NotNil(t, c.gotRawReq.Request)
	assert.Equal(t, apitype.Update, c.gotRawReq.Request.Op)
	assert.True(t, c.gotRawReq.Request.InheritSettings)
	assert.Nil(t, c.gotDriftReq)
	assert.Nil(t, c.gotTTLReq)

	assert.Contains(t, buf.String(), "Type:      raw")
	assert.Contains(t, buf.String(), "Settings:  pulumi update")
	assert.Contains(t, buf.String(), "Schedule:  0 */4 * * *")
}

func TestStackScheduleNew_Raw_Once(t *testing.T) {
	t.Parallel()

	c := &mockScheduleNewClient{
		schedule: apitype.ScheduledAction{
			ID:           "once",
			ScheduleOnce: "2026-12-31T23:59:00Z",
			Kind:         apitype.ScheduledActionKindDeployment,
			Definition:   deploymentDefinitionJSON(t, apitype.Destroy),
		},
	}
	err := runStackScheduleNew(
		t.Context(), &bytes.Buffer{}, newClientFactory(c),
		rawArgs("destroy", "", "2026-12-31T23:59:00Z"), renderScheduleGetText,
	)
	require.NoError(t, err)

	require.NotNil(t, c.gotRawReq)
	assert.Empty(t, c.gotRawReq.ScheduleCron)
	assert.Equal(t, "2026-12-31T23:59:00Z", c.gotRawReq.ScheduleOnce)
	assert.Equal(t, apitype.Destroy, c.gotRawReq.Request.Op)
}

func TestStackScheduleNew_Drift(t *testing.T) {
	t.Parallel()

	c := &mockScheduleNewClient{
		schedule: apitype.ScheduledAction{
			ID:           "drift",
			ScheduleCron: "0 */4 * * *",
			Kind:         apitype.ScheduledActionKindDeployment,
			Definition:   deploymentDefinitionJSON(t, apitype.DetectDrift),
		},
	}
	err := runStackScheduleNew(t.Context(), &bytes.Buffer{}, newClientFactory(c), stackScheduleNewArgs{
		kind:          scheduleKindDrift,
		cron:          "0 */4 * * *",
		autoRemediate: true,
	}, renderScheduleGetText)
	require.NoError(t, err)

	require.NotNil(t, c.gotDriftReq)
	assert.Equal(t, "0 */4 * * *", c.gotDriftReq.ScheduleCron)
	assert.True(t, c.gotDriftReq.AutoRemediate)
	assert.Nil(t, c.gotRawReq)
	assert.Nil(t, c.gotTTLReq)
}

func TestStackScheduleNew_TTL(t *testing.T) {
	t.Parallel()

	c := &mockScheduleNewClient{
		schedule: apitype.ScheduledAction{
			ID:           "ttl",
			ScheduleOnce: "2026-12-31T23:59:00Z",
			Kind:         apitype.ScheduledActionKindDeployment,
			Definition:   deploymentDefinitionJSON(t, apitype.Destroy),
		},
	}
	err := runStackScheduleNew(t.Context(), &bytes.Buffer{}, newClientFactory(c), stackScheduleNewArgs{
		kind:               scheduleKindTTL,
		once:               "2026-12-31T23:59:00Z",
		deleteAfterDestroy: true,
	}, renderScheduleGetText)
	require.NoError(t, err)

	require.NotNil(t, c.gotTTLReq)
	assert.Equal(t, "2026-12-31T23:59:00Z", c.gotTTLReq.Timestamp)
	assert.True(t, c.gotTTLReq.DeleteAfterDestroy)
	assert.True(t, c.createdViaTTL)
	assert.Nil(t, c.gotRawReq)
	assert.Nil(t, c.gotDriftReq)
}

func TestStackScheduleNew_ValidationErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args stackScheduleNewArgs
		want string
	}{
		{
			name: "raw missing operation",
			args: rawArgs("", "0 */4 * * *", ""),
			want: "--operation is required for --kind=raw",
		},
		{
			name: "raw invalid operation",
			args: rawArgs("detect-drift", "0 */4 * * *", ""),
			want: "invalid --operation",
		},
		{
			name: "drift missing cron",
			args: stackScheduleNewArgs{kind: scheduleKindDrift},
			want: "--cron is required for --kind=drift",
		},
		{
			name: "drift rejects once",
			args: stackScheduleNewArgs{
				kind: scheduleKindDrift, cron: "0 */4 * * *", once: "2026-12-31T23:59:00Z",
			},
			want: "--once is not valid for --kind=drift",
		},
		{
			name: "ttl missing once",
			args: stackScheduleNewArgs{kind: scheduleKindTTL},
			want: "--once is required for --kind=ttl",
		},
		{
			name: "ttl rejects cron",
			args: stackScheduleNewArgs{
				kind: scheduleKindTTL, cron: "0 */4 * * *", once: "2026-12-31T23:59:00Z",
			},
			want: "--cron is not valid for --kind=ttl",
		},
		{
			name: "raw rejects auto-remediate",
			args: stackScheduleNewArgs{
				kind: scheduleKindRaw, cron: "0 */4 * * *", operation: "update", autoRemediate: true,
			},
			want: "--auto-remediate is only valid for --kind=drift",
		},
		{
			name: "raw rejects delete-after-destroy",
			args: stackScheduleNewArgs{
				kind: scheduleKindRaw, cron: "0 */4 * * *", operation: "update", deleteAfterDestroy: true,
			},
			want: "--delete-after-destroy is only valid for --kind=ttl",
		},
		{
			name: "drift rejects operation",
			args: stackScheduleNewArgs{
				kind: scheduleKindDrift, cron: "0 */4 * * *", operation: "update",
			},
			want: "--operation is not valid for --kind=drift",
		},
		{
			name: "drift rejects delete-after-destroy",
			args: stackScheduleNewArgs{
				kind: scheduleKindDrift, cron: "0 */4 * * *", deleteAfterDestroy: true,
			},
			want: "--delete-after-destroy is only valid for --kind=ttl",
		},
		{
			name: "ttl rejects operation",
			args: stackScheduleNewArgs{
				kind: scheduleKindTTL, once: "2026-12-31T23:59:00Z", operation: "update",
			},
			want: "--operation is not valid for --kind=ttl",
		},
		{
			name: "ttl rejects auto-remediate",
			args: stackScheduleNewArgs{
				kind: scheduleKindTTL, once: "2026-12-31T23:59:00Z", autoRemediate: true,
			},
			want: "--auto-remediate is only valid for --kind=drift",
		},
		{
			name: "unknown kind",
			args: stackScheduleNewArgs{kind: "weekly"},
			want: "invalid --kind",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := runStackScheduleNew(
				t.Context(), &bytes.Buffer{}, newClientFactory(&mockScheduleNewClient{}), tc.args,
				renderScheduleGetText,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
