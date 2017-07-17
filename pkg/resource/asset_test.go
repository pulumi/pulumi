// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssetSerialize(t *testing.T) {
	// Ensure that asset and archive serialization round trips.
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
