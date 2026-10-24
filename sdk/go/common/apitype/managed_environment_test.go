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

package apitype

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagedEnvironmentJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{
			name:  "stack",
			value: Stack{ManagedEnvironment: "proj/env", ManagedEnvironmentMissing: true},
			expected: `{"id":"","orgName":"","projectName":"","stackName":"","activeUpdate":"","version":0,` +
				`"managedEnvironment":"proj/env","managedEnvironmentMissing":true}`,
		},
		{
			name:     "unmanaged stack",
			value:    Stack{},
			expected: `{"id":"","orgName":"","projectName":"","stackName":"","activeUpdate":"","version":0}`,
		},
		{
			name:     "create stack request",
			value:    CreateStackRequest{StackName: "dev", ManagedEnvironment: true},
			expected: `{"stackName":"dev","managedEnvironment":true}`,
		},
		{
			name:     "unmanaged create stack request",
			value:    CreateStackRequest{StackName: "dev"},
			expected: `{"stackName":"dev"}`,
		},
		{
			name:     "create stack response",
			value:    CreateStackResponse{ManagedEnvironment: "proj/env"},
			expected: `{"managedEnvironment":"proj/env"}`,
		},
		{
			name:     "unmanaged create stack response",
			value:    CreateStackResponse{},
			expected: `{}`,
		},
		{
			name: "draft revision 0",
			value: UpdateProgramEnvironment{
				BaseRevision:    3,
				ChangeRequestID: "cr-1",
				DraftRevision:   0,
			},
			expected: `{"baseRevision":3,"changeRequestID":"cr-1","draftRevision":0}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := json.Marshal(tt.value)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(actual))
		})
	}
}

func TestUpdateProgramRequestEnvironmentJSON(t *testing.T) {
	t.Parallel()

	request := UpdateProgramRequest{
		Environment: &UpdateProgramEnvironment{
			BaseRevision:    3,
			ChangeRequestID: "cr-1",
			DraftRevision:   0,
		},
	}
	actual, err := json.Marshal(request)
	require.NoError(t, err)
	var roundTripped UpdateProgramRequest
	require.NoError(t, json.Unmarshal(actual, &roundTripped))
	assert.Equal(t, request, roundTripped)

	unmanaged, err := json.Marshal(UpdateProgramRequest{})
	require.NoError(t, err)
	var unmanagedFields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(unmanaged, &unmanagedFields))
	assert.NotContains(t, unmanagedFields, "environment")
}
