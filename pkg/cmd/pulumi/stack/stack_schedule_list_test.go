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

// mockScheduleClient implements stackScheduleListClient for tests.
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

func sampleSchedules(t *testing.T) []apitype.ScheduledAction {
	t.Helper()
	return []apitype.ScheduledAction{
		{
			ID:            "bb61b60a",
			OrgID:         "feacc792",
			ScheduleOnce:  "2026-05-13 10:51:00.000",
			NextExecution: "2026-05-13 10:51:00.000",
			Paused:        false,
			Kind:          apitype.ScheduledActionKindDeployment,
			Definition:    deploymentDefinitionJSON(t, apitype.Destroy),
			Created:       "2026-05-13 08:51:42.176",
			Modified:      "2026-05-13 08:51:42.176",
		},
		{
			ID:            "abc-cron",
			OrgID:         "feacc792",
			ScheduleCron:  "0 */4 * * *",
			NextExecution: "2026-05-13 12:00:00.000",
			Paused:        true,
			Kind:          apitype.ScheduledActionKindDeployment,
			Definition:    deploymentDefinitionJSON(t, apitype.Refresh),
		},
	}
}

func TestStackScheduleList_TableOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleClient{schedules: sampleSchedules(t)}
	err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", "default")
	require.NoError(t, err)

	out := buf.String()

	// Headers
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "KIND")
	assert.Contains(t, out, "SCHEDULE")
	assert.Contains(t, out, "OPERATION")
	assert.Contains(t, out, "NEXT EXECUTION")
	assert.Contains(t, out, "PAUSED")

	// First schedule
	assert.Contains(t, out, "bb61b60a")
	assert.Contains(t, out, "deployment")
	assert.Contains(t, out, "2026-05-13 10:51:00.000")
	assert.Contains(t, out, "destroy")
	assert.Contains(t, out, "false")

	// Second schedule
	assert.Contains(t, out, "abc-cron")
	assert.Contains(t, out, "0 */4 * * *")
	assert.Contains(t, out, "refresh")
	assert.Contains(t, out, "true")
}

func TestStackScheduleList_TableOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleClient{schedules: []apitype.ScheduledAction{}}
	err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", "default")
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "No scheduled actions configured for this stack.")
}

func TestStackScheduleList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleClient{schedules: sampleSchedules(t)}
	err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", "json")
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"schedules": [
			{
				"id": "bb61b60a",
				"orgID": "feacc792",
				"scheduleOnce": "2026-05-13 10:51:00.000",
				"nextExecution": "2026-05-13 10:51:00.000",
				"paused": false,
				"kind": "deployment",
				"definition": {
					"programID": "5f337707-1981-48b7-a795-1ba559068db2",
					"request": {
						"operation": "destroy",
						"inheritSettings": true
					}
				},
				"created": "2026-05-13 08:51:42.176",
				"modified": "2026-05-13 08:51:42.176"
			},
			{
				"id": "abc-cron",
				"orgID": "feacc792",
				"scheduleCron": "0 */4 * * *",
				"nextExecution": "2026-05-13 12:00:00.000",
				"paused": true,
				"kind": "deployment",
				"definition": {
					"programID": "5f337707-1981-48b7-a795-1ba559068db2",
					"request": {
						"operation": "refresh",
						"inheritSettings": true
					}
				}
			}
		]
	}`, buf.String())
}

func TestStackScheduleList_JSONOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleClient{schedules: []apitype.ScheduledAction{}}
	err := runStackScheduleList(t.Context(), &buf, clientFactory(c), "", "json")
	require.NoError(t, err)

	assert.JSONEq(t, `{"schedules": []}`, buf.String())
}
