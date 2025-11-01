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

//nolint:lll
package stack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	combinations "github.com/mxschmitt/golang-combinations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	rasset "github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	resource_testing "github.com/pulumi/pulumi/sdk/v3/go/common/resource/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// TestDeploymentSerialization creates a basic snapshot of a given resource state.
func TestDeploymentSerialization(t *testing.T) {
	t.Parallel()
	res := resource.NewState{
		Type: tokens.Type("Test"),
		URN: resource.NewURN(
			tokens.QName("test"),
			tokens.PackageName("resource/test"),
			tokens.Type(""),
			tokens.Type("Test"),
			"resource-x",
		),
		Custom: true,
		Delete: false,
		ID:     resource.ID("test-resource-x"),
		Inputs: resource.NewPropertyMapFromMap(map[string]any{
			"in-nil":         nil,
			"in-bool":        true,
			"in-float64":     float64(1.5),
			"in-string":      "lumilumilo",
			"in-array":       []any{"a", true, float64(32)},
			"in-empty-array": []any{},
			"in-map": map[string]any{
				"a": true,
				"b": float64(88),
				"c": "c-see-saw",
				"d": "d-dee-daw",
			},
			"in-empty-map":                            map[string]any{},
			"in-component-resource-reference":         resource.MakeComponentResourceReference("urn", "1.2.3").V,
			"in-custom-resource-reference":            resource.MakeCustomResourceReference("urn2", "id", "2.3.4").V,
			"in-custom-resource-reference-unknown-id": resource.MakeCustomResourceReference("urn3", "", "3.4.5").V,
		}),
		Outputs: resource.NewPropertyMapFromMap(map[string]any{
			"out-nil":         nil,
			"out-bool":        false,
			"out-float64":     float64(76),
			"out-string":      "loyolumiloom",
			"out-array":       []any{false, "zzxx"},
			"out-empty-array": []any{},
			"out-map": map[string]any{
				"x": false,
				"y": "z-zee-zaw",
				"z": float64(999.9),
			},
			"out-empty-map": map[string]any{},
		}),
		Parent:   "",
		Protect:  false,
		Taint:    false,
		External: false,
		Dependencies: []resource.URN{
			resource.URN("foo:bar:baz"),
			resource.URN("foo:bar:boo"),
		},
		InitErrors:              []string{},
		Provider:                "",
		PropertyDependencies:    nil,
		PendingReplacement:      false,
		AdditionalSecretOutputs: nil,
		Aliases:                 nil,
		CustomTimeouts:          nil,
		ImportID:                "",
		RetainOnDelete:          false,
		DeletedWith:             "",
		Created:                 time.Now().UTC(),
		Modified:                time.Now().UTC(),
		SourcePosition:          "",
		HideDiff:                nil,
		StackTrace:              nil,
		IgnoreChanges:           nil,
		ReplaceOnChanges:        nil,
		RefreshBeforeUpdate:     false,
		ViewOf:                  "",
		ResourceHooks: map[resource.HookType][]string{
			resource.BeforeCreate: {"hook1"},
			resource.AfterDelete:  {"hook2"},
		},
	}.Make()
	dep, err := SerializeResource(context.Background(), res, config.NopEncrypter, false /* showSecrets */)
	require.NoError(t, err)

	// assert some things about the deployment record:
	require.NotNil(t, dep)
	require.NotNil(t, dep.ID)
	assert.Equal(t, resource.ID("test-resource-x"), dep.ID)
	assert.Equal(t, tokens.Type("Test"), dep.Type)
	require.Len(t, dep.Dependencies, 2)
	assert.Equal(t, resource.URN("foo:bar:baz"), dep.Dependencies[0])
	assert.Equal(t, resource.URN("foo:bar:boo"), dep.Dependencies[1])
	assert.Equal(t, map[resource.HookType][]string{
		"BeforeCreate": {"hook1"},
		"AfterDelete":  {"hook2"},
	}, dep.ResourceHooks)

	// assert some things about the inputs:
	require.NotNil(t, dep.Inputs)
	assert.Nil(t, dep.Inputs["in-nil"])
	require.NotNil(t, dep.Inputs["in-bool"])
	assert.True(t, dep.Inputs["in-bool"].(bool))
	require.NotNil(t, dep.Inputs["in-float64"])
	assert.Equal(t, float64(1.5), dep.Inputs["in-float64"].(float64))
	require.NotNil(t, dep.Inputs["in-string"])
	assert.Equal(t, "lumilumilo", dep.Inputs["in-string"].(string))
	require.NotNil(t, dep.Inputs["in-array"])
	require.Len(t, dep.Inputs["in-array"].([]any), 3)
	assert.Equal(t, "a", dep.Inputs["in-array"].([]any)[0])
	assert.Equal(t, true, dep.Inputs["in-array"].([]any)[1])
	assert.Equal(t, float64(32), dep.Inputs["in-array"].([]any)[2])
	require.NotNil(t, dep.Inputs["in-empty-array"])
	assert.Empty(t, dep.Inputs["in-empty-array"].([]any))
	require.NotNil(t, dep.Inputs["in-map"])
	inmap := dep.Inputs["in-map"].(map[string]any)
	require.Len(t, inmap, 4)
	require.NotNil(t, inmap["a"])
	assert.Equal(t, true, inmap["a"].(bool))
	require.NotNil(t, inmap["b"])
	assert.Equal(t, float64(88), inmap["b"].(float64))
	require.NotNil(t, inmap["c"])
	assert.Equal(t, "c-see-saw", inmap["c"].(string))
	require.NotNil(t, inmap["d"])
	assert.Equal(t, "d-dee-daw", inmap["d"].(string))
	require.NotNil(t, dep.Inputs["in-empty-map"])
	assert.Empty(t, dep.Inputs["in-empty-map"].(map[string]any))
	assert.Equal(t, map[string]any{
		resource.SigKey:  resource.ResourceReferenceSig,
		"urn":            "urn",
		"packageVersion": "1.2.3",
	}, dep.Inputs["in-component-resource-reference"])
	assert.Equal(t, map[string]any{
		resource.SigKey:  resource.ResourceReferenceSig,
		"urn":            "urn2",
		"id":             "id",
		"packageVersion": "2.3.4",
	}, dep.Inputs["in-custom-resource-reference"])
	assert.Equal(t, map[string]any{
		resource.SigKey:  resource.ResourceReferenceSig,
		"urn":            "urn3",
		"id":             "",
		"packageVersion": "3.4.5",
	}, dep.Inputs["in-custom-resource-reference-unknown-id"])

	// assert some things about the outputs:
	require.NotNil(t, dep.Outputs)
	assert.Nil(t, dep.Outputs["out-nil"])
	require.NotNil(t, dep.Outputs["out-bool"])
	assert.False(t, dep.Outputs["out-bool"].(bool))
	require.NotNil(t, dep.Outputs["out-float64"])
	assert.Equal(t, float64(76), dep.Outputs["out-float64"].(float64))
	require.NotNil(t, dep.Outputs["out-string"])
	assert.Equal(t, "loyolumiloom", dep.Outputs["out-string"].(string))
	require.NotNil(t, dep.Outputs["out-array"])
	require.Len(t, dep.Outputs["out-array"].([]any), 2)
	assert.Equal(t, false, dep.Outputs["out-array"].([]any)[0])
	assert.Equal(t, "zzxx", dep.Outputs["out-array"].([]any)[1])
	require.NotNil(t, dep.Outputs["out-empty-array"])
	assert.Empty(t, dep.Outputs["out-empty-array"].([]any))
	require.NotNil(t, dep.Outputs["out-map"])
	outmap := dep.Outputs["out-map"].(map[string]any)
	require.Len(t, outmap, 3)
	require.NotNil(t, outmap["x"])
	assert.Equal(t, false, outmap["x"].(bool))
	require.NotNil(t, outmap["y"])
	assert.Equal(t, "z-zee-zaw", outmap["y"].(string))
	require.NotNil(t, outmap["z"])
	assert.Equal(t, float64(999.9), outmap["z"].(float64))
	require.NotNil(t, dep.Outputs["out-empty-map"])
	assert.Empty(t, dep.Outputs["out-empty-map"].(map[string]any))
}

// TestSerializeDeploymentWithMetadata tests that the appropriate version and features are used when
// serializing a deployment.
func TestSerializeDeploymentWithMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name             string
		resources        []*resource.State
		expectedVersion  int
		expectedFeatures []string
	}{
		{
			name: "v3 deployment with no features",
			resources: []*resource.State{
				{
					URN: "urn1",
				},
			},
			expectedVersion:  3,
			expectedFeatures: nil,
		},
		{
			name: "v4 deployment with refreshBeforeUpdate",
			resources: []*resource.State{
				{
					URN:                 "urn1",
					RefreshBeforeUpdate: true,
				},
			},
			expectedVersion:  4,
			expectedFeatures: []string{"refreshBeforeUpdate"},
		},
		{
			name: "v4 deployment with views",
			resources: []*resource.State{
				{
					URN: "urn1",
				},
				{
					URN:    "urn2",
					Parent: "urn1",
					ViewOf: "urn1",
				},
			},
			expectedVersion:  4,
			expectedFeatures: []string{"views"},
		},
		{
			name: "v4 deployment with hooks",
			resources: []*resource.State{
				{
					URN: "urn1",
					ResourceHooks: map[resource.HookType][]string{
						resource.AfterCreate: {"hook1"},
					},
				},
			},
			expectedVersion:  4,
			expectedFeatures: []string{"hooks"},
		},
		{
			name: "v4 deployment with taint",
			resources: []*resource.State{
				{
					URN:   "urn1",
					Taint: true,
				},
			},
			expectedVersion:  4,
			expectedFeatures: []string{"taint"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			snap := &deploy.Snapshot{
				Resources: tt.resources,
			}
			deployment, version, features, err := SerializeDeploymentWithMetadata(ctx, snap, false)
			require.NoError(t, err)
			require.NotNil(t, deployment)
			assert.Equal(t, tt.expectedVersion, version)
			assert.Equal(t, tt.expectedFeatures, features)

			deployment2, err := SerializeDeployment(ctx, snap, false)
			require.NoError(t, err)
			require.NotNil(t, deployment2)
			assert.Equal(t, deployment, deployment2)

			untypedDeployment, err := SerializeUntypedDeployment(ctx, snap, nil /*opts*/)
			require.NoError(t, err)
			require.NotNil(t, untypedDeployment)
			assert.Equal(t, tt.expectedVersion, untypedDeployment.Version)
			assert.Equal(t, tt.expectedFeatures, untypedDeployment.Features)

			deploymentJSON, err := json.Marshal(deployment)
			require.NoError(t, err)
			require.NotNil(t, deploymentJSON)
			assert.Equal(t, json.RawMessage(deploymentJSON), untypedDeployment.Deployment)
		})
	}
}

func TestLoadTooNewDeployment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	untypedDeployment := &apitype.UntypedDeployment{
		Version: DeploymentSchemaVersionLatest + 1,
	}

	deployment, err := DeserializeUntypedDeployment(ctx, untypedDeployment, b64.Base64SecretsProvider)
	assert.Nil(t, deployment)
	assert.Error(t, err)
	assert.Equal(t, ErrDeploymentSchemaVersionTooNew, err)
}

func TestLoadTooOldDeployment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	untypedDeployment := &apitype.UntypedDeployment{
		Version: DeploymentSchemaVersionOldestSupported - 1,
	}

	deployment, err := DeserializeUntypedDeployment(ctx, untypedDeployment, b64.Base64SecretsProvider)
	assert.Nil(t, deployment)
	assert.Error(t, err)
	assert.Equal(t, ErrDeploymentSchemaVersionTooOld, err)
}

// TestUnsupportedFeature tests that an unsupported feature in the deployment errors as expected.
func TestUnsupportedFeature(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	untypedDeployment := &apitype.UntypedDeployment{
		Version: DeploymentSchemaVersionLatest,
		Features: []string{
			"unsupported-feature",
		},
	}

	deployment, err := DeserializeUntypedDeployment(ctx, untypedDeployment, b64.Base64SecretsProvider)
	require.Nil(t, deployment)
	var expectedErr *ErrDeploymentUnsupportedFeatures
	require.ErrorAs(t, err, &expectedErr)
	require.Equal(t, []string{"unsupported-feature"}, expectedErr.Features)
}

// TestDeserializeUntypedDeploymentFeatures tests that the deserializer does not error for features that are supported.
func TestDeserializeUntypedDeploymentFeatures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for _, features := range combinations.All([]string{
		"refreshBeforeUpdate",
		"views",
		"hooks",
		"taint",
	}) {
		t.Run(strings.Join(features, ","), func(t *testing.T) {
			t.Parallel()

			untypedDeployment := &apitype.UntypedDeployment{
				Version:    DeploymentSchemaVersionLatest,
				Features:   features,
				Deployment: json.RawMessage("{}"),
			}
			deployment, err := DeserializeUntypedDeployment(ctx, untypedDeployment, b64.Base64SecretsProvider)
			require.NoError(t, err)
			require.NotNil(t, deployment)
		})
	}
}

func TestUnsupportedSecret(t *testing.T) {
	t.Parallel()

	rawProp := map[string]any{
		resource.SigKey: resource.SecretSig,
	}
	_, err := DeserializePropertyValue(rawProp, config.NewPanicCrypter())
	assert.Error(t, err)
}

func TestUnknownSig(t *testing.T) {
	t.Parallel()

	rawProp := map[string]any{
		resource.SigKey: "foobar",
	}
	_, err := DeserializePropertyValue(rawProp, config.NewPanicCrypter())
	assert.Error(t, err)
}

// TestDeserializeResourceReferencePropertyValueID tests the ability of the deserializer to handle resource references
// that were serialized without unwrapping their ID PropertyValue due to a bug in the serializer. Such resource
// references were produced by Pulumi v2.18.0.
func TestDeserializeResourceReferencePropertyValueID(t *testing.T) {
	t.Parallel()

	// Serialize replicates Pulumi 2.18.0's buggy resource reference serializer. We round-trip the value through JSON
	// in order to convert the ID property value into a plain map[string]any.
	serialize := func(v resource.PropertyValue) any {
		ref := v.ResourceReferenceValue()
		bytes, err := json.Marshal(map[string]any{
			resource.SigKey:  resource.ResourceReferenceSig,
			"urn":            ref.URN,
			"id":             ref.ID,
			"packageVersion": ref.PackageVersion,
		})
		contract.IgnoreError(err)
		var sv any
		err = json.Unmarshal(bytes, &sv)
		contract.IgnoreError(err)
		return sv
	}

	serialized := map[string]any{
		"component-resource":         serialize(resource.MakeComponentResourceReference("urn", "1.2.3")),
		"custom-resource":            serialize(resource.MakeCustomResourceReference("urn2", "id", "2.3.4")),
		"custom-resource-unknown-id": serialize(resource.MakeCustomResourceReference("urn3", "", "3.4.5")),
	}

	deserialized, err := DeserializePropertyValue(serialized, config.NewPanicCrypter())
	require.NoError(t, err)

	assert.Equal(t, resource.NewPropertyValue(map[string]any{
		"component-resource":         resource.MakeComponentResourceReference("urn", "1.2.3").V,
		"custom-resource":            resource.MakeCustomResourceReference("urn2", "id", "2.3.4").V,
		"custom-resource-unknown-id": resource.MakeCustomResourceReference("urn3", "", "3.4.5").V,
	}), deserialized)
}

func TestCustomSerialization(t *testing.T) {
	t.Parallel()

	textAsset, err := rasset.FromText("alpha beta gamma")
	require.NoError(t, err)

	strProp := resource.NewProperty("strProp")

	computed := resource.Computed{Element: strProp}
	output := resource.Output{Element: strProp}
	secret := &resource.Secret{Element: strProp}

	propMap := resource.NewPropertyMapFromMap(map[string]any{
		// Primitive types
		"nil":     nil,
		"bool":    true,
		"int32":   int64(41),
		"int64":   int64(42),
		"float32": float32(2.5),
		"float64": float64(1.5),
		"string":  "string literal",

		// Data structures
		"array":       []any{"a", true, float64(32)},
		"array-empty": []any{},

		"map": map[string]any{
			"a": true,
			"b": float64(88),
			"c": "c-see-saw",
			"d": "d-dee-daw",
		},
		"map-empty": map[string]any{},

		// Specialized resource types
		"asset-text": textAsset,

		"computed": computed,
		"output":   output,
		"secret":   secret,
	})

	assert.True(t, propMap.ContainsSecrets())
	assert.True(t, propMap.ContainsUnknowns())

	// Confirm the expected shape of serializing a ResourceProperty and PropertyMap using the
	// reflection-based default JSON encoder. This should NOT be used when serializing resources,
	// but we confirm the expected shape here while we migrate older code that relied on the
	// specific format.
	t.Run("SerializeToJSON", func(t *testing.T) {
		t.Parallel()

		b, err := json.Marshal(propMap)
		if err != nil {
			t.Fatalf("Marshalling PropertyMap: %v", err)
		}
		json := string(b)

		// Look for the specific JSON serialization of the properties.
		tests := []string{
			// Primitives
			`"nil":{"V":null}`,
			`"bool":{"V":true}`,
			`"string":{"V":"string literal"}}`,
			`"float32":{"V":2.5}`,
			`"float64":{"V":1.5}`,
			`"int32":{"V":41}`,
			`"int64":{"V":42}`,

			// Data structures
			`array":{"V":[{"V":"a"},{"V":true},{"V":32}]}`,
			`"array-empty":{"V":[]}`,
			`"map":{"V":{"a":{"V":true},"b":{"V":88},"c":{"V":"c-see-saw"},"d":{"V":"d-dee-daw"}}}`,
			`"map-empty":{"V":{}}`,

			// Specialized resource types
			//nolint:lll
			`"asset-text":{"V":{"4dabf18193072939515e22adb298388d":"c44067f5952c0a294b673a41bacd8c17","hash":"64989ccbf3efa9c84e2afe7cee9bc5828bf0fcb91e44f8c1e591638a2c2e90e3","text":"alpha beta gamma"}}`,

			`"computed":{"V":{"Element":{"V":"strProp"}}}`,
			`"output":{"V":{"Element":{"V":"strProp"}}}`,
			`"secret":{"V":{"Element":{"V":"strProp"}}}`,
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
		t.Parallel()

		serializedPropMap, err := SerializeProperties(context.Background(), propMap, config.BlindingCrypter, false /* showSecrets */)
		require.NoError(t, err)

		// Now JSON encode the results?
		b, err := json.Marshal(serializedPropMap)
		if err != nil {
			t.Fatalf("Marshalling PropertyMap: %v", err)
		}
		json := string(b)

		// Look for the specific JSON serialization of the properties.
		tests := []string{
			// Primitives
			`"bool":true`,
			`"string":"string literal"`,
			`"float32":2.5`,
			`"float64":1.5`,
			`"int32":41`,
			`"int64":42`,
			`"nil":null`,

			// Data structures
			`"array":["a",true,32]`,
			`"array-empty":[]`,
			`"map":{"a":true,"b":88,"c":"c-see-saw","d":"d-dee-daw"}`,
			`"map-empty":{}`,

			// Specialized resource types
			//nolint:lll
			`"asset-text":{"4dabf18193072939515e22adb298388d":"c44067f5952c0a294b673a41bacd8c17","hash":"64989ccbf3efa9c84e2afe7cee9bc5828bf0fcb91e44f8c1e591638a2c2e90e3","text":"alpha beta gamma"}`,

			// Computed values are replaced with a magic constant.
			`"computed":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"`,
			`"output":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"`,

			// Secrets are serialized with the special sig key, and their underlying cipher text.
			// Since we passed in a config.BlindingCrypter the cipher text isn't super-useful.
			`"secret":{"4dabf18193072939515e22adb298388d":"1b47061264138c4ac30d75fd1eb44270","ciphertext":"[secret]"}`,
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

func TestDeserializeDeploymentSecretCache(t *testing.T) {
	t.Parallel()

	urn := "urn:pulumi:prod::acme::acme:erp:Backend$aws:ebs/volume:Volume::PlatformBackendDb"
	ctx := context.Background()
	_, err := DeserializeDeploymentV3(ctx, apitype.DeploymentV3{
		SecretsProviders: &apitype.SecretsProvidersV1{Type: b64.Type},
		Resources: []apitype.ResourceV3{
			{
				URN:    resource.URN(urn),
				Type:   "aws:ebs/volume:Volume",
				Custom: true,
				ID:     "vol-044ba5ad2bd959bc1",
			},
		},
	}, b64.Base64SecretsProvider)
	require.NoError(t, err)
}

func TestDeserializeInvalidResourceErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	deployment, err := DeserializeDeploymentV3(ctx, apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{
			{},
		},
	}, b64.Base64SecretsProvider)
	assert.Nil(t, deployment)
	assert.EqualError(t, err, "resource missing required 'urn' field")

	urn := "urn:pulumi:prod::acme::acme:erp:Backend$aws:ebs/volume:Volume::PlatformBackendDb"

	deployment, err = DeserializeDeploymentV3(ctx, apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{
			{
				URN: resource.URN(urn),
			},
		},
	}, b64.Base64SecretsProvider)
	assert.Nil(t, deployment)
	assert.EqualError(t, err, fmt.Sprintf("resource '%s' missing required 'type' field", urn))

	deployment, err = DeserializeDeploymentV3(ctx, apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{
			{
				URN:    resource.URN(urn),
				Type:   "aws:ebs/volume:Volume",
				Custom: false,
				ID:     "vol-044ba5ad2bd959bc1",
			},
		},
	}, b64.Base64SecretsProvider)
	assert.Nil(t, deployment)
	assert.EqualError(t, err, fmt.Sprintf("resource '%s' has 'custom' false but non-empty ID", urn))
}

func TestDeserializeMissingSecretsManager(t *testing.T) {
	t.Parallel()

	urn := "urn:pulumi:urn:pulumi:test_stack::test_project::pkg:index:type::name"
	ctx := context.Background()
	deployment, err := DeserializeDeploymentV3(ctx, apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{
			{
				URN:  resource.URN(urn),
				Type: "pkg:index:type",
				Outputs: map[string]any{
					"secret": map[string]any{
						"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
						"ciphertext":                       "v1:xRi3+sQJSJHR8sha:RM8BfzSAJI84QMl+zLGjzPvwSqV6zOSdd/I/V34h",
					},
				},
			},
		},
	}, b64.Base64SecretsProvider)
	assert.Nil(t, deployment)
	assert.EqualError(t, err, "decrypting secret value: failed to decrypt: snapshot contains encrypted secrets but no secrets manager could be found")

	deployment, err = DeserializeDeploymentV3(ctx, apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{
			{
				URN:  resource.URN(urn),
				Type: "pkg:index:type",
			},
		},
	}, b64.Base64SecretsProvider)
	require.NoError(t, err)
	assert.Equal(t, deployment, &deploy.Snapshot{
		Manifest: deploy.Manifest{
			Time:    time.Time{},
			Version: "",
			Plugins: nil,
		},
		SecretsManager: nil,
		Resources: []*resource.State{
			{
				Type:         "pkg:index:type",
				URN:          resource.URN(urn),
				Custom:       false,
				Delete:       false,
				ID:           "",
				Inputs:       resource.PropertyMap{},
				Outputs:      resource.PropertyMap{},
				Parent:       "",
				Protect:      false,
				Dependencies: nil,
			},
		},
		PendingOperations: nil,
	})
}

func TestSerializePropertyValue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rapid.Check(t, func(t *rapid.T) {
		v := resource_testing.PropertyValueGenerator(6).Draw(t, "property value")
		_, err := SerializePropertyValue(ctx, v, config.NopEncrypter, false)
		require.NoError(t, err)
	})
}

// Test that if ShowSecrets is set the encrypter is not called into at all.
func TestSerializePropertyValue_ShowSecrets(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	crypter := config.NewPanicCrypter()

	secret := resource.MakeSecret(resource.NewProperty("secret"))
	_, err := SerializePropertyValue(ctx, secret, crypter, true)
	require.NoError(t, err)

	secret = resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
		resource.MakeSecret(resource.NewProperty("secret")),
	}))
	_, err = SerializePropertyValue(ctx, secret, crypter, true)
	require.NoError(t, err)
}

func TestDeserializePropertyValue(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		v := ObjectValueGenerator(6).Draw(t, "property value")
		_, err := DeserializePropertyValue(v, config.NopDecrypter)
		require.NoError(t, err)
	})
}

func wireValue(v resource.PropertyValue) (any, error) {
	object, err := SerializePropertyValue(context.Background(), v, config.NopEncrypter, false)
	if err != nil {
		return nil, err
	}

	wire, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	var wireObject any
	err = json.Unmarshal(wire, &wireObject)
	if err != nil {
		return nil, err
	}
	return wireObject, nil
}

func TestPropertyValueSchema(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // uses rapid.T not golang testing.T
	t.Run("serialized", rapid.MakeCheck(func(t *rapid.T) {
		wireObject, err := wireValue(resource_testing.PropertyValueGenerator(6).Draw(t, "property value"))
		require.NoError(t, err)

		err = propertyValueSchema.Validate(wireObject)
		require.NoError(t, err)
	}))

	//nolint:paralleltest // uses rapid.T not golang testing.T
	t.Run("synthetic", rapid.MakeCheck(func(t *rapid.T) {
		wireObject := ObjectValueGenerator(6).Draw(t, "wire object")
		err := propertyValueSchema.Validate(wireObject)
		require.NoError(t, err)
	}))
}

func replaceOutputsWithComputed(v resource.PropertyValue) resource.PropertyValue {
	switch {
	case v.IsArray():
		a := v.ArrayValue()
		for i, v := range a {
			a[i] = replaceOutputsWithComputed(v)
		}
	case v.IsObject():
		o := v.ObjectValue()
		for k, v := range o {
			o[k] = replaceOutputsWithComputed(v)
		}
	case v.IsOutput():
		return resource.MakeComputed(resource.NewProperty(""))
	case v.IsSecret():
		v.SecretValue().Element = replaceOutputsWithComputed(v.SecretValue().Element)
	}
	return v
}

func TestRoundTripPropertyValue(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		original := resource_testing.PropertyValueGenerator(6).Draw(t, "property value")
		wireObject, err := wireValue(original)
		require.NoError(t, err)

		deserialized, err := DeserializePropertyValue(wireObject, config.NopDecrypter)
		require.NoError(t, err)

		resource_testing.AssertEqualPropertyValues(t, replaceOutputsWithComputed(original), deserialized)
	})
}

// UnknownObjectGenerator generates the unknown object value.
func UnknownObjectGenerator() *rapid.Generator[any] {
	return rapid.Custom(func(t *rapid.T) any {
		return rapid.Just(computedValuePlaceholder).Draw(t, "unknowns")
	})
}

// BoolObjectGenerator generates boolean object values.
func BoolObjectGenerator() *rapid.Generator[bool] {
	return rapid.Custom(func(t *rapid.T) bool {
		return rapid.Bool().Draw(t, "booleans")
	})
}

// NumberObjectGenerator generates numeric object values.
func NumberObjectGenerator() *rapid.Generator[float64] {
	return rapid.Custom(func(t *rapid.T) float64 {
		return rapid.Float64().Draw(t, "numbers")
	})
}

// StringObjectGenerator generates string object values.
func StringObjectGenerator() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		return rapid.String().Draw(t, "strings")
	})
}

// TextAssetObjectGenerator generates textual asset object values.
func TextAssetObjectGenerator() *rapid.Generator[map[string]any] {
	return rapid.Custom(func(t *rapid.T) map[string]any {
		return map[string]any{
			resource.SigKey:            resource.AssetSig,
			resource.AssetTextProperty: rapid.String().Draw(t, "text asset contents"),
		}
	})
}

// AssetObjectGenerator generates asset object values.
func AssetObjectGenerator() *rapid.Generator[map[string]any] {
	return TextAssetObjectGenerator()
}

// LiteralArchiveObjectGenerator generates archive object values with literal archive contents.
func LiteralArchiveObjectGenerator(maxDepth int) *rapid.Generator[map[string]any] {
	return rapid.Custom(func(t *rapid.T) map[string]any {
		var contentsGenerator *rapid.Generator[map[string]any]
		if maxDepth > 0 {
			contentsGenerator = rapid.MapOfN(
				rapid.StringMatching(`^(/[^[:cntrl:]/]+)*/?[^[:cntrl:]/]+$`),
				rapid.OneOf(
					AssetObjectGenerator().AsAny(),
					ArchiveObjectGenerator(maxDepth-1).AsAny(),
				), 0, 16)
		} else {
			contentsGenerator = rapid.Just(map[string]any{})
		}

		return map[string]any{
			resource.SigKey:                resource.ArchiveSig,
			resource.ArchiveAssetsProperty: contentsGenerator.Draw(t, "literal archive contents"),
		}
	})
}

// ArchiveObjectGenerator generates archive object values.
func ArchiveObjectGenerator(maxDepth int) *rapid.Generator[map[string]any] {
	return LiteralArchiveObjectGenerator(maxDepth)
}

// ResourceReferenceObjectGenerator generates resource reference object values.
func ResourceReferenceObjectGenerator() *rapid.Generator[any] {
	return rapid.Custom(func(t *rapid.T) any {
		fields := map[string]any{
			resource.SigKey:  resource.ResourceReferenceSig,
			"urn":            string(resource_testing.URNGenerator().Draw(t, "referenced URN")),
			"packageVersion": resource_testing.SemverStringGenerator().Draw(t, "package version"),
		}

		id := rapid.OneOf(
			UnknownObjectGenerator(),
			StringObjectGenerator().AsAny(),
		).Draw(t, "referenced ID")
		if idstr := id.(string); idstr != "" && idstr != computedValuePlaceholder {
			fields["id"] = id
		}

		return fields
	})
}

// ArrayObjectGenerator generates array object values. The maxDepth parameter controls the maximum
// depth of the elements of the array.
func ArrayObjectGenerator(maxDepth int) *rapid.Generator[[]any] {
	return rapid.Custom(func(t *rapid.T) []any {
		return rapid.SliceOfN(ObjectValueGenerator(maxDepth-1), 0, 32).Draw(t, "array elements")
	})
}

// MapObjectGenerator generates map object values. The maxDepth parameter controls the maximum
// depth of the elements of the map.
func MapObjectGenerator(maxDepth int) *rapid.Generator[map[string]any] {
	return rapid.Custom(func(t *rapid.T) map[string]any {
		return rapid.MapOfN(rapid.String(), ObjectValueGenerator(maxDepth-1), 0, 32).Draw(t, "map elements")
	})
}

// SecretObjectGenerator generates secret object values. The maxDepth parameter controls the maximum
// depth of the plaintext value of the secret, if any.
func SecretObjectGenerator(maxDepth int) *rapid.Generator[map[string]any] {
	return rapid.Custom(func(t *rapid.T) map[string]any {
		value := ObjectValueGenerator(maxDepth-1).Draw(t, "secret element")
		bytes, err := json.Marshal(value)
		require.NoError(t, err)

		return map[string]any{
			resource.SigKey: resource.SecretSig,
			"plaintext":     string(bytes),
		}
	})
}

// ObjectValueGenerator generates arbitrary object values. The maxDepth parameter controls the maximum
// number of times the generator may recur.
func ObjectValueGenerator(maxDepth int) *rapid.Generator[any] {
	choices := []*rapid.Generator[any]{
		UnknownObjectGenerator(),
		BoolObjectGenerator().AsAny(),
		NumberObjectGenerator().AsAny(),
		StringObjectGenerator().AsAny(),
		AssetObjectGenerator().AsAny(),
		ResourceReferenceObjectGenerator(),
	}
	if maxDepth > 0 {
		choices = append(choices,
			ArchiveObjectGenerator(maxDepth).AsAny(),
			ArrayObjectGenerator(maxDepth).AsAny(),
			MapObjectGenerator(maxDepth).AsAny(),
			SecretObjectGenerator(maxDepth).AsAny())
	}
	return rapid.OneOf(choices...)
}

func TestSecretInputRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	res := &resource.State{
		URN:  "urn:pulumi:stack::project::pkg:index:type::name",
		Type: "pkg:index:type",
		Inputs: resource.NewPropertyMapFromMap(map[string]any{
			"normal": "hello",
			"secret": resource.MakeSecret(resource.NewProperty("there")),
		}),
	}

	sm := b64.NewBase64SecretsManager()

	serialized, err := SerializeResource(ctx, res, sm.Encrypter(), false /* showSecrets */)
	require.NoError(t, err)

	deserialized, err := DeserializeResource(serialized, sm.Decrypter())
	require.NoError(t, err)
	require.Equal(t, resource.NewPropertyMapFromMap(map[string]any{
		"normal": "hello",
		"secret": resource.MakeSecret(resource.NewProperty("there")),
	}), deserialized.Inputs)
}
