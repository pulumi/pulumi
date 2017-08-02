// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-fabric/pkg/resource"
)

func TestAssetSerialize(t *testing.T) {
	// Ensure that asset and archive serialization round trips.
	text := "a test asset"
	asset := resource.NewTextAsset(text)
	assetProps := MarshalPropertyValue(resource.NewAssetProperty(asset), MarshalOptions{})
	assetValue := UnmarshalPropertyValue(assetProps, MarshalOptions{})
	assert.True(t, assetValue.IsAsset())
	assetDes := assetValue.AssetValue()
	assert.True(t, assetDes.IsText())
	assert.Equal(t, text, assetDes.Text)

	arch := resource.NewAssetArchive(map[string]resource.Asset{"foo": asset})
	archProps := MarshalPropertyValue(resource.NewArchiveProperty(arch), MarshalOptions{})
	archValue := UnmarshalPropertyValue(archProps, MarshalOptions{})
	assert.True(t, archValue.IsArchive())
	archDes := archValue.ArchiveValue()
	assert.True(t, archDes.IsAssets())
	assert.Equal(t, 1, len(archDes.Assets))
	assert.True(t, archDes.Assets["foo"].IsText())
	assert.Equal(t, text, archDes.Assets["foo"].Text)
}

func TestComputedSerialize(t *testing.T) {
	// Ensure that computed properties survive round trips.
	opts := MarshalOptions{}
	{
		cprop := MarshalPropertyValue(
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")}), opts)
		cpropU := UnmarshalPropertyValue(cprop, opts)
		assert.True(t, cpropU.IsComputed())
		assert.True(t, cpropU.ComputedValue().Element.IsString())
	}
	{
		cprop := MarshalPropertyValue(
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewNumberProperty(0)}), opts)
		cpropU := UnmarshalPropertyValue(cprop, opts)
		assert.True(t, cpropU.IsComputed())
		assert.True(t, cpropU.ComputedValue().Element.IsNumber())
	}
}
