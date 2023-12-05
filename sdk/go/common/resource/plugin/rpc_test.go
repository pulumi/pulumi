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

package plugin

import (
	"fmt"
	"runtime"
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func setProperty(key resource.PropertyKey, s *structpb.Value, k string, v interface{}) {
	marshaled, err := MarshalPropertyValue(key, resource.NewPropertyValue(v), MarshalOptions{})
	contract.Assertf(err == nil, "error marshaling property value")
	s.GetStructValue().Fields[k] = marshaled
}

func TestAssetSerialize(t *testing.T) {
	t.Parallel()

	// Ensure that asset and archive serialization round trips.
	text := "a test asset"
	pk := resource.PropertyKey("a test asset uri")
	asset, err := resource.NewTextAsset(text)
	assert.NoError(t, err)
	assert.Equal(t, text, asset.Text)
	assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", asset.Hash)
	assetProps, err := MarshalPropertyValue(pk, resource.NewAssetProperty(asset), MarshalOptions{})
	assert.NoError(t, err)
	t.Logf("%v", assetProps)
	assetValue, err := UnmarshalPropertyValue("", assetProps, MarshalOptions{})
	assert.NoError(t, err)
	assert.True(t, assetValue.IsAsset())
	assetDes := assetValue.AssetValue()
	assert.True(t, assetDes.IsText())
	assert.Equal(t, text, assetDes.Text)
	assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", assetDes.Hash)

	// Ensure that an invalid asset produces an error.
	setProperty(pk, assetProps, resource.AssetHashProperty, 0)
	_, err = UnmarshalPropertyValue("", assetProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(pk, assetProps, resource.AssetHashProperty, asset.Hash)

	setProperty(pk, assetProps, resource.AssetTextProperty, 0)
	_, err = UnmarshalPropertyValue("", assetProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(pk, assetProps, resource.AssetTextProperty, "")

	setProperty(pk, assetProps, resource.AssetPathProperty, 0)
	_, err = UnmarshalPropertyValue("", assetProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(pk, assetProps, resource.AssetPathProperty, "")

	setProperty(pk, assetProps, resource.AssetURIProperty, 0)
	_, err = UnmarshalPropertyValue("", assetProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(pk, assetProps, resource.AssetURIProperty, "")

	arch, err := resource.NewAssetArchive(map[string]interface{}{"foo": asset})
	assert.NoError(t, err)
	switch runtime.Version() {
	case "go1.9":
		assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", arch.Hash)
	default:
		// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
		assert.Equal(t, "27ab4a14a617df10cff3e1cf4e30cf510302afe56bf4cc91f84041c9f7b62fd8", arch.Hash)
	}
	archProps, err := MarshalPropertyValue(pk, resource.NewArchiveProperty(arch), MarshalOptions{})
	assert.NoError(t, err)
	archValue, err := UnmarshalPropertyValue("", archProps, MarshalOptions{})
	assert.NoError(t, err)
	assert.True(t, archValue.IsArchive())
	archDes := archValue.ArchiveValue()
	assert.True(t, archDes.IsAssets())
	assert.Equal(t, 1, len(archDes.Assets))
	assert.True(t, archDes.Assets["foo"].(*resource.Asset).IsText())
	assert.Equal(t, text, archDes.Assets["foo"].(*resource.Asset).Text)
	switch runtime.Version() {
	case "go1.9":
		assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", archDes.Hash)
	default:
		// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
		assert.Equal(t, "27ab4a14a617df10cff3e1cf4e30cf510302afe56bf4cc91f84041c9f7b62fd8", archDes.Hash)
	}

	// Ensure that an invalid archive produces an error.
	setProperty(pk, archProps, resource.ArchiveHashProperty, 0)
	_, err = UnmarshalPropertyValue("", archProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(pk, archProps, resource.ArchiveHashProperty, arch.Hash)

	setProperty(pk, archProps, resource.ArchiveAssetsProperty, 0)
	_, err = UnmarshalPropertyValue("", archProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(pk, archProps, resource.ArchiveAssetsProperty, nil)

	setProperty(pk, archProps, resource.ArchivePathProperty, 0)
	_, err = UnmarshalPropertyValue("", archProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(pk, archProps, resource.ArchivePathProperty, "")

	setProperty(pk, archProps, resource.ArchiveURIProperty, 0)
	_, err = UnmarshalPropertyValue("", archProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(pk, archProps, resource.ArchiveURIProperty, "")
}

func TestComputedSerialize(t *testing.T) {
	t.Parallel()

	// Ensure that computed properties survive round trips.
	opts := MarshalOptions{KeepUnknowns: true}
	pk := resource.PropertyKey("pk")
	{
		cprop, err := MarshalPropertyValue(pk,
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")}), opts)
		assert.NoError(t, err)
		cpropU, err := UnmarshalPropertyValue(pk, cprop, opts)
		assert.NoError(t, err)
		assert.True(t, cpropU.IsComputed())
		assert.True(t, cpropU.Input().Element.IsString())
	}
	{
		cprop, err := MarshalPropertyValue(pk,
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewNumberProperty(0)}), opts)
		assert.NoError(t, err)
		cpropU, err := UnmarshalPropertyValue(pk, cprop, opts)
		assert.NoError(t, err)
		assert.True(t, cpropU.IsComputed())
		assert.True(t, cpropU.Input().Element.IsNumber())
	}
}

func TestComputedSkip(t *testing.T) {
	t.Parallel()

	// Ensure that computed properties are skipped when KeepUnknowns == false.
	opts := MarshalOptions{KeepUnknowns: false}
	pk := resource.PropertyKey("pk")
	{
		cprop, err := MarshalPropertyValue(pk,
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")}), opts)
		assert.NoError(t, err)
		assert.Nil(t, cprop)
	}
	{
		cprop, err := MarshalPropertyValue(pk,
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewNumberProperty(0)}), opts)
		assert.NoError(t, err)
		assert.Nil(t, cprop)
	}
}

func TestComputedReject(t *testing.T) {
	t.Parallel()

	// Ensure that computed properties produce errors when RejectUnknowns == true.
	opts := MarshalOptions{RejectUnknowns: true}
	pk := resource.PropertyKey("pk")
	{
		cprop, err := MarshalPropertyValue(pk,
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")}), opts)
		assert.Error(t, err)
		assert.Nil(t, cprop)
	}
	{
		cprop, err := MarshalPropertyValue(pk,
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")}), MarshalOptions{KeepUnknowns: true})
		assert.NoError(t, err)
		cpropU, err := UnmarshalPropertyValue(pk, cprop, opts)
		assert.Error(t, err)
		assert.Nil(t, cpropU)
	}
}

func TestAssetReject(t *testing.T) {
	t.Parallel()

	// Ensure that asset and archive properties produce errors when RejectAssets == true.

	opts := MarshalOptions{RejectAssets: true}

	text := "a test asset"
	pk := resource.PropertyKey("an asset URI")
	asset, err := resource.NewTextAsset(text)
	assert.NoError(t, err)
	{
		assetProps, err := MarshalPropertyValue(pk, resource.NewAssetProperty(asset), opts)
		assert.Error(t, err)
		assert.Nil(t, assetProps)
	}
	{
		assetProps, err := MarshalPropertyValue(pk, resource.NewAssetProperty(asset), MarshalOptions{})
		assert.NoError(t, err)
		assetPropU, err := UnmarshalPropertyValue(pk, assetProps, opts)
		assert.Error(t, err)
		assert.Nil(t, assetPropU)
	}

	arch, err := resource.NewAssetArchive(map[string]interface{}{"foo": asset})
	assert.NoError(t, err)
	{
		archProps, err := MarshalPropertyValue(pk, resource.NewArchiveProperty(arch), opts)
		assert.Error(t, err)
		assert.Nil(t, archProps)
	}
	{
		archProps, err := MarshalPropertyValue(pk, resource.NewArchiveProperty(arch), MarshalOptions{})
		assert.NoError(t, err)
		archValue, err := UnmarshalPropertyValue(pk, archProps, opts)
		assert.Error(t, err)
		assert.Nil(t, archValue)
	}
}

func TestUnsupportedSecret(t *testing.T) {
	t.Parallel()

	rawProp := resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
		resource.SigKey: resource.SecretSig,
		"value":         "foo",
	}))
	pk := resource.PropertyKey("pk")
	prop, err := MarshalPropertyValue(pk, rawProp, MarshalOptions{})
	assert.NoError(t, err)
	val, err := UnmarshalPropertyValue(pk, prop, MarshalOptions{})
	assert.NoError(t, err)
	assert.True(t, val.IsString())
	assert.False(t, val.IsSecret())
	assert.Equal(t, "foo", val.StringValue())
}

func TestSupportedSecret(t *testing.T) {
	t.Parallel()

	rawProp := resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
		resource.SigKey: resource.SecretSig,
		"value":         "foo",
	}))
	pk := resource.PropertyKey("pk")

	prop, err := MarshalPropertyValue(pk, rawProp, MarshalOptions{KeepSecrets: true})
	assert.NoError(t, err)
	val, err := UnmarshalPropertyValue(pk, prop, MarshalOptions{KeepSecrets: true})
	assert.NoError(t, err)
	assert.False(t, val.IsString())
	assert.True(t, val.IsSecret())
	assert.Equal(t, "foo", val.SecretValue().Element.StringValue())
}

func TestUnknownSig(t *testing.T) {
	t.Parallel()

	rawProp := resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
		resource.SigKey: "foobar",
	}))
	pk := resource.PropertyKey("pk")

	prop, err := MarshalPropertyValue(pk, rawProp, MarshalOptions{})
	assert.NoError(t, err)
	_, err = UnmarshalPropertyValue(pk, prop, MarshalOptions{})
	assert.Error(t, err)
}

func TestSkipInternalKeys(t *testing.T) {
	t.Parallel()

	opts := MarshalOptions{SkipInternalKeys: true}
	expected := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"keepers": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{},
					},
				},
			},
		},
	}
	props := resource.NewPropertyMapFromMap(map[string]interface{}{
		"__defaults": []string{},
		"keepers": map[string]interface{}{
			"__defaults": []string{},
		},
	})
	actual, err := MarshalProperties(props, opts)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestMarshalProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     MarshalOptions
		props    resource.PropertyMap
		expected *structpb.Struct
	}{
		{
			name: "empty (default)",
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{},
			},
		},
		{
			name: "unknown (default)",
			props: resource.PropertyMap{
				"foo": resource.MakeOutput(resource.NewStringProperty("")),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{},
			},
		},
		{
			name: "unknown with deps (default)",
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty(""),
					Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{},
			},
		},
		{
			name: "known (default)",
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("hello"),
					Known:   true,
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StringValue{
							StringValue: "hello",
						},
					},
				},
			},
		},
		{
			name: "known with deps (default)",
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("hello"),
					Known:        true,
					Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StringValue{
							StringValue: "hello",
						},
					},
				},
			},
		},
		{
			name: "secret (default)",
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("hello"),
					Known:   true,
					Secret:  true,
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StringValue{
							StringValue: "hello",
						},
					},
				},
			},
		},
		{
			name: "secret with deps (default)",
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("hello"),
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StringValue{
							StringValue: "hello",
						},
					},
				},
			},
		},
		{
			name: "unknown secret (default)",
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("shhh"),
					Secret:  true,
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{},
			},
		},
		{
			name: "unknown secret with deps (default)",
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("shhh"),
					Secret:       true,
					Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{},
			},
		},
		{
			name: "empty (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "unknown (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			props: resource.PropertyMap{
				"foo": resource.MakeOutput(resource.NewStringProperty("")),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "unknown with deps (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty(""),
					Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
									"dependencies": {
										Kind: &structpb.Value_ListValue{
											ListValue: &structpb.ListValue{
												Values: []*structpb.Value{
													{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
													{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "known (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("hello"),
					Known:   true,
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
									"value": {
										Kind: &structpb.Value_StringValue{StringValue: "hello"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "known with deps (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("hello"),
					Known:        true,
					Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
									"value": {
										Kind: &structpb.Value_StringValue{StringValue: "hello"},
									},
									"dependencies": {
										Kind: &structpb.Value_ListValue{
											ListValue: &structpb.ListValue{
												Values: []*structpb.Value{
													{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
													{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "secret (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("shhh"),
					Known:   true,
					Secret:  true,
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
									"value": {
										Kind: &structpb.Value_StringValue{StringValue: "shhh"},
									},
									"secret": {
										Kind: &structpb.Value_BoolValue{BoolValue: true},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "secret with deps (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("shhh"),
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
									"value": {
										Kind: &structpb.Value_StringValue{StringValue: "shhh"},
									},
									"secret": {
										Kind: &structpb.Value_BoolValue{BoolValue: true},
									},
									"dependencies": {
										Kind: &structpb.Value_ListValue{
											ListValue: &structpb.ListValue{
												Values: []*structpb.Value{
													{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
													{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "unknown secret (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("shhh"),
					Secret:  true,
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
									"secret": {
										Kind: &structpb.Value_BoolValue{BoolValue: true},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "unknown secret with deps (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			props: resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("shhh"),
					Secret:       true,
					Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
									"secret": {
										Kind: &structpb.Value_BoolValue{BoolValue: true},
									},
									"dependencies": {
										Kind: &structpb.Value_ListValue{
											ListValue: &structpb.ListValue{
												Values: []*structpb.Value{
													{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
													{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := MarshalProperties(tt.props, tt.opts)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestResourceReference(t *testing.T) {
	t.Parallel()

	// Test round-trip
	opts := MarshalOptions{KeepResources: true}
	rawProp := resource.MakeCustomResourceReference("fakeURN", "fakeID", "fakeVersion")
	pk := resource.PropertyKey("pk")
	prop, err := MarshalPropertyValue(pk, rawProp, opts)
	assert.NoError(t, err)
	actual, err := UnmarshalPropertyValue(pk, prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, rawProp, *actual)

	// Test unmarshaling as an ID
	opts.KeepResources = false
	actual, err = UnmarshalPropertyValue(pk, prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty(rawProp.ResourceReferenceValue().ID.StringValue()), *actual)

	// Test marshaling as an ID
	prop, err = MarshalPropertyValue(pk, rawProp, opts)
	assert.NoError(t, err)
	opts.KeepResources = true
	actual, err = UnmarshalPropertyValue(pk, prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty(rawProp.ResourceReferenceValue().ID.StringValue()), *actual)

	// Test unmarshaling as a URN
	rawProp = resource.MakeComponentResourceReference("fakeURN", "fakeVersion")
	prop, err = MarshalPropertyValue(pk, rawProp, opts)
	assert.NoError(t, err)
	opts.KeepResources = false
	actual, err = UnmarshalPropertyValue(pk, prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty(string(rawProp.ResourceReferenceValue().URN)), *actual)

	// Test marshaling as a URN
	prop, err = MarshalPropertyValue(pk, rawProp, opts)
	assert.NoError(t, err)
	opts.KeepResources = true
	actual, err = UnmarshalPropertyValue(pk, prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty(string(rawProp.ResourceReferenceValue().URN)), *actual)
}

func TestOutputValueRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  resource.PropertyValue
	}{
		{
			name: "unknown",
			raw:  resource.NewOutputProperty(resource.Output{}),
		},
		{
			name: "unknown with deps",
			raw: resource.NewOutputProperty(resource.Output{
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
		},
		{
			name: "known",
			raw: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("hello"),
				Known:   true,
			}),
		},
		{
			name: "known with deps",
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("hello"),
				Known:        true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
		},
		{
			name: "secret",
			raw: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("secret"),
				Known:   true,
				Secret:  true,
			}),
		},
		{
			name: "secret with deps",
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("secret"),
				Known:        true,
				Secret:       true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
		},
		{
			name: "unknown secret",
			raw: resource.NewOutputProperty(resource.Output{
				Secret: true,
			}),
		},
		{
			name: "unknown secret with deps",
			raw: resource.NewOutputProperty(resource.Output{
				Secret:       true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := MarshalOptions{KeepOutputValues: true}
			prop, err := MarshalPropertyValue("", tt.raw, opts)
			assert.NoError(t, err)
			actual, err := UnmarshalPropertyValue("", prop, opts)
			assert.NoError(t, err)
			assert.Equal(t, tt.raw, *actual)
		})
	}
}

func TestOutputValueMarshaling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     MarshalOptions
		raw      resource.PropertyValue
		expected *structpb.Value
	}{
		{
			name:     "empty (default)",
			raw:      resource.NewOutputProperty(resource.Output{}),
			expected: nil,
		},
		{
			name:     "unknown (default)",
			raw:      resource.MakeOutput(resource.NewStringProperty("")),
			expected: nil,
		},
		{
			name: "unknown with deps (default)",
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("hello"),
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
			expected: nil,
		},
		{
			name: "known (default)",
			raw: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("hello"),
				Known:   true,
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: "hello"},
			},
		},
		{
			name: "known with deps (default)",
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("hello"),
				Known:        true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: "hello"},
			},
		},
		{
			name: "secret (default)",
			raw: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("shhh"),
				Known:   true,
				Secret:  true,
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: "shhh"},
			},
		},
		{
			name: "secret with deps (default)",
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("shhh"),
				Known:        true,
				Secret:       true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: "shhh"},
			},
		},
		{
			name: "unknown secret (default)",
			raw: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("shhh"),
				Secret:  true,
			}),
			expected: nil,
		},
		{
			name: "unknown secret with deps (default)",
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("shhh"),
				Secret:       true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
			expected: nil,
		},
		{
			name: "empty (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw:  resource.NewOutputProperty(resource.Output{}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
			},
		},
		{
			name: "unknown (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw:  resource.MakeOutput(resource.NewStringProperty("")),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
			},
		},
		{
			name: "unknown with deps (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("hello"),
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
			},
		},
		{
			name: "unknown secret (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("shhh"),
				Secret:  true,
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
			},
		},
		{
			name: "unknown secret with deps (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("shhh"),
				Secret:       true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
			},
		},
		{
			name: "secret (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("shhh"),
				Known:   true,
				Secret:  true,
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: "shhh"},
			},
		},
		{
			name: "secret with deps (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("shhh"),
				Known:        true,
				Secret:       true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StringValue{StringValue: "shhh"},
			},
		},
		{
			name: "secret (KeepSecrets)",
			opts: MarshalOptions{KeepSecrets: true},
			raw: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("shhh"),
				Known:   true,
				Secret:  true,
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.SecretSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "shhh"},
							},
						},
					},
				},
			},
		},
		{
			name: "secret with deps (KeepSecrets)",
			opts: MarshalOptions{KeepSecrets: true},
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("shhh"),
				Known:        true,
				Secret:       true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.SecretSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "shhh"},
							},
						},
					},
				},
			},
		},
		{
			name: "unknown secret (KeepUnknowns, KeepSecrets)",
			opts: MarshalOptions{KeepUnknowns: true, KeepSecrets: true},
			raw: resource.NewOutputProperty(resource.Output{
				Element: resource.NewStringProperty("shhh"),
				Secret:  true,
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.SecretSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
							},
						},
					},
				},
			},
		},
		{
			name: "unknown secret with deps (KeepUnknowns, KeepSecrets)",
			opts: MarshalOptions{KeepUnknowns: true, KeepSecrets: true},
			raw: resource.NewOutputProperty(resource.Output{
				Element:      resource.NewStringProperty("shhh"),
				Secret:       true,
				Dependencies: []resource.URN{"fakeURN1", "fakeURN2"},
			}),
			expected: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.SecretSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
							},
						},
					},
				},
			},
		},
		{
			name: "unknown value (KeepOutputValues)",
			opts: MarshalOptions{KeepOutputValues: true},
			raw:  resource.MakeOutput(resource.NewStringProperty("")),
			expected: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := MarshalPropertyValue("", tt.raw, tt.opts)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestOutputValueUnmarshaling(t *testing.T) {
	t.Parallel()

	ptr := func(v resource.PropertyValue) *resource.PropertyValue {
		return &v
	}
	tests := []struct {
		name     string
		opts     MarshalOptions
		raw      *structpb.Value
		expected *resource.PropertyValue
	}{
		{
			name: "unknown (default)",
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "known (default)",
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "hello"},
							},
						},
					},
				},
			},
			expected: ptr(resource.NewStringProperty("hello")),
		},
		{
			name: "known with deps (default)",
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "hello"},
							},
							"dependencies": {
								Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: ptr(resource.NewStringProperty("hello")),
		},
		{
			name: "secret (default)",
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "shhh"},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
						},
					},
				},
			},
			expected: ptr(resource.NewStringProperty("shhh")),
		},
		{
			name: "secret with deps (default)",
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "shhh"},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
							"dependencies": {
								Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: ptr(resource.NewStringProperty("shhh")),
		},
		{
			name: "unknown secret (default)",
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "unknown secret with deps (default)",
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
							"dependencies": {
								Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "unknown (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
						},
					},
				},
			},
			expected: ptr(resource.MakeComputed(resource.NewStringProperty(""))),
		},
		{
			name: "unknown with deps (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"dependencies": {
								Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: ptr(resource.MakeComputed(resource.NewStringProperty(""))),
		},
		{
			name: "unknown secret (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
						},
					},
				},
			},
			expected: ptr(resource.MakeComputed(resource.NewStringProperty(""))),
		},
		{
			name: "unknown secret with deps (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
							"dependencies": {
								Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: ptr(resource.MakeComputed(resource.NewStringProperty(""))),
		},
		{
			name: "secret (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "shhh"},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
						},
					},
				},
			},
			expected: ptr(resource.NewStringProperty("shhh")),
		},
		{
			name: "secret with deps (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "shhh"},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
							"dependencies": {
								Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: ptr(resource.NewStringProperty("shhh")),
		},
		{
			name: "secret (KeepSecrets)",
			opts: MarshalOptions{KeepSecrets: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "shhh"},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
						},
					},
				},
			},
			expected: ptr(resource.MakeSecret(resource.NewStringProperty("shhh"))),
		},
		{
			name: "secret with deps (KeepSecrets)",
			opts: MarshalOptions{KeepSecrets: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"value": {
								Kind: &structpb.Value_StringValue{StringValue: "shhh"},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
							"dependencies": {
								Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: ptr(resource.MakeSecret(resource.NewStringProperty("shhh"))),
		},
		{
			name: "unknown secret (KeepUnknowns, KeepSecrets)",
			opts: MarshalOptions{KeepUnknowns: true, KeepSecrets: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
						},
					},
				},
			},
			expected: ptr(resource.MakeSecret(resource.MakeComputed(resource.NewStringProperty("")))),
		},
		{
			name: "unknown secret with deps (KeepUnknowns, KeepSecrets)",
			opts: MarshalOptions{KeepUnknowns: true, KeepSecrets: true},
			raw: &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: {
								Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
							},
							"secret": {
								Kind: &structpb.Value_BoolValue{BoolValue: true},
							},
							"dependencies": {
								Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN1"}},
											{Kind: &structpb.Value_StringValue{StringValue: "fakeURN2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: ptr(resource.MakeSecret(resource.MakeComputed(resource.NewStringProperty("")))),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prop, err := UnmarshalPropertyValue("", tt.raw, tt.opts)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, prop)
		})
	}
}

func TestMarshalPropertiesDiscardsListNestedUnknowns(t *testing.T) {
	t.Parallel()

	proto := pbStruct("resources", pbList(pbString(UnknownStringValue)))

	propsWithUnknowns, err := UnmarshalProperties(proto, MarshalOptions{KeepUnknowns: true})
	if err != nil {
		t.Errorf("UnmarshalProperties failed: %v", err)
	}

	protobufStruct, err := MarshalProperties(propsWithUnknowns, MarshalOptions{KeepUnknowns: false})
	if err != nil {
		t.Error(err)
		return
	}

	protobufValue := &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: protobufStruct,
		},
	}

	assertValidProtobufValue(t, protobufValue)
}

func pbString(s string) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: s,
		},
	}
}

func pbStruct(name string, value *structpb.Value) *structpb.Struct {
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			name: value,
		},
	}
}

func pbList(elems ...*structpb.Value) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_ListValue{
			ListValue: &structpb.ListValue{Values: elems},
		},
	}
}

func assertValidProtobufValue(t *testing.T, value *structpb.Value) {
	err := walkValueSelfWithDescendants(value, "", func(path string, v *structpb.Value) error {
		return nil
	})
	if err != nil {
		t.Errorf("This is not a valid *structpb.Value: %v\n%v", value, err)
	}
}

func walkValueSelfWithDescendants(
	v *structpb.Value,
	path string,
	visit func(path string, v *structpb.Value) error,
) error {
	if v == nil {
		return fmt.Errorf("bad *structpb.Value nil at %s", path)
	}
	err := visit(path, v)
	if err != nil {
		return err
	}
	switch v.Kind.(type) {
	case *structpb.Value_ListValue:
		values := v.GetListValue().GetValues()
		for i, v := range values {
			err = walkValueSelfWithDescendants(v, fmt.Sprintf("%s[%d]", path, i), visit)
			if err != nil {
				return err
			}
		}
	case *structpb.Value_StructValue:
		s := v.GetStructValue()
		for fn, fv := range s.Fields {
			err = walkValueSelfWithDescendants(fv, fmt.Sprintf(`%s["%s"]`, path, fn), visit)
			if err != nil {
				return err
			}
		}
	case *structpb.Value_NumberValue:
		return nil
	case *structpb.Value_StringValue:
		return nil
	case *structpb.Value_BoolValue:
		return nil
	case *structpb.Value_NullValue:
		return nil
	default:
		return fmt.Errorf("bad *structpb.Value of unknown type at %s: %v", path, v)
	}
	return nil
}

func TestMarshalPropertiesDontSkipOutputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     MarshalOptions
		props    resource.PropertyMap
		expected *structpb.Struct
	}{
		{
			name: "Computed (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			props: resource.PropertyMap{
				"message": resource.MakeComputed(resource.NewStringProperty("")),
				"nested": resource.NewObjectProperty(resource.PropertyMap{
					"value": resource.MakeComputed(resource.NewStringProperty("")),
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"message": {
						Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
					},
					"nested": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"value": {
										Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Output (KeepUnknowns)",
			opts: MarshalOptions{KeepUnknowns: true},
			props: resource.PropertyMap{
				"message": resource.NewOutputProperty(resource.Output{}),
				"nested": resource.NewObjectProperty(resource.PropertyMap{
					"value": resource.NewOutputProperty(resource.Output{}),
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"message": {
						Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
					},
					"nested": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"value": {
										Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Output (KeepUnknowns, KeepOutputValues)",
			opts: MarshalOptions{KeepUnknowns: true, KeepOutputValues: true},
			props: resource.PropertyMap{
				"message": resource.NewOutputProperty(resource.Output{}),
				"nested": resource.NewObjectProperty(resource.PropertyMap{
					"value": resource.NewOutputProperty(resource.Output{}),
				}),
			},
			expected: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"message": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									resource.SigKey: {
										Kind: &structpb.Value_StringValue{StringValue: resource.OutputValueSig},
									},
								},
							},
						},
					},
					"nested": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"value": {
										Kind: &structpb.Value_StructValue{
											StructValue: &structpb.Struct{
												Fields: map[string]*structpb.Value{
													resource.SigKey: {
														Kind: &structpb.Value_StringValue{
															StringValue: resource.OutputValueSig,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := MarshalProperties(tt.props, tt.opts)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
