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

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
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
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		v := resource_testing.OutputPropertyGenerator(1).Draw(t, "output")
		pb, err := MarshalPropertyValue("", v, marshalOpts)
		require.NoError(t, err)

		v2, err := UnmarshalPropertyValue("", pb, marshalOpts)
		require.NoError(t, err)
		require.NotNil(t, v2)

		resource_testing.AssertEqualPropertyValues(t, v, *v2)
	})
}

func TestSerializePropertyValues(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		v := resource_testing.PropertyValueGenerator(6).Draw(t, "property value")
		_, err := MarshalPropertyValue("", v, marshalOpts)
		require.NoError(t, err)
	})
}

func TestDeserializePropertyValues(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		v := ValueGenerator(6).Draw(t, "wire value")
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

// UnknownValueGenerator generates the unknown *structpb.Value.
func UnknownValueGenerator() *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return rapid.Just(stringValue(UnknownStringValue)).Draw(t, "unknowns")
	})
}

// NullValueGenerator generates the null *structpb.Value.
func NullValueGenerator() *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return rapid.Just(&structpb.Value{
			Kind: &structpb.Value_NullValue{
				NullValue: structpb.NullValue_NULL_VALUE,
			},
		}).Draw(t, "nulls")
	})
}

// BoolValueGenerator generates boolean *structpb.Values.
func BoolValueGenerator() *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_BoolValue{
				BoolValue: rapid.Bool().Draw(t, "booleans"),
			},
		}
	})
}

// NumberValueGenerator generates numeric *structpb.Values.
func NumberValueGenerator() *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_NumberValue{
				NumberValue: rapid.Float64().Draw(t, "numbers"),
			},
		}
	})
}

// StringValueGenerator generates string *structpb.Values.
func StringValueGenerator() *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return stringValue(rapid.String().Draw(t, "strings"))
	})
}

// TextAssetValueGenerator generates textual asset *structpb.Values.
func TextAssetValueGenerator() *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						resource.SigKey:            stringValue(resource.AssetSig),
						resource.AssetTextProperty: stringValue(rapid.String().Draw(t, "text asset contents")),
					},
				},
			},
		}
	})
}

// AssetValueGenerator generates *structpb.Values.
func AssetValueGenerator() *rapid.Generator[*structpb.Value] {
	return TextAssetValueGenerator()
}

// LiteralArchiveValueGenerator generates *structpb.Values with literal archive contents.
func LiteralArchiveValueGenerator(maxDepth int) *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		var contentsGenerator *rapid.Generator[map[string]*structpb.Value]
		if maxDepth > 0 {
			contentsGenerator = rapid.MapOfN(rapid.StringMatching(`^(/[^[:cntrl:]/]+)*/?[^[:cntrl:]/]+$`), rapid.OneOf(AssetValueGenerator(), ArchiveValueGenerator(maxDepth-1)), 0, 16)
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
									Fields: contentsGenerator.Draw(t, "literal archive contents"),
								},
							},
						},
					},
				},
			},
		}
	})
}

// ArchiveValueGenerator generates archive *structpb.Values.
func ArchiveValueGenerator(maxDepth int) *rapid.Generator[*structpb.Value] {
	return LiteralArchiveValueGenerator(maxDepth)
}

// ResourceReferenceValueGenerator generates resource reference *structpb.Values.
func ResourceReferenceValueGenerator() *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		fields := map[string]*structpb.Value{
			resource.SigKey: stringValue(resource.ResourceReferenceSig),
			"urn":           stringValue(string(resource_testing.URNGenerator().Draw(t, "referenced URN"))),
		}

		id := rapid.OneOf(UnknownValueGenerator(), StringValueGenerator()).Draw(t, "referenced ID")
		if idstr := id.Kind.(*structpb.Value_StringValue).StringValue; idstr != "" && idstr != UnknownStringValue {
			fields["id"] = id
		}

		packageVersion := resource_testing.SemverStringGenerator().Draw(t, "package version")
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

// ArrayValueGenerator generates array *structpb.Values. The maxDepth parameter controls the maximum
// depth of the elements of the array.
func ArrayValueGenerator(maxDepth int) *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{
					Values: rapid.SliceOfN(ValueGenerator(maxDepth-1), 0, 32).Draw(t, "array elements"),
				},
			},
		}
	})
}

// ObjectValueGenerator generates *structpb.Values. The maxDepth parameter controls the maximum
// depth of the elements of the map.
func ObjectValueGenerator(maxDepth int) *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: rapid.MapOfN(rapid.String(), ValueGenerator(maxDepth-1), 0, 32).Draw(t, "map elements"),
				},
			},
		}
	})
}

// OutputValueGenerator generates output *structpb.Values. The maxDepth parameter controls the maximum
// depth of the resolved value of the output, if any.
func OutputValueGenerator(maxDepth int) *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		fields := map[string]*structpb.Value{
			resource.SigKey: stringValue(resource.OutputValueSig),
		}

		if rapid.Bool().Draw(t, "known") {
			fields["value"] = ValueGenerator(maxDepth-1).Draw(t, "output element")
		}

		if rapid.Bool().Draw(t, "secret") {
			fields["secret"] = &structpb.Value{
				Kind: &structpb.Value_BoolValue{
					BoolValue: true,
				},
			}
		}

		deps := rapid.SliceOfN(resource_testing.URNGenerator(), 0, 32).Draw(t, "dependencies")
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

// SecretValueGenerator generates secret *structpb.Values. The maxDepth parameter controls the maximum
// depth of the plaintext value of the secret, if any.
func SecretValueGenerator(maxDepth int) *rapid.Generator[*structpb.Value] {
	return rapid.Custom(func(t *rapid.T) *structpb.Value {
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						resource.SigKey: stringValue(resource.SecretSig),
						"value":         ValueGenerator(maxDepth-1).Draw(t, "secret element"),
					},
				},
			},
		}
	})
}

// ValueGenerator generates arbitrary *structpb.Values. The maxDepth parameter controls the maximum
// number of times the generator may recur.
func ValueGenerator(maxDepth int) *rapid.Generator[*structpb.Value] {
	choices := []*rapid.Generator[*structpb.Value]{
		UnknownValueGenerator(),
		NullValueGenerator(),
		BoolValueGenerator(),
		NumberValueGenerator(),
		StringValueGenerator(),
		AssetValueGenerator(),
		ResourceReferenceValueGenerator(),
	}
	if maxDepth > 0 {
		choices = append(choices,
			ArchiveValueGenerator(maxDepth),
			ArrayValueGenerator(maxDepth),
			ObjectValueGenerator(maxDepth),
			OutputValueGenerator(maxDepth),
			SecretValueGenerator(maxDepth))
	}
	return rapid.OneOf(choices...)
}
