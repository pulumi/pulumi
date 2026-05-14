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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockScheduleClient struct {
	schedules []apitype.ScheduledAction
	err       error
}

func (m *mockScheduleClient) ListStackSchedules(
	_ context.Context, _ client.StackIdentifier,
) ([]apitype.ScheduledAction, error) {
	return m.schedules, m.err
}

var testScheduleStackID = client.StackIdentifier{
	Owner:   "my-org",
	Project: "my-project",
	Stack:   tokens.MustParseStackName("dev"),
}

func clientFactory(c stackScheduleListClient) stackScheduleListClientFactory {
	return func(_ context.Context, _ string) (stackScheduleListClient, client.StackIdentifier, error) {
		return c, testScheduleStackID, nil
	}
}

func deploymentDefinitionJSON(t *testing.T, op apitype.PulumiOperation) json.RawMessage {
	t.Helper()
	def := apitype.ScheduledDeploymentDefinition{
		ProgramID: "5f337707-1981-48b7-a795-1ba559068db2",
		Request: &apitype.CreateDeploymentRequest{
			Op:              op,
			InheritSettings: true,
		},
	}
	b, err := json.Marshal(def)
	require.NoError(t, err)
	return b
}

func deploymentDefinitionWithOptionsJSON(
	t *testing.T, op apitype.PulumiOperation, opts apitype.OperationContextOptions,
) json.RawMessage {
	t.Helper()
	def := apitype.ScheduledDeploymentDefinition{
		ProgramID: "5f337707-1981-48b7-a795-1ba559068db2",
		Request: &apitype.CreateDeploymentRequest{
			Op:              op,
			InheritSettings: true,
			Operation:       &apitype.OperationContext{Options: &opts},
		},
	}
	b, err := json.Marshal(def)
	require.NoError(t, err)
	return b
}

func sampleSchedules(t *testing.T) []apitype.ScheduledAction {
	t.Helper()
	lastExecuted := "2026-05-13 09:00:00.000"
	return []apitype.ScheduledAction{
		{
			// TTL: destroy with operationContext set (deleteAfterDestroy=false).
			ID:            "bb61b60a",
			OrgID:         "feacc792",
			ScheduleOnce:  "2026-05-13 10:51:00.000",
			NextExecution: "2026-05-13 10:51:00.000",
			Paused:        false,
			Kind:          apitype.ScheduledActionKindDeployment,
			Definition: deploymentDefinitionWithOptionsJSON(t, apitype.Destroy,
				apitype.OperationContextOptions{DeleteAfterDestroy: false}),
			Created:      "2026-05-13 08:51:42.176",
			Modified:     "2026-05-13 08:51:42.176",
			LastExecuted: &lastExecuted,
		},
		{
			// Raw: refresh, no operationContext.
			ID:            "abc-cron",
			OrgID:         "feacc792",
			ScheduleCron:  "0 */4 * * *",
			NextExecution: "2026-05-13 12:00:00.000",
			Paused:        true,
			Kind:          apitype.ScheduledActionKindDeployment,
			Definition:    deploymentDefinitionJSON(t, apitype.Refresh),
			Created:       "2026-05-13 09:00:00.000",
		},
	}
}

func TestStackScheduleList_TableOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleClient{schedules: sampleSchedules(t)}
	err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", 0, renderScheduleListTable)
	require.NoError(t, err)

	out := buf.String()

	// Headers
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "TYPE")
	assert.Contains(t, out, "SETTINGS")
	assert.Contains(t, out, "SCHEDULE")
	assert.Contains(t, out, "NEXT RUN")
	assert.Contains(t, out, "LAST RUN")
	assert.Contains(t, out, "CREATED")

	// First schedule: one-shot destroy → type=ttl, schedule="Once", settings="destroy".
	assert.Contains(t, out, "bb61b60a")
	assert.Contains(t, out, "ttl")
	assert.Contains(t, out, "destroy")
	assert.Contains(t, out, "Once")
	assert.Contains(t, out, "2026-05-13 10:51:00.000") // next run
	assert.Contains(t, out, "2026-05-13 09:00:00.000") // last run
	assert.Contains(t, out, "2026-05-13 08:51:42.176") // created

	// Second schedule: refresh on cron → type=raw, settings="pulumi refresh".
	assert.Contains(t, out, "abc-cron")
	assert.Contains(t, out, "raw")
	assert.Contains(t, out, "pulumi refresh")
	assert.Contains(t, out, "0 */4 * * *")
	assert.Contains(t, out, "2026-05-13 12:00:00.000") // next run
}

func TestStackScheduleList_TableOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleClient{schedules: []apitype.ScheduledAction{}}
	err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", 0, renderScheduleListTable)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "No scheduled actions configured for this stack.")
}

func TestStackScheduleList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleClient{schedules: sampleSchedules(t)}
	err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", 0, renderScheduleListJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"schedules": [
			{
				"id": "bb61b60a",
				"type": "ttl",
				"settings": "destroy",
				"schedule": "Once",
				"nextRun": "2026-05-13 10:51:00.000",
				"lastRun": "2026-05-13 09:00:00.000",
				"created": "2026-05-13 08:51:42.176"
			},
			{
				"id": "abc-cron",
				"type": "raw",
				"settings": "pulumi refresh",
				"schedule": "0 */4 * * *",
				"nextRun": "2026-05-13 12:00:00.000",
				"created": "2026-05-13 09:00:00.000"
			}
		]
	}`, buf.String())
}

func TestStackScheduleList_JSONOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleClient{schedules: []apitype.ScheduledAction{}}
	err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", 0, renderScheduleListJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{"schedules": []}`, buf.String())
}

func TestStackScheduleList_Count(t *testing.T) {
	t.Parallel()

	t.Run("limits to N", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		c := &mockScheduleClient{schedules: sampleSchedules(t)}
		err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", 1, renderScheduleListTable)
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "bb61b60a")
		assert.NotContains(t, out, "abc-cron")
	})

	t.Run("count larger than total", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		c := &mockScheduleClient{schedules: sampleSchedules(t)}
		err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", 99, renderScheduleListTable)
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "bb61b60a")
		assert.Contains(t, out, "abc-cron")
	})

	t.Run("json truncated", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		c := &mockScheduleClient{schedules: sampleSchedules(t)}
		err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", 1, renderScheduleListJSON)
		require.NoError(t, err)
		var got struct {
			Schedules []scheduleSummary `json:"schedules"`
		}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
		assert.Len(t, got.Schedules, 1)
		assert.Equal(t, "bb61b60a", got.Schedules[0].ID)
	})
}
