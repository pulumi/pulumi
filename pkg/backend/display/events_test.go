// Copyright 2016-2023, Pulumi Corporation.
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

// Note: to regenerate the baselines for these tests, run `go test` with `PULUMI_ACCEPT=true`.

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/stretchr/testify/assert"
)

// This test checks that the ANSI control codes are removed from EngineEvents
// converted to be sent to the Pulumi Service API.
func TestRemoveANSI(t *testing.T) {
	t.Parallel()
	input := "\033[31mHello, World!\033[0m"
	expected := "Hello, World!"
	e := engine.NewEvent(
		engine.DiagEventPayload{
			Message: input,
		},
	)

	res, err := ConvertEngineEvent(e, false /* showSecrets */)
	assert.NoError(t, err, "unable to convert engine event")
	assert.Equal(t, expected, res.DiagnosticEvent.Message)
}

func TestEmptyDetailedDiff(t *testing.T) {
	t.Parallel()
	expected := `{"sequence":0,"timestamp":0,"resOutputsEvent":{"metadata":{"op":"import","urn":"urn:pul:resource:type::name","type":"urn:pul:resource:type","old":null,"new":null,"detailedDiff":{},"provider":""}}}` //nolint:lll
	e := engine.NewEvent(
		engine.ResourceOutputsEventPayload{
			Metadata: engine.StepEventMetadata{
				Op:           deploy.OpImport,
				URN:          "urn:pul:resource:type::name",
				Type:         "urn:pul:resource:type",
				DetailedDiff: map[string]plugin.PropertyDiff{},
			},
		},
	)
	res, err := ConvertEngineEvent(e, false /* showSecrets */)
	assert.NoError(t, err, "unable to convert engine event")
	jsonEvent, err := json.Marshal(res)
	assert.NoError(t, err, "unable to marshal to json")
	assert.Equal(t, expected, string(jsonEvent))
}
