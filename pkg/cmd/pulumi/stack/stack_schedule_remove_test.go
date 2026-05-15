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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockScheduleRemoveClient struct {
	err           error
	gotScheduleID string
}

func (m *mockScheduleRemoveClient) DeleteStackSchedule(
	_ context.Context, _ client.StackIdentifier, scheduleID string,
) error {
	m.gotScheduleID = scheduleID
	return m.err
}

func removeClientFactory(c stackScheduleRemoveClient) stackScheduleRemoveClientFactory {
	return func(_ context.Context, _ string) (stackScheduleRemoveClient, client.StackIdentifier, error) {
		return c, testScheduleStackID, nil
	}
}

func TestStackScheduleRemove_Success(t *testing.T) {
	t.Parallel()

	c := &mockScheduleRemoveClient{}
	var buf bytes.Buffer
	err := runStackScheduleRemove(
		t.Context(), &buf, removeClientFactory(c),
		"", "bb61b60a-a313-46cb-b4ab-9d42dce46de8", true, // yes=true skips confirmation
	)
	require.NoError(t, err)

	assert.Equal(t, "bb61b60a-a313-46cb-b4ab-9d42dce46de8", c.gotScheduleID)
	assert.Contains(t, buf.String(),
		"Schedule 'bb61b60a-a313-46cb-b4ab-9d42dce46de8' has been removed.")
}

func TestStackScheduleRemove_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockScheduleRemoveClient{err: errors.New("not found")}
	var buf bytes.Buffer
	err := runStackScheduleRemove(
		t.Context(), &buf, removeClientFactory(c),
		"", "no-such-schedule", true,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing stack schedule")
	assert.Contains(t, err.Error(), "not found")
}
