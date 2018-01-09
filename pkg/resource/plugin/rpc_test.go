// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"fmt"
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
	assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", arch.Hash)
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
	assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", archDes.Hash)
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
		assert.True(t, cpropU.ComputedValue().Element.IsString())
	}
	{
		cprop, err := MarshalPropertyValue(
			resource.NewComputedProperty(
				resource.Computed{Element: resource.NewNumberProperty(0)}), opts)
		assert.Nil(t, err)
		cpropU, err := UnmarshalPropertyValue(cprop, opts)
		assert.Nil(t, err)
		assert.True(t, cpropU.IsComputed())
		assert.True(t, cpropU.ComputedValue().Element.IsNumber())
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
