// Copyright 2016-2018, Pulumi Corporation.
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
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// TestDeploymentSerialization creates a basic snapshot of a given resource state.
func TestDeploymentSerialization(t *testing.T) {
	res := resource.NewState(
		tokens.Type("Test"),
		resource.NewURN(
			tokens.QName("test"),
			tokens.PackageName("resource/test"),
			tokens.Type(""),
			tokens.Type("Test"),
			tokens.QName("resource-x"),
		),
		true,
		false,
		resource.ID("test-resource-x"),
		resource.NewPropertyMapFromMap(map[string]interface{}{
			"in-nil":         nil,
			"in-bool":        true,
			"in-float64":     float64(1.5),
			"in-string":      "lumilumilo",
			"in-array":       []interface{}{"a", true, float64(32)},
			"in-empty-array": []interface{}{},
			"in-map": map[string]interface{}{
				"a": true,
				"b": float64(88),
				"c": "c-see-saw",
				"d": "d-dee-daw",
			},
			"in-empty-map": map[string]interface{}{},
		}),
		resource.NewPropertyMapFromMap(map[string]interface{}{
			"out-nil":         nil,
			"out-bool":        false,
			"out-float64":     float64(76),
			"out-string":      "loyolumiloom",
			"out-array":       []interface{}{false, "zzxx"},
			"out-empty-array": []interface{}{},
			"out-map": map[string]interface{}{
				"x": false,
				"y": "z-zee-zaw",
				"z": float64(999.9),
			},
			"out-empty-map": map[string]interface{}{},
		}),
		"",
		false,
		false,
		[]resource.URN{
			resource.URN("foo:bar:baz"),
			resource.URN("foo:bar:boo"),
		},
		[]string{},
		"",
		nil,
		false,
		nil,
		nil,
		nil,
	)

	dep, err := SerializeResource(res, config.NopEncrypter)
	assert.NoError(t, err)

	// assert some things about the deployment record:
	assert.NotNil(t, dep)
	assert.NotNil(t, dep.ID)
	assert.Equal(t, resource.ID("test-resource-x"), dep.ID)
	assert.Equal(t, tokens.Type("Test"), dep.Type)
	assert.Equal(t, 2, len(dep.Dependencies))
	assert.Equal(t, resource.URN("foo:bar:baz"), dep.Dependencies[0])
	assert.Equal(t, resource.URN("foo:bar:boo"), dep.Dependencies[1])

	// assert some things about the inputs:
	assert.NotNil(t, dep.Inputs)
	assert.Nil(t, dep.Inputs["in-nil"])
	assert.NotNil(t, dep.Inputs["in-bool"])
	assert.True(t, dep.Inputs["in-bool"].(bool))
	assert.NotNil(t, dep.Inputs["in-float64"])
	assert.Equal(t, float64(1.5), dep.Inputs["in-float64"].(float64))
	assert.NotNil(t, dep.Inputs["in-string"])
	assert.Equal(t, "lumilumilo", dep.Inputs["in-string"].(string))
	assert.NotNil(t, dep.Inputs["in-array"])
	assert.Equal(t, 3, len(dep.Inputs["in-array"].([]interface{})))
	assert.Equal(t, "a", dep.Inputs["in-array"].([]interface{})[0])
	assert.Equal(t, true, dep.Inputs["in-array"].([]interface{})[1])
	assert.Equal(t, float64(32), dep.Inputs["in-array"].([]interface{})[2])
	assert.NotNil(t, dep.Inputs["in-empty-array"])
	assert.Equal(t, 0, len(dep.Inputs["in-empty-array"].([]interface{})))
	assert.NotNil(t, dep.Inputs["in-map"])
	inmap := dep.Inputs["in-map"].(map[string]interface{})
	assert.Equal(t, 4, len(inmap))
	assert.NotNil(t, inmap["a"])
	assert.Equal(t, true, inmap["a"].(bool))
	assert.NotNil(t, inmap["b"])
	assert.Equal(t, float64(88), inmap["b"].(float64))
	assert.NotNil(t, inmap["c"])
	assert.Equal(t, "c-see-saw", inmap["c"].(string))
	assert.NotNil(t, inmap["d"])
	assert.Equal(t, "d-dee-daw", inmap["d"].(string))
	assert.NotNil(t, dep.Inputs["in-empty-map"])
	assert.Equal(t, 0, len(dep.Inputs["in-empty-map"].(map[string]interface{})))

	// assert some things about the outputs:
	assert.NotNil(t, dep.Outputs)
	assert.Nil(t, dep.Outputs["out-nil"])
	assert.NotNil(t, dep.Outputs["out-bool"])
	assert.False(t, dep.Outputs["out-bool"].(bool))
	assert.NotNil(t, dep.Outputs["out-float64"])
	assert.Equal(t, float64(76), dep.Outputs["out-float64"].(float64))
	assert.NotNil(t, dep.Outputs["out-string"])
	assert.Equal(t, "loyolumiloom", dep.Outputs["out-string"].(string))
	assert.NotNil(t, dep.Outputs["out-array"])
	assert.Equal(t, 2, len(dep.Outputs["out-array"].([]interface{})))
	assert.Equal(t, false, dep.Outputs["out-array"].([]interface{})[0])
	assert.Equal(t, "zzxx", dep.Outputs["out-array"].([]interface{})[1])
	assert.NotNil(t, dep.Outputs["out-empty-array"])
	assert.Equal(t, 0, len(dep.Outputs["out-empty-array"].([]interface{})))
	assert.NotNil(t, dep.Outputs["out-map"])
	outmap := dep.Outputs["out-map"].(map[string]interface{})
	assert.Equal(t, 3, len(outmap))
	assert.NotNil(t, outmap["x"])
	assert.Equal(t, false, outmap["x"].(bool))
	assert.NotNil(t, outmap["y"])
	assert.Equal(t, "z-zee-zaw", outmap["y"].(string))
	assert.NotNil(t, outmap["z"])
	assert.Equal(t, float64(999.9), outmap["z"].(float64))
	assert.NotNil(t, dep.Outputs["out-empty-map"])
	assert.Equal(t, 0, len(dep.Outputs["out-empty-map"].(map[string]interface{})))
}

func TestLoadTooNewDeployment(t *testing.T) {
	untypedDeployment := &apitype.UntypedDeployment{
		Version: apitype.DeploymentSchemaVersionCurrent + 1,
	}

	deployment, err := DeserializeUntypedDeployment(untypedDeployment, DefaultSecretsProvider)
	assert.Nil(t, deployment)
	assert.Error(t, err)
	assert.Equal(t, ErrDeploymentSchemaVersionTooNew, err)
}

func TestLoadTooOldDeployment(t *testing.T) {
	untypedDeployment := &apitype.UntypedDeployment{
		Version: DeploymentSchemaVersionOldestSupported - 1,
	}

	deployment, err := DeserializeUntypedDeployment(untypedDeployment, DefaultSecretsProvider)
	assert.Nil(t, deployment)
	assert.Error(t, err)
	assert.Equal(t, ErrDeploymentSchemaVersionTooOld, err)
}

func TestUnsupportedSecret(t *testing.T) {
	rawProp := map[string]interface{}{
		resource.SigKey: resource.SecretSig,
	}
	_, err := DeserializePropertyValue(rawProp, config.NewPanicCrypter())
	assert.Error(t, err)
}

func TestUnknownSig(t *testing.T) {
	rawProp := map[string]interface{}{
		resource.SigKey: "foobar",
	}
	_, err := DeserializePropertyValue(rawProp, config.NewPanicCrypter())
	assert.Error(t, err)
}

func TestCustomSerialization(t *testing.T) {
	textAsset, err := resource.NewTextAsset("alpha beta gamma")
	assert.NoError(t, err)

	strProp := resource.NewStringProperty("strProp")

	computed := resource.Computed{Element: strProp}
	output := resource.Output{Element: strProp}

	secretElement := resource.Secret{Element: strProp}
	secret := &resource.Secret{Element: strProp}

	propMap := resource.NewPropertyMapFromMap(map[string]interface{}{
		// Primitive types
		"nil":     nil,
		"bool":    true,
		"int32":   int64(41),
		"int64":   int64(42),
		"float32": float32(2.5),
		"float64": float64(1.5),
		"string":  "string literal",

		// Data structures
		"array":       []interface{}{"a", true, float64(32)},
		"array-empty": []interface{}{},

		"map": map[string]interface{}{
			"a": true,
			"b": float64(88),
			"c": "c-see-saw",
			"d": "d-dee-daw",
		},
		"map-empty": map[string]interface{}{},

		// Specialized resource types
		"asset-text": textAsset,

		"computed": computed,
		"output":   output,
		"secret":   secret,
	})

	// BUG? Shouldn't this be true because of the "secret" field?
	assert.False(t, propMap.ContainsSecrets())
	// BUG? How would this ever be true? Is there a special way to
	// mark computed/output as being unknown?
	assert.False(t, propMap.ContainsUnknowns())

	// Confirm the expected shape of serializing a ResourceProperty and PropertyMap using the
	// reflection-based default JSON encoder. This should NOT be used when serializing resources,
	// but we confirm the expected shape here while we migrate older code that relied on the
	// specific format.
	t.Run("SerializeToJSON", func(t *testing.T) {
		b, err := json.Marshal(propMap)
		if err != nil {
			t.Fatalf("Marshalling PropertyMap: %v", err)
		}
		json := string(b)

		// Look for the specific JSON serialization of the properties.
		tests := []string{
			`"bool":{"V":true}`,
			`"string":{"V":"string literal"}}`,

			`array":{"V":[{"V":"a"},{"V":true},{"V":32}]}`,
			`"map-empty":{"V":{}}`,

			// Assets
			`"asset-text":{"V":{"4dabf18193072939515e22adb298388d":"c44067f5952c0a294b673a41bacd8c17","hash":"64989ccbf3efa9c84e2afe7cee9bc5828bf0fcb91e44f8c1e591638a2c2e90e3","text":"alpha beta gamma"}}`,

			// Wrapped properties
			// BUG? Why isn't the nested ResourceProperty serialized here?
			// Shouldn't this be "output": { "V": { "strProp" } }
			`"computed":{"V":{}}`,
			`"output":{"V":{}}`,
			`"secret":{"V":{}}`,
		}

		for _, want := range tests {
			if !strings.Contains(json, want) {
				t.Errorf("Did not find expected snippet: %v", want)
			}
		}

		if t.Failed() {
			t.Logf("Full JSON encoding:\n%v", json)
		}
	})

	// Using stack.SerializeProperties will get the correct behavior and should be used
	// whenever persisting resources into some durable form.
	t.Run("SerializeProperties", func(t *testing.T) {
		serializedPropMap, err := SerializeProperties(propMap, config.BlindingCrypter)
		assert.NoError(t, err)

		// Now JSON encode the results?
		b, err := json.Marshal(serializedPropMap)
		if err != nil {
			t.Fatalf("Marshalling PropertyMap: %v", err)
		}
		json := string(b)

		// Look for the specific JSON serialization of the properties.
		tests := []string{
			`"bool":true`,
			`"string":"string literal"}`,

			`"array":["a",true,32]`,
			`"map":{"a":true,"b":88,"c":"c-see-saw","d":"d-dee-daw"}`,
			`"map-empty":{}`,

			// Assets
			`"asset-text":{"4dabf18193072939515e22adb298388d":"c44067f5952c0a294b673a41bacd8c17","hash":"64989ccbf3efa9c84e2afe7cee9bc5828bf0fcb91e44f8c1e591638a2c2e90e3","text":"alpha beta gamma"}`,

			// Wrapped properties
			// BUG? Why isn't the nested ResourceProperty serialized here?
			// Shouldn't this be "output": "strProp"
			`"computed":{}`,
			`"output":{}`,
			`"secret":{}`,
		}

		for _, want := range tests {
			if !strings.Contains(json, want) {
				t.Errorf("Did not find expected snippet: %v", want)
			}
		}

		if t.Failed() {
			t.Logf("Full JSON encoding:\n%v", json)
		}
	})
}
