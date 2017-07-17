// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssetSerialize(t *testing.T) {
	// Ensure that asset and archive serialization round trips.
	{
		text := "a test asset"
		asset := NewTextAsset(text)
		assetSer := asset.Serialize()
		assetDes, isasset := DeserializeAsset(assetSer)
		assert.True(t, isasset)
		assert.True(t, assetDes.IsText())
		assert.Equal(t, text, assetDes.Text)

		arch := NewAssetArchive(map[string]Asset{"foo": asset})
		archSer := arch.Serialize()
		archDes, isarch := DeserializeArchive(archSer)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].IsText())
		assert.Equal(t, text, archDes.Assets["foo"].Text)
	}
	{
		file := "/dev/null"
		asset := NewPathAsset(file)
		assetSer := asset.Serialize()
		assetDes, isasset := DeserializeAsset(assetSer)
		assert.True(t, isasset)
		assert.True(t, assetDes.IsPath())
		assert.Equal(t, file, assetDes.Path)

		arch := NewAssetArchive(map[string]Asset{"foo": asset})
		archSer := arch.Serialize()
		archDes, isarch := DeserializeArchive(archSer)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].IsPath())
		assert.Equal(t, file, archDes.Assets["foo"].Path)
	}
	{
		url := "https://pulumi.com/robots.txt"
		asset := NewURIAsset(url)
		assetSer := asset.Serialize()
		assetDes, isasset := DeserializeAsset(assetSer)
		assert.True(t, isasset)
		assert.True(t, assetDes.IsURI())
		assert.Equal(t, url, assetDes.URI)

		arch := NewAssetArchive(map[string]Asset{"foo": asset})
		archSer := arch.Serialize()
		archDes, isarch := DeserializeArchive(archSer)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].IsURI())
		assert.Equal(t, url, archDes.Assets["foo"].URI)
	}
	{
		file := "/dev/null"
		arch := NewPathArchive(file)
		archSer := arch.Serialize()
		archDes, isarch := DeserializeArchive(archSer)
		assert.True(t, isarch)
		assert.True(t, archDes.IsPath())
		assert.Equal(t, file, archDes.Path)
	}
	{
		url := "https://pulumi.com/robots.tgz"
		arch := NewURIArchive(url)
		archSer := arch.Serialize()
		archDes, isarch := DeserializeArchive(archSer)
		assert.True(t, isarch)
		assert.True(t, archDes.IsURI())
		assert.Equal(t, url, archDes.URI)
	}
}
