// Copyright 2016-2025, Pulumi Corporation.
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

package ui

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChangeSummaryJSON(t *testing.T) {
	t.Parallel()

	changes := display.ResourceChanges{
		deploy.OpCreate:  3,
		deploy.OpUpdate:  2,
		deploy.OpDelete:  1,
		deploy.OpSame:    10,
		deploy.OpReplace: 0,
	}

	summary := NewChangeSummaryJSON(changes)

	assert.Equal(t, 3, summary.Create)
	assert.Equal(t, 2, summary.Update)
	assert.Equal(t, 1, summary.Delete)
	assert.Equal(t, 10, summary.Same)
	assert.Equal(t, 0, summary.Replace)
}

func TestOperationSummaryJSON_Serialization(t *testing.T) {
	t.Parallel()

	summary := OperationSummaryJSON{
		Result:        "succeeded",
		Duration:      "10s",
		ResourceCount: 15,
		Changes: ChangeSummaryJSON{
			Create: 2,
			Update: 1,
			Delete: 0,
			Same:   12,
		},
		Outputs: map[string]any{
			"url": "https://example.com",
		},
	}

	jsonBytes, err := json.Marshal(summary)
	require.NoError(t, err)

	var decoded OperationSummaryJSON
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "succeeded", decoded.Result)
	assert.Equal(t, 2, decoded.Changes.Create)
	assert.Equal(t, "https://example.com", decoded.Outputs["url"])
}

func TestOperationSummaryJSON_OmitsEmptyOutputs(t *testing.T) {
	t.Parallel()

	summary := OperationSummaryJSON{
		Result:   "succeeded",
		Duration: "5s",
	}

	jsonBytes, err := json.Marshal(summary)
	require.NoError(t, err)

	assert.NotContains(t, string(jsonBytes), "outputs")
}
