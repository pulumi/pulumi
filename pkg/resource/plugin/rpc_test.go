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

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
)

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
