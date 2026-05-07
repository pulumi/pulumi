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

package display

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummaryJSONFromEvent(t *testing.T) {
	t.Parallel()

	payload := engine.SummaryEventPayload{
		Result:          apitype.OperationResultSucceeded,
		Duration:        7 * time.Second,
		ResourceChanges: display.ResourceChanges{"create": 2, "update": 1},
	}

	got := summaryJSONFromEvent(payload)

	assert.Equal(t, apitype.OperationResultSucceeded, got.Result)
	assert.Equal(t, 7*time.Second, got.Duration)
	assert.Equal(t, 2, got.Summary["create"])
	assert.Equal(t, 1, got.Summary["update"])
}

func TestWriteSummaryJSON(t *testing.T) {
	t.Parallel()

	s := SummaryJSON{
		Result:   apitype.OperationResultFailed,
		Duration: 3 * time.Second,
		Summary:  display.ResourceChanges{"delete": 1},
	}

	var buf bytes.Buffer
	require.NoError(t, writeSummaryJSON(&buf, s))

	// Output is a single line of JSON terminated by a newline.
	out := buf.String()
	assert.True(t, strings.HasSuffix(out, "\n"), "output should end with newline")
	assert.Equal(t, 1, strings.Count(out, "\n"), "output should be a single line")

	var roundTrip SummaryJSON
	require.NoError(t, json.Unmarshal([]byte(out), &roundTrip))
	assert.Equal(t, s, roundTrip)

	// The wire shape uses our preferred keys.
	assert.Contains(t, out, `"result":"failed"`)
	assert.Contains(t, out, `"summary":`)
	assert.NotContains(t, out, "changeSummary")
	assert.NotContains(t, out, "resourceChanges")
}

func TestTapSummaryJSON_EmitsOnSummaryEvent(t *testing.T) {
	t.Parallel()

	in := make(chan engine.Event, 2)
	in <- engine.NewEvent(engine.SummaryEventPayload{
		Result:          apitype.OperationResultSucceeded,
		Duration:        1 * time.Second,
		ResourceChanges: display.ResourceChanges{"same": 5},
	})
	close(in)

	var buf bytes.Buffer
	out := tapSummaryJSON(in, Options{Stdout: &buf})

	// Drain the output channel so the goroutine completes.
	for range out { //nolint:revive // intentional drain
	}

	var summary SummaryJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &summary))
	assert.Equal(t, apitype.OperationResultSucceeded, summary.Result)
	assert.Equal(t, 1*time.Second, summary.Duration)
	assert.Equal(t, 5, summary.Summary["same"])
}
