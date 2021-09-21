// Copyright 2016-2021, Pulumi Corporation.
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
package plugin

import (
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	resource_testing "github.com/pulumi/pulumi/sdk/v3/go/common/resource/testing"
)

var marshalOpts = MarshalOptions{
	KeepUnknowns:     true,
	KeepSecrets:      true,
	KeepResources:    true,
	KeepOutputValues: true,
}

func TestOutputValueTurnaround(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := resource_testing.OutputPropertyGenerator(1).Draw(t, "output").(resource.PropertyValue)
		pb, err := MarshalPropertyValue("", v, marshalOpts)
		require.NoError(t, err)

		v2, err := UnmarshalPropertyValue("", pb, marshalOpts)
		require.NoError(t, err)
		require.NotNil(t, v2)

		resource_testing.AssertEqualPropertyValues(t, v, *v2)
	})
}

func TestSerializePropertyValues(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := resource_testing.PropertyValueGenerator(6).Draw(t, "property value").(resource.PropertyValue)
		_, err := MarshalPropertyValue("", v, marshalOpts)
		require.NoError(t, err)
	})
}

func TestDeserializePropertyValues(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := ValueGenerator(6).Draw(t, "wire value").(*structpb.Value)
		_, err := UnmarshalPropertyValue("", v, marshalOpts)
		require.NoError(t, err)
	})
}

func stringValue(s string) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: s,
		},
	}
}

// UnknownGenerator generates the unknown resource.PropertyValue.
func UnknownGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return rapid.Just(stringValue(UnknownStringValue)).Draw(t, "unknowns").(*structpb.Value)
	})
}

// NullGenerator generates the null resource.PropertyValue.
func NullGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return rapid.Just(&structpb.Value{
			Kind: &structpb.Value_NullValue{
				NullValue: structpb.NullValue_NULL_VALUE,
			},
		}).Draw(t, "nulls").(*structpb.Value)
	})
}

// BoolGenerator generates boolean resource.PropertyValues.
func BoolGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_BoolValue{
				BoolValue: rapid.Bool().Draw(t, "booleans").(bool),
			},
		}
	})
}

// NumberGenerator generates numeric resource.PropertyValues.
func NumberGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_NumberValue{
				NumberValue: rapid.Float64().Draw(t, "numbers").(float64),
			},
		}
	})
}

// StringGenerator generates string resource.PropertyValues.
func StringGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return stringValue(rapid.String().Draw(t, "strings").(string))
	})
}

// TextAssetGenerator generates textual *resource.Asset values.
func TextAssetGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						resource.SigKey:            stringValue(resource.AssetSig),
						resource.AssetTextProperty: stringValue(rapid.String().Draw(t, "text asset contents").(string)),
					},
				},
			},
		}
	})
}

// AssetGenerator generates *resource.Asset values.
func AssetGenerator() *rapid.Generator {
	return TextAssetGenerator()
}

// LiteralArchiveGenerator generates *resource.Archive values with literal archive contents.
func LiteralArchiveGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		var contentsGenerator *rapid.Generator
		if maxDepth > 0 {
			contentsGenerator = rapid.MapOfN(rapid.StringMatching(`^(/[^[:cntrl:]/]+)*/?[^[:cntrl:]/]+$`), rapid.OneOf(AssetGenerator(), ArchiveGenerator(maxDepth-1)), 0, 16)
		} else {
			contentsGenerator = rapid.Just(map[string]*structpb.Value{})
		}

		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						resource.SigKey: stringValue(resource.ArchiveSig),
						resource.ArchiveAssetsProperty: {
							Kind: &structpb.Value_StructValue{
								StructValue: &structpb.Struct{
									Fields: contentsGenerator.Draw(t, "literal archive contents").(map[string]*structpb.Value),
								},
							},
						},
					},
				},
			},
		}
	})
}

// ArchiveGenerator generates *resource.Archive values.
func ArchiveGenerator(maxDepth int) *rapid.Generator {
	return LiteralArchiveGenerator(maxDepth)
}

// ResourceReferenceGenerator generates resource.ResourceReference values.
func ResourceReferenceGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		fields := map[string]*structpb.Value{
			resource.SigKey: stringValue(resource.ResourceReferenceSig),
			"urn":           stringValue(string(resource_testing.URNGenerator().Draw(t, "referenced URN").(resource.URN))),
		}

		id := rapid.OneOf(UnknownGenerator(), StringGenerator()).Draw(t, "referenced ID").(*structpb.Value)
		if id.Kind.(*structpb.Value_StringValue).StringValue != UnknownStringValue {
			fields["id"] = id
		}

		packageVersion := resource_testing.SemverStringGenerator().Draw(t, "package version").(string)
		if packageVersion != "" {
			fields["packageVersion"] = stringValue(packageVersion)
		}

		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: fields,
				},
			},
		}
	})
}

// ArrayGenerator generates array resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the elements of the array.
func ArrayGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{
					Values: rapid.SliceOfN(ValueGenerator(maxDepth-1), 0, 32).Draw(t, "array elements").([]*structpb.Value),
				},
			},
		}
	})
}

// ObjectGenerator generates resource.PropertyMap values. The maxDepth parameter controls the maximum
// depth of the elements of the map.
func ObjectGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: rapid.MapOfN(rapid.String(), ValueGenerator(maxDepth-1), 0, 32).Draw(t, "map elements").(map[string]*structpb.Value),
				},
			},
		}
	})
}

// OutputGenerator generates output resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the resolved value of the output, if any.
func OutputGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		fields := map[string]*structpb.Value{
			resource.SigKey: stringValue(resource.OutputValueSig),
		}

		if rapid.Bool().Draw(t, "known").(bool) {
			fields["value"] = ValueGenerator(maxDepth-1).Draw(t, "output element").(*structpb.Value)
		}

		if rapid.Bool().Draw(t, "secret").(bool) {
			fields["secret"] = &structpb.Value{
				Kind: &structpb.Value_BoolValue{
					BoolValue: true,
				},
			}
		}

		deps := rapid.SliceOfN(resource_testing.URNGenerator(), 0, 32).Draw(t, "dependencies").([]resource.URN)
		if len(deps) != 0 {
			wireDeps := make([]*structpb.Value, len(deps))
			for i, urn := range deps {
				wireDeps[i] = stringValue(string(urn))
			}
			fields["dependencies"] = &structpb.Value{
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: wireDeps,
					},
				},
			}
		}

		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: fields,
				},
			},
		}
	})
}

// SecretGenerator generates secret resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the plaintext value of the secret, if any.
func SecretGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						resource.SigKey: stringValue(resource.SecretSig),
						"value":         ValueGenerator(maxDepth-1).Draw(t, "secret element").(*structpb.Value),
					},
				},
			},
		}
	})
}

// ValueGenerator generates arbitrary resource.PropertyValues. The maxDepth parameter controls the maximum
// number of times the generator may recur.
func ValueGenerator(maxDepth int) *rapid.Generator {
	choices := []*rapid.Generator{
		UnknownGenerator(),
		NullGenerator(),
		BoolGenerator(),
		NumberGenerator(),
		StringGenerator(),
		AssetGenerator(),
		ResourceReferenceGenerator(),
	}
	if maxDepth > 0 {
		choices = append(choices,
			ArchiveGenerator(maxDepth),
			ArrayGenerator(maxDepth),
			ObjectGenerator(maxDepth),
			OutputGenerator(maxDepth),
			SecretGenerator(maxDepth))
	}
	return rapid.OneOf(choices...)
}
