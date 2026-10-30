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
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMassageSecretsRedactsContainers(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]property.Value{
		"map":           property.New(map[string]property.Value{"k": property.New("DUMMY")}),
		"array":         property.New([]property.Value{property.New("DUMMY")}),
		"string":        property.New("DUMMY"),
		"invalid UTF-8": property.New("DUMMY\xff"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			secret := value.WithSecret(true)
			input := property.NewMap(map[string]property.Value{
				"secret": secret,
				"nested": property.New([]property.Value{
					property.New(map[string]property.Value{"secret": secret}),
				}),
				"public": property.New("visible"),
			})
			expected := property.NewMap(map[string]property.Value{
				"secret": property.New("[secret]"),
				"nested": property.New([]property.Value{
					property.New(map[string]property.Value{"secret": property.New("[secret]")}),
				}),
				"public": property.New("visible"),
			})

			actual := MassageSecrets(input, false)
			assert.True(t, expected.Equals(actual))
			assert.False(t, actual.HasSecrets())

			shown := massagePropertyValue(secret, true)
			assert.True(t, massagePropertyValue(value, true).Equals(shown))
			assert.False(t, shown.HasSecrets())
		})
	}
}

func TestPreviewMetadataStepRedactsSecretMaps(t *testing.T) {
	t.Parallel()

	var apiEvent apitype.EngineEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"resourcePreEvent": {"metadata": {
			"op": "update",
			"urn": "urn:pulumi:dev::test::random:index/randomPet:RandomPet::pet",
			"old": {
				"type": "random:index/randomPet:RandomPet",
				"urn": "urn:pulumi:dev::test::random:index/randomPet:RandomPet::pet",
				"inputs": {"keepers": {
					"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
					"plaintext": "{\"k\":\"DUMMY-old\"}"
				}}
			},
			"new": {
				"type": "random:index/randomPet:RandomPet",
				"urn": "urn:pulumi:dev::test::random:index/randomPet:RandomPet::pet",
				"inputs": {"keepers": {
					"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
					"plaintext": "{\"k\":\"DUMMY-new\"}"
				}}
			}
		}}
	}`), &apiEvent))
	event, err := ConvertJSONEvent(apiEvent)
	require.NoError(t, err)
	metadata := event.Payload().(engine.ResourcePreEventPayload).Metadata
	for _, state := range []*engine.StepEventStateMetadata{metadata.Old, metadata.New} {
		require.NotNil(t, state)
		require.True(t, state.Inputs["keepers"].IsSecret())
		// JSON event replay does not populate the engine's internal resource state.
		state.State = pkgresource.NewState{ //nolint:requiredfield // Only properties and identity matter here.
			Type: state.Type, URN: state.URN,
			Inputs: state.Inputs, Outputs: state.Inputs,
		}.Make()
	}

	step := getPreviewMetadataStep(metadata, Options{})
	encoded, err := json.Marshal(step)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "DUMMY")
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	for _, key := range []string{"oldState", "newState"} {
		state := decoded[key].(map[string]any)
		for _, field := range []string{"inputs", "outputs"} {
			assert.Equal(t, map[string]any{"keepers": "[secret]"}, state[field])
		}
	}
}
