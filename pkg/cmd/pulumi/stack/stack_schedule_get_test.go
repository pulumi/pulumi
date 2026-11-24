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

type mockScheduleGetClient struct {
	schedule apitype.ScheduledAction
	err      error
}

func (m *mockScheduleGetClient) GetStackSchedule(
	_ context.Context, _ client.StackIdentifier, _ string,
) (apitype.ScheduledAction, error) {
	return m.schedule, m.err
}

func getClientFactory(c stackScheduleGetClient) stackScheduleGetClientFactory {
	return func(_ context.Context, _ string) (stackScheduleGetClient, client.StackIdentifier, error) {
		return c, testScheduleStackID, nil
	}
}

func sampleSchedule(t *testing.T) apitype.ScheduledAction {
	t.Helper()
	return apitype.ScheduledAction{
		ID:            "bb61b60a",
		OrgID:         "feacc792",
		ScheduleCron:  "12 16 * * *",
		NextExecution: "2026-05-13 16:12:00.000",
		Paused:        false,
		Kind:          apitype.ScheduledActionKindDeployment,
		Definition:    deploymentDefinitionJSON(t, apitype.DetectDrift),
		Created:       "2026-05-13 08:51:42.176",
		Modified:      "2026-05-13 09:47:37.982",
	}
}

func TestStackScheduleGet_TextOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleGetClient{schedule: sampleSchedule(t)}
	err := runStackScheduleGet(t.Context(), &buf, getClientFactory(c), "", "bb61b60a", renderScheduleGetText)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID:        bb61b60a")
	assert.Contains(t, out, "Type:      drift")
	assert.Contains(t, out, "Settings:  detect")
	assert.Contains(t, out, "Schedule:  12 16 * * *")
	assert.Contains(t, out, "Next run:  2026-05-13 16:12:00.000")
	assert.Contains(t, out, "Last run:  (never)")
	assert.Contains(t, out, "Created:   2026-05-13 08:51:42.176")
}

func TestStackScheduleGet_TextOutput_TTL(t *testing.T) {
	t.Parallel()

	lastExecuted := "2026-05-13 11:00:00.000"
	s := apitype.ScheduledAction{
		ID:           "abc-once",
		Kind:         apitype.ScheduledActionKindDeployment,
		ScheduleOnce: "2026-05-13 10:51:00.000",
		Definition: deploymentDefinitionWithOptionsJSON(t, apitype.Destroy,
			apitype.OperationContextOptions{DeleteAfterDestroy: true}),
		LastExecuted: &lastExecuted,
	}

	var buf bytes.Buffer
	c := &mockScheduleGetClient{schedule: s}
	err := runStackScheduleGet(t.Context(), &buf, getClientFactory(c), "", "abc-once", renderScheduleGetText)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Type:      ttl")
	assert.Contains(t, out, "Settings:  destroy + delete stack")
	assert.Contains(t, out, "Schedule:  Once")
	assert.Contains(t, out, "Last run:  2026-05-13 11:00:00.000")
}

func TestStackScheduleGet_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockScheduleGetClient{schedule: sampleSchedule(t)}
	err := runStackScheduleGet(t.Context(), &buf, getClientFactory(c), "", "bb61b60a", renderScheduleGetJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"id": "bb61b60a",
		"type": "drift",
		"settings": "detect",
		"schedule": "12 16 * * *",
		"nextRun": "2026-05-13 16:12:00.000",
		"created": "2026-05-13 08:51:42.176"
	}`, buf.String())
}
