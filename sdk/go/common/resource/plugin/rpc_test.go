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

func setProperty(s *structpb.Value, k string, v interface{}) {
	marshaled, err := MarshalPropertyValue(resource.NewPropertyValue(v), MarshalOptions{})
	contract.Assert(err == nil)
	s.GetStructValue().Fields[k] = marshaled
}

func TestAssetSerialize(t *testing.T) {
	// Ensure that asset and archive serialization round trips.
	text := "a test asset"
	asset, err := resource.NewTextAsset(text)
	assert.Nil(t, err)
	assert.Equal(t, text, asset.Text)
	assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", asset.Hash)
	assetProps, err := MarshalPropertyValue(resource.NewAssetProperty(asset), MarshalOptions{})
	assert.Nil(t, err)
	fmt.Printf("%v\n", assetProps)
	assetValue, err := UnmarshalPropertyValue(assetProps, MarshalOptions{})
	assert.Nil(t, err)
	assert.True(t, assetValue.IsAsset())
	assetDes := assetValue.AssetValue()
	assert.True(t, assetDes.IsText())
	assert.Equal(t, text, assetDes.Text)
	assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", assetDes.Hash)

	// Ensure that an invalid asset produces an error.
	setProperty(assetProps, resource.AssetHashProperty, 0)
	_, err = UnmarshalPropertyValue(assetProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(assetProps, resource.AssetHashProperty, asset.Hash)

	setProperty(assetProps, resource.AssetTextProperty, 0)
	_, err = UnmarshalPropertyValue(assetProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(assetProps, resource.AssetTextProperty, "")

	setProperty(assetProps, resource.AssetPathProperty, 0)
	_, err = UnmarshalPropertyValue(assetProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(assetProps, resource.AssetPathProperty, "")

	setProperty(assetProps, resource.AssetURIProperty, 0)
	_, err = UnmarshalPropertyValue(assetProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(assetProps, resource.AssetURIProperty, "")

	arch, err := resource.NewAssetArchive(map[string]interface{}{"foo": asset})
	assert.Nil(t, err)
	switch runtime.Version() {
	case "go1.9":
		assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", arch.Hash)
	default:
		// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
		assert.Equal(t, "27ab4a14a617df10cff3e1cf4e30cf510302afe56bf4cc91f84041c9f7b62fd8", arch.Hash)
	}
	archProps, err := MarshalPropertyValue(resource.NewArchiveProperty(arch), MarshalOptions{})
	assert.Nil(t, err)
	archValue, err := UnmarshalPropertyValue(archProps, MarshalOptions{})
	assert.Nil(t, err)
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
	setProperty(archProps, resource.ArchiveHashProperty, 0)
	_, err = UnmarshalPropertyValue(archProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(archProps, resource.ArchiveHashProperty, arch.Hash)

	setProperty(archProps, resource.ArchiveAssetsProperty, 0)
	_, err = UnmarshalPropertyValue(archProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(archProps, resource.ArchiveAssetsProperty, nil)

	setProperty(archProps, resource.ArchivePathProperty, 0)
	_, err = UnmarshalPropertyValue(archProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(archProps, resource.ArchivePathProperty, "")

	setProperty(archProps, resource.ArchiveURIProperty, 0)
	_, err = UnmarshalPropertyValue(archProps, MarshalOptions{})
	assert.Error(t, err)
	setProperty(archProps, resource.ArchiveURIProperty, "")
}

func TestComputedSerialize(t *testing.T) {
	// Ensure that computed properties survive round trips.
	opts := MarshalOptions{KeepUnknowns: true}
	{
		cprop, err := MarshalPropertyValue(
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")}), opts)
		assert.Nil(t, err)
		cpropU, err := UnmarshalPropertyValue(cprop, opts)
		assert.Nil(t, err)
		assert.True(t, cpropU.IsComputed())
		assert.True(t, cpropU.Input().Element.IsString())
	}
	{
		cprop, err := MarshalPropertyValue(
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewNumberProperty(0)}), opts)
		assert.Nil(t, err)
		cpropU, err := UnmarshalPropertyValue(cprop, opts)
		assert.Nil(t, err)
		assert.True(t, cpropU.IsComputed())
		assert.True(t, cpropU.Input().Element.IsNumber())
	}
}

func TestComputedSkip(t *testing.T) {
	// Ensure that computed properties are skipped when KeepUnknowns == false.
	opts := MarshalOptions{KeepUnknowns: false}
	{
		cprop, err := MarshalPropertyValue(
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")}), opts)
		assert.Nil(t, err)
		assert.Nil(t, cprop)
	}
	{
		cprop, err := MarshalPropertyValue(
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewNumberProperty(0)}), opts)
		assert.Nil(t, err)
		assert.Nil(t, cprop)
	}
}

func TestComputedReject(t *testing.T) {
	// Ensure that computed properties produce errors when RejectUnknowns == true.
	opts := MarshalOptions{RejectUnknowns: true}
	{
		cprop, err := MarshalPropertyValue(
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")}), opts)
		assert.NotNil(t, err)
		assert.Nil(t, cprop)
	}
	{
		cprop, err := MarshalPropertyValue(
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")}), MarshalOptions{KeepUnknowns: true})
		assert.Nil(t, err)
		cpropU, err := UnmarshalPropertyValue(cprop, opts)
		assert.NotNil(t, err)
		assert.Nil(t, cpropU)
	}
}

func TestAssetReject(t *testing.T) {
	// Ensure that asset and archive properties produce errors when RejectAssets == true.

	opts := MarshalOptions{RejectAssets: true}

	text := "a test asset"
	asset, err := resource.NewTextAsset(text)
	assert.Nil(t, err)
	{
		assetProps, err := MarshalPropertyValue(resource.NewAssetProperty(asset), opts)
		assert.NotNil(t, err)
		assert.Nil(t, assetProps)
	}
	{
		assetProps, err := MarshalPropertyValue(resource.NewAssetProperty(asset), MarshalOptions{})
		assert.Nil(t, err)
		assetPropU, err := UnmarshalPropertyValue(assetProps, opts)
		assert.NotNil(t, err)
		assert.Nil(t, assetPropU)
	}

	arch, err := resource.NewAssetArchive(map[string]interface{}{"foo": asset})
	assert.Nil(t, err)
	{
		archProps, err := MarshalPropertyValue(resource.NewArchiveProperty(arch), opts)
		assert.NotNil(t, err)
		assert.Nil(t, archProps)
	}
	{
		archProps, err := MarshalPropertyValue(resource.NewArchiveProperty(arch), MarshalOptions{})
		assert.Nil(t, err)
		archValue, err := UnmarshalPropertyValue(archProps, opts)
		assert.NotNil(t, err)
		assert.Nil(t, archValue)
	}

}

func TestUnsupportedSecret(t *testing.T) {
	rawProp := resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
		resource.SigKey: resource.SecretSig,
		"value":         "foo",
	}))
	prop, err := MarshalPropertyValue(rawProp, MarshalOptions{})
	assert.Nil(t, err)
	val, err := UnmarshalPropertyValue(prop, MarshalOptions{})
	assert.Nil(t, err)
	assert.True(t, val.IsString())
	assert.False(t, val.IsSecret())
	assert.Equal(t, "foo", val.StringValue())
}

func TestSupportedSecret(t *testing.T) {
	rawProp := resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
		resource.SigKey: resource.SecretSig,
		"value":         "foo",
	}))
	prop, err := MarshalPropertyValue(rawProp, MarshalOptions{KeepSecrets: true})
	assert.Nil(t, err)
	val, err := UnmarshalPropertyValue(prop, MarshalOptions{KeepSecrets: true})
	assert.Nil(t, err)
	assert.False(t, val.IsString())
	assert.True(t, val.IsSecret())
	assert.Equal(t, "foo", val.SecretValue().Element.StringValue())
}

func TestUnknownSig(t *testing.T) {
	rawProp := resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
		resource.SigKey: "foobar",
	}))
	prop, err := MarshalPropertyValue(rawProp, MarshalOptions{})
	assert.Nil(t, err)
	_, err = UnmarshalPropertyValue(prop, MarshalOptions{})
	assert.Error(t, err)

}

func TestSkipInternalKeys(t *testing.T) {
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

func TestResourceReference(t *testing.T) {
	// Test round-trip
	opts := MarshalOptions{KeepResources: true}
	rawProp := resource.MakeCustomResourceReference("fakeURN", "fakeID", "fakeVersion")
	prop, err := MarshalPropertyValue(rawProp, opts)
	assert.NoError(t, err)
	actual, err := UnmarshalPropertyValue(prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, rawProp, *actual)

	// Test unmarshaling as an ID
	opts.KeepResources = false
	actual, err = UnmarshalPropertyValue(prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty(rawProp.ResourceReferenceValue().ID.StringValue()), *actual)

	// Test marshaling as an ID
	prop, err = MarshalPropertyValue(rawProp, opts)
	assert.NoError(t, err)
	opts.KeepResources = true
	actual, err = UnmarshalPropertyValue(prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty(rawProp.ResourceReferenceValue().ID.StringValue()), *actual)

	// Test unmarshaling as a URN
	rawProp = resource.MakeComponentResourceReference("fakeURN", "fakeVersion")
	prop, err = MarshalPropertyValue(rawProp, opts)
	assert.NoError(t, err)
	opts.KeepResources = false
	actual, err = UnmarshalPropertyValue(prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty(string(rawProp.ResourceReferenceValue().URN)), *actual)

	// Test marshaling as a URN
	prop, err = MarshalPropertyValue(rawProp, opts)
	assert.NoError(t, err)
	opts.KeepResources = true
	actual, err = UnmarshalPropertyValue(prop, opts)
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty(string(rawProp.ResourceReferenceValue().URN)), *actual)
}

func TestMarshalPropertiesDiscardsListNestedUnknowns(t *testing.T) {
	proto := pbStruct("resources", pbList(pbString(UnknownStringValue)))

	propsWithUnknowns, err := UnmarshalProperties(proto, MarshalOptions{KeepUnknowns: true})
	if err != nil {
		t.Errorf("UnmarhsalProperties failed: %w", err)
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
		t.Errorf("This is not a valid *structpb.Value: %v\n%w", value, err)
	}
}

func walkValueSelfWithDescendants(
	v *structpb.Value,
	path string,
	visit func(path string, v *structpb.Value) error) error {

	if v == nil {
		return fmt.Errorf("Bad *structpb.Value nil at %s", path)
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
		return fmt.Errorf("Bad *structpb.Value of unknown type at %s: %v", path, v)
	}
	return nil
}
