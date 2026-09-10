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

package propertyrpc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pTest "github.com/pulumi/pulumi/sdk/v3/go/property/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/propertyrpc"
)

// TestMarshalThroughGRPC checks that a [property.Map] survives a round trip through [propertyrpc.Marshal] and
// [plugin.UnmarshalProperties].
func TestMarshalThroughGRPC(t *testing.T) {
	t.Parallel()

	opts := plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	}

	m := pTest.Map(10)
	rapid.Check(t, func(t *rapid.T) {
		source := m.Draw(t, "round-trip map")

		unmarshalled, err := plugin.UnmarshalProperties(propertyrpc.Marshal(source), opts)
		require.NoError(t, err)

		assert.Equal(t, source, resource.FromResourcePropertyMap(unmarshalled))
	})
}

// TestUnmarshalThroughGRPC checks that a [property.Map] survives a round trip through
// [plugin.MarshalProperties] and [propertyrpc.Unmarshal].
func TestUnmarshalThroughGRPC(t *testing.T) {
	t.Parallel()

	opts := plugin.MarshalOptions{
		KeepUnknowns:          true,
		KeepSecrets:           true,
		KeepResources:         true,
		KeepOutputValues:      true,
		UpgradeToOutputValues: true,
		KeepByteString:        true,
	}

	m := pTest.Map(10)
	rapid.Check(t, func(t *rapid.T) {
		source := m.Draw(t, "round-trip map")

		marshalled, err := plugin.MarshalProperties(resource.ToResourcePropertyMap(source), opts)
		require.NoError(t, err)

		actual, err := propertyrpc.Unmarshal(marshalled)
		require.NoError(t, err)

		assert.Equal(t, source, actual)
	})
}

// TestRoundTrip checks that [propertyrpc.Unmarshal] inverts [propertyrpc.Marshal].
func TestRoundTrip(t *testing.T) {
	t.Parallel()

	m := pTest.Map(10)
	rapid.Check(t, func(t *rapid.T) {
		source := m.Draw(t, "round-trip map")

		actual, err := propertyrpc.Unmarshal(propertyrpc.Marshal(source))
		require.NoError(t, err)

		assert.Equal(t, source, actual)
	})
}

// TestResourceReferenceRoundTrip checks resource references, which [pTest.Map] does not
// generate.
func TestResourceReferenceRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ref  property.ResourceReference
	}{
		{"known-id", property.ResourceReference{
			URN:            "urn:pulumi:stack::proj::pkg:index:Res::name",
			Name:           "name",
			Type:           "pkg:index:Res",
			ID:             property.New("id"),
			PackageVersion: "1.2.3",
		}},
		{"unknown-id", property.ResourceReference{
			URN: "urn:pulumi:stack::proj::pkg:index:Res::name",
			ID:  property.New(property.Computed),
		}},
		{"component", property.ResourceReference{
			URN: "urn:pulumi:stack::proj::pkg:index:Component::name",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			source := property.New(tt.ref)
			actual, err := propertyrpc.UnmarshalValue(propertyrpc.MarshalValue(source))
			require.NoError(t, err)
			assert.Equal(t, source, actual)
		})
	}
}

func TestUnmarshalErrors(t *testing.T) {
	t.Parallel()

	obj := func(fields map[string]*structpb.Value) *structpb.Value {
		return structpb.NewStructValue(&structpb.Struct{Fields: fields})
	}
	list := func(values ...*structpb.Value) *structpb.Value {
		return structpb.NewListValue(&structpb.ListValue{Values: values})
	}
	key, index := property.NewSegment[string], property.NewSegment[int]
	bogus := obj(map[string]*structpb.Value{sig.Key: structpb.NewStringValue("bogus")})

	tests := []struct {
		name         string
		input        *structpb.Value
		expectedPath property.Path
		expected     string
	}{
		{
			name:         "root",
			input:        bogus,
			expectedPath: property.PathFromSegments(),
			expected:     `<root>: unknown signature "bogus"`,
		},
		{
			name:         "map-key",
			input:        obj(map[string]*structpb.Value{"k": bogus}),
			expectedPath: property.PathFromSegments(key("k")),
			expected:     `k: unknown signature "bogus"`,
		},
		{
			name:         "array-index",
			input:        list(structpb.NewNumberValue(0), bogus),
			expectedPath: property.PathFromSegments(index(1)),
			expected:     `[1]: unknown signature "bogus"`,
		},
		{
			name:         "quoted-key",
			input:        obj(map[string]*structpb.Value{"a.b": bogus}),
			expectedPath: property.PathFromSegments(key("a.b")),
			expected:     `["a.b"]: unknown signature "bogus"`,
		},
		{
			name: "non-string-signature",
			input: obj(map[string]*structpb.Value{"k": obj(map[string]*structpb.Value{
				sig.Key: structpb.NewNumberValue(1),
			})}),
			expectedPath: property.PathFromSegments(key("k")),
			expected:     `k: unknown signature ""`,
		},
		{
			// The secret marker does not add a segment to the path.
			name: "in-secret",
			input: obj(map[string]*structpb.Value{"s": obj(map[string]*structpb.Value{
				sig.Key: structpb.NewStringValue(sig.Secret),
				"value": obj(map[string]*structpb.Value{"k": bogus}),
			})}),
			expectedPath: property.PathFromSegments(key("s"), key("k")),
			expected:     `s.k: unknown signature "bogus"`,
		},
		{
			// The output marker does not add a segment to the path.
			name: "in-output-value",
			input: obj(map[string]*structpb.Value{"o": obj(map[string]*structpb.Value{
				sig.Key: structpb.NewStringValue(sig.OutputValue),
				"value": list(bogus),
			})}),
			expectedPath: property.PathFromSegments(key("o"), index(0)),
			expected:     `o[0]: unknown signature "bogus"`,
		},
		{
			name: "missing-secret-value",
			input: list(obj(map[string]*structpb.Value{
				sig.Key: structpb.NewStringValue(sig.Secret),
			})),
			expectedPath: property.PathFromSegments(index(0)),
			expected:     "[0]: malformed secret: missing value",
		},
		{
			name: "non-bool-secret",
			input: obj(map[string]*structpb.Value{"a": obj(map[string]*structpb.Value{"b": obj(
				map[string]*structpb.Value{
					sig.Key:  structpb.NewStringValue(sig.OutputValue),
					"secret": structpb.NewStringValue("true"),
				})})}),
			expectedPath: property.PathFromSegments(key("a"), key("b")),
			expected:     "a.b: malformed output value: secret is not a bool",
		},
		{
			name: "non-array-dependencies",
			input: obj(map[string]*structpb.Value{
				sig.Key:        structpb.NewStringValue(sig.OutputValue),
				"dependencies": structpb.NewStringValue("urn"),
			}),
			expectedPath: property.PathFromSegments(),
			expected:     "<root>: malformed output value: dependencies is not an array",
		},
		{
			name: "non-string-dependency",
			input: list(list(obj(map[string]*structpb.Value{
				sig.Key:        structpb.NewStringValue(sig.OutputValue),
				"dependencies": list(structpb.NewStringValue("urn"), structpb.NewNumberValue(0)),
			}))),
			expectedPath: property.PathFromSegments(index(0), index(0)),
			expected:     "[0][0]: malformed output value: dependency 1 is not a string",
		},
		{
			name: "missing-urn",
			input: obj(map[string]*structpb.Value{"ref": obj(map[string]*structpb.Value{
				sig.Key: structpb.NewStringValue(sig.ResourceReference),
			})}),
			expectedPath: property.PathFromSegments(key("ref")),
			expected:     "ref: malformed resource reference: missing urn",
		},
		{
			name: "non-string-urn",
			input: obj(map[string]*structpb.Value{"ref": obj(map[string]*structpb.Value{
				sig.Key: structpb.NewStringValue(sig.ResourceReference),
				"urn":   structpb.NewBoolValue(true),
			})}),
			expectedPath: property.PathFromSegments(key("ref")),
			expected:     "ref: malformed resource reference: urn is not a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := propertyrpc.UnmarshalValue(tt.input)

			var unmarshalErr propertyrpc.UnmarshalError
			require.ErrorAs(t, err, &unmarshalErr)
			assert.Equal(t, tt.expectedPath, unmarshalErr.Path)
			assert.EqualError(t, err, tt.expected)
		})
	}
}

// TestUnmarshalUnknownSentinel checks that [propertyrpc.Unmarshal] recovers a computed value
// from each unknown sentinel. Peers that marshal without KeepOutputValues send these sentinels
// instead of output values.
func TestUnmarshalUnknownSentinel(t *testing.T) {
	t.Parallel()

	source := resource.PropertyMap{
		"bool":    resource.MakeComputed(resource.NewProperty(false)),
		"number":  resource.MakeComputed(resource.NewProperty(0.0)),
		"string":  resource.MakeComputed(resource.NewProperty("")),
		"array":   resource.MakeComputed(resource.NewProperty([]resource.PropertyValue{})),
		"asset":   resource.MakeComputed(resource.NewProperty(&asset.Asset{})),
		"archive": resource.MakeComputed(resource.NewProperty(&archive.Archive{})),
		"object":  resource.MakeComputed(resource.NewProperty(resource.PropertyMap{})),
	}

	marshalled, err := plugin.MarshalProperties(source, plugin.MarshalOptions{
		KeepUnknowns: true,
		KeepSecrets:  true,
	})
	require.NoError(t, err)

	actual, err := propertyrpc.Unmarshal(marshalled)
	require.NoError(t, err)

	computed := property.New(property.Computed)
	assert.Equal(t, property.NewMap(map[string]property.Value{
		"bool":    computed,
		"number":  computed,
		"string":  computed,
		"array":   computed,
		"asset":   computed,
		"archive": computed,
		"object":  computed,
	}), actual)
}

// TestByteString checks strings that contain bytes which are not valid UTF-8. A protobuf string
// field cannot hold such a string, so it travels as a base64 encoded object.
func TestByteString(t *testing.T) {
	t.Parallel()

	const invalid = "hello\xff\xfeworld"
	source := property.NewMap(map[string]property.Value{
		"bytes": property.New(invalid),
	})

	t.Run("round-trip", func(t *testing.T) {
		t.Parallel()

		// The marshalled value must survive protobuf serialization, which rejects a string
		// field that is not valid UTF-8.
		wire, err := proto.Marshal(propertyrpc.Marshal(source))
		require.NoError(t, err)
		var received structpb.Struct
		require.NoError(t, proto.Unmarshal(wire, &received))

		actual, err := propertyrpc.Unmarshal(&received)
		require.NoError(t, err)
		assert.Equal(t, source, actual)
	})

	t.Run("marshal-through-grpc", func(t *testing.T) {
		t.Parallel()

		unmarshalled, err := plugin.UnmarshalProperties(propertyrpc.Marshal(source), plugin.MarshalOptions{
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		})
		require.NoError(t, err)
		assert.Equal(t, source, resource.FromResourcePropertyMap(unmarshalled))
	})

	t.Run("unmarshal-through-grpc", func(t *testing.T) {
		t.Parallel()

		marshalled, err := plugin.MarshalProperties(resource.ToResourcePropertyMap(source), plugin.MarshalOptions{
			KeepUnknowns:   true,
			KeepSecrets:    true,
			KeepResources:  true,
			KeepByteString: true,
		})
		require.NoError(t, err)

		actual, err := propertyrpc.Unmarshal(marshalled)
		require.NoError(t, err)
		assert.Equal(t, source, actual)
	})
}
