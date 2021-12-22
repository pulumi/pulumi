package events

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	resource_testing "github.com/pulumi/pulumi/sdk/v3/go/common/resource/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func TestUnsupportedSecret(t *testing.T) {
	rawProp := map[string]interface{}{
		resource.SigKey: resource.SecretSig,
	}
	_, err := DeserializePropertyValue(rawProp, config.NewPanicCrypter(), config.NewPanicCrypter())
	assert.Error(t, err)
}

func TestUnknownSig(t *testing.T) {
	rawProp := map[string]interface{}{
		resource.SigKey: "foobar",
	}
	_, err := DeserializePropertyValue(rawProp, config.NewPanicCrypter(), config.NewPanicCrypter())
	assert.Error(t, err)
}

// TestDeserializeResourceReferencePropertyValueID tests the ability of the deserializer to handle resource references
// that were serialized without unwrapping their ID PropertyValue due to a bug in the serializer. Such resource
// references were produced by Pulumi v2.18.0.
func TestDeserializeResourceReferencePropertyValueID(t *testing.T) {
	// Serialize replicates Pulumi 2.18.0's buggy resource reference serializer. We round-trip the value through JSON
	// in order to convert the ID property value into a plain map[string]interface{}.
	serialize := func(v resource.PropertyValue) interface{} {
		ref := v.ResourceReferenceValue()
		bytes, err := json.Marshal(map[string]interface{}{
			resource.SigKey:  resource.ResourceReferenceSig,
			"urn":            ref.URN,
			"id":             ref.ID,
			"packageVersion": ref.PackageVersion,
		})
		contract.IgnoreError(err)
		var sv interface{}
		err = json.Unmarshal(bytes, &sv)
		contract.IgnoreError(err)
		return sv
	}

	serialized := map[string]interface{}{
		"component-resource":         serialize(resource.MakeComponentResourceReference("urn", "1.2.3")),
		"custom-resource":            serialize(resource.MakeCustomResourceReference("urn2", "id", "2.3.4")),
		"custom-resource-unknown-id": serialize(resource.MakeCustomResourceReference("urn3", "", "3.4.5")),
	}

	deserialized, err := DeserializePropertyValue(serialized, config.NewPanicCrypter(), config.NewPanicCrypter())
	assert.NoError(t, err)

	assert.Equal(t, resource.NewPropertyValue(map[string]interface{}{
		"component-resource":         resource.MakeComponentResourceReference("urn", "1.2.3").V,
		"custom-resource":            resource.MakeCustomResourceReference("urn2", "id", "2.3.4").V,
		"custom-resource-unknown-id": resource.MakeCustomResourceReference("urn3", "", "3.4.5").V,
	}), deserialized)
}

func TestCustomSerialization(t *testing.T) {
	textAsset, err := resource.NewTextAsset("alpha beta gamma")
	assert.NoError(t, err)

	strProp := resource.NewStringProperty("strProp")

	computed := resource.Computed{Element: strProp}
	output := resource.Output{Element: strProp}
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

	assert.True(t, propMap.ContainsSecrets())
	assert.True(t, propMap.ContainsUnknowns())

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
			// nolint: lll
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
		serializedPropMap, err := SerializeProperties(propMap, config.BlindingCrypter, false /* showSecrets */)
		assert.NoError(t, err)

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
			// nolint: lll
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

func TestSerializePropertyValue(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := resource_testing.PropertyValueGenerator(6).Draw(t, "property value").(resource.PropertyValue)
		_, err := SerializePropertyValue(v, config.NopEncrypter, false)
		assert.NoError(t, err)
	})
}

func TestDeserializePropertyValue(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := ObjectValueGenerator(6).Draw(t, "property value")
		_, err := DeserializePropertyValue(v, config.NopDecrypter, config.NopEncrypter)
		assert.NoError(t, err)
	})
}

func wireValue(v resource.PropertyValue) (interface{}, error) {
	object, err := SerializePropertyValue(v, config.NopEncrypter, false)
	if err != nil {
		return nil, err
	}

	wire, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	var wireObject interface{}
	err = json.Unmarshal(wire, &wireObject)
	if err != nil {
		return nil, err
	}
	return wireObject, nil
}

func TestPropertyValueSchema(t *testing.T) {
	t.Run("serialized", rapid.MakeCheck(func(t *rapid.T) {
		wireObject, err := wireValue(resource_testing.PropertyValueGenerator(6).Draw(t, "property value").(resource.PropertyValue))
		require.NoError(t, err)

		err = propertyValueSchema.Validate(wireObject)
		assert.NoError(t, err)
	}))

	t.Run("synthetic", rapid.MakeCheck(func(t *rapid.T) {
		wireObject := ObjectValueGenerator(6).Draw(t, "wire object")
		err := propertyValueSchema.Validate(wireObject)
		assert.NoError(t, err)
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
		return resource.MakeComputed(resource.NewStringProperty(""))
	case v.IsSecret():
		v.SecretValue().Element = replaceOutputsWithComputed(v.SecretValue().Element)
	}
	return v
}

func TestRoundTripPropertyValue(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := resource_testing.PropertyValueGenerator(6).Draw(t, "property value").(resource.PropertyValue)
		wireObject, err := wireValue(original)
		require.NoError(t, err)

		deserialized, err := DeserializePropertyValue(wireObject, config.NopDecrypter, config.NopEncrypter)
		require.NoError(t, err)

		resource_testing.AssertEqualPropertyValues(t, replaceOutputsWithComputed(original), deserialized)
	})
}

// UnknownObjectGenerator generates the unknown object value.
func UnknownObjectGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) interface{} {
		return rapid.Just(computedValuePlaceholder).Draw(t, "unknowns")
	})
}

// BoolObjectGenerator generates boolean object values.
func BoolObjectGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) interface{} {
		return rapid.Bool().Draw(t, "booleans")
	})
}

// NumberObjectGenerator generates numeric object values.
func NumberObjectGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) interface{} {
		return rapid.Float64().Draw(t, "numbers")
	})
}

// StringObjectGenerator generates string object values.
func StringObjectGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) interface{} {
		return rapid.String().Draw(t, "strings")
	})
}

// TextAssetObjectGenerator generates textual asset object values.
func TextAssetObjectGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) interface{} {
		return map[string]interface{}{
			resource.SigKey:            resource.AssetSig,
			resource.AssetTextProperty: rapid.String().Draw(t, "text asset contents"),
		}
	})
}

// AssetObjectGenerator generates asset object values.
func AssetObjectGenerator() *rapid.Generator {
	return TextAssetObjectGenerator()
}

// LiteralArchiveObjectGenerator generates archive object values with literal archive contents.
func LiteralArchiveObjectGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) map[string]interface{} {
		var contentsGenerator *rapid.Generator
		if maxDepth > 0 {
			contentsGenerator = rapid.MapOfN(rapid.StringMatching(`^(/[^[:cntrl:]/]+)*/?[^[:cntrl:]/]+$`), rapid.OneOf(AssetObjectGenerator(), ArchiveObjectGenerator(maxDepth-1)), 0, 16)
		} else {
			contentsGenerator = rapid.Just(map[string]interface{}{})
		}

		return map[string]interface{}{
			resource.SigKey:                resource.ArchiveSig,
			resource.ArchiveAssetsProperty: contentsGenerator.Draw(t, "literal archive contents"),
		}
	})
}

// ArchiveObjectGenerator generates archive object values.
func ArchiveObjectGenerator(maxDepth int) *rapid.Generator {
	return LiteralArchiveObjectGenerator(maxDepth)
}

// ResourceReferenceObjectGenerator generates resource reference object values.
func ResourceReferenceObjectGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) interface{} {
		fields := map[string]interface{}{
			resource.SigKey:  resource.ResourceReferenceSig,
			"urn":            string(resource_testing.URNGenerator().Draw(t, "referenced URN").(resource.URN)),
			"packageVersion": resource_testing.SemverStringGenerator().Draw(t, "package version"),
		}

		id := rapid.OneOf(UnknownObjectGenerator(), StringObjectGenerator()).Draw(t, "referenced ID")
		if idstr := id.(string); idstr != "" && idstr != computedValuePlaceholder {
			fields["id"] = id
		}

		return fields
	})
}

// ArrayObjectGenerator generates array object values. The maxDepth parameter controls the maximum
// depth of the elements of the array.
func ArrayObjectGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) interface{} {
		return rapid.SliceOfN(ObjectValueGenerator(maxDepth-1), 0, 32).Draw(t, "array elements")
	})
}

// MapObjectGenerator generates map object values. The maxDepth parameter controls the maximum
// depth of the elements of the map.
func MapObjectGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) interface{} {
		return rapid.MapOfN(rapid.String(), ObjectValueGenerator(maxDepth-1), 0, 32).Draw(t, "map elements")
	})
}

// SecretObjectGenerator generates secret object values. The maxDepth parameter controls the maximum
// depth of the plaintext value of the secret, if any.
func SecretObjectGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) interface{} {
		value := ObjectValueGenerator(maxDepth-1).Draw(t, "secret element")
		bytes, err := json.Marshal(value)
		require.NoError(t, err)

		return map[string]interface{}{
			resource.SigKey: resource.SecretSig,
			"plaintext":     string(bytes),
		}
	})
}

// ObjectValueGenerator generates arbitrary object values. The maxDepth parameter controls the maximum
// number of times the generator may recur.
func ObjectValueGenerator(maxDepth int) *rapid.Generator {
	choices := []*rapid.Generator{
		UnknownObjectGenerator(),
		BoolObjectGenerator(),
		NumberObjectGenerator(),
		StringObjectGenerator(),
		AssetObjectGenerator(),
		ResourceReferenceObjectGenerator(),
	}
	if maxDepth > 0 {
		choices = append(choices,
			ArchiveObjectGenerator(maxDepth),
			ArrayObjectGenerator(maxDepth),
			MapObjectGenerator(maxDepth),
			SecretObjectGenerator(maxDepth))
	}
	return rapid.OneOf(choices...)
}
