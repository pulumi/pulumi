// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"archive/tar"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

func TestAssetSerialize(t *testing.T) {
	// Ensure that asset and archive serialization round trips.
	{
		text := "a test asset"
		asset, err := NewTextAsset(text)
		assert.Nil(t, err)
		assert.Equal(t, text, asset.Text)
		assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", asset.Hash)
		assetSer := asset.Serialize()
		assetDes, isasset, err := DeserializeAsset(assetSer)
		assert.Nil(t, err)
		assert.True(t, isasset)
		assert.True(t, assetDes.IsText())
		assert.Equal(t, text, assetDes.Text)
		assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", assetDes.Hash)

		// another text asset with the same contents, should hash the same way.
		text2 := "a test asset"
		asset2, err := NewTextAsset(text2)
		assert.Nil(t, err)
		assert.Equal(t, text2, asset2.Text)
		assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", asset2.Hash)

		// another text asset, but with different contents, should be a different hash.
		text3 := "a very different and special test asset"
		asset3, err := NewTextAsset(text3)
		assert.Nil(t, err)
		assert.Equal(t, text3, asset3.Text)
		assert.Equal(t, "9a6ed070e1ff834427105844ffd8a399a634753ce7a60ec5aae541524bbe7036", asset3.Hash)

		arch, err := NewAssetArchive(map[string]interface{}{"foo": asset})
		assert.Nil(t, err)
		assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", arch.Hash)
		archSer := arch.Serialize()
		archDes, isarch, err := DeserializeArchive(archSer)
		assert.Nil(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].(*Asset).IsText())
		assert.Equal(t, text, archDes.Assets["foo"].(*Asset).Text)
		assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", archDes.Hash)
	}
	{
		file := "/dev/null"
		asset, err := NewPathAsset(file)
		assert.Nil(t, err)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", asset.Hash)
		assetSer := asset.Serialize()
		assetDes, isasset, err := DeserializeAsset(assetSer)
		assert.Nil(t, err)
		assert.True(t, isasset)
		assert.True(t, assetDes.IsPath())
		assert.Equal(t, file, assetDes.Path)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", assetDes.Hash)

		arch, err := NewAssetArchive(map[string]interface{}{"foo": asset})
		assert.Nil(t, err)
		assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", arch.Hash)
		archSer := arch.Serialize()
		archDes, isarch, err := DeserializeArchive(archSer)
		assert.Nil(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].(*Asset).IsPath())
		assert.Equal(t, file, archDes.Assets["foo"].(*Asset).Path)
		assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", archDes.Hash)
	}
	{
		url := "file:///dev/null"
		asset, err := NewURIAsset(url)
		assert.Nil(t, err)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", asset.Hash)
		assetSer := asset.Serialize()
		assetDes, isasset, err := DeserializeAsset(assetSer)
		assert.Nil(t, err)
		assert.True(t, isasset)
		assert.True(t, assetDes.IsURI())
		assert.Equal(t, url, assetDes.URI)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", assetDes.Hash)

		arch, err := NewAssetArchive(map[string]interface{}{"foo": asset})
		assert.Nil(t, err)
		assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", arch.Hash)
		archSer := arch.Serialize()
		archDes, isarch, err := DeserializeArchive(archSer)
		assert.Nil(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].(*Asset).IsURI())
		assert.Equal(t, url, archDes.Assets["foo"].(*Asset).URI)
		assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", archDes.Hash)
	}
	{
		file, err := tempArchive("test")
		assert.Nil(t, err)
		defer func() { contract.IgnoreError(os.Remove(file)) }()
		arch, err := NewPathArchive(file)
		assert.Nil(t, err)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", arch.Hash)
		archSer := arch.Serialize()
		archDes, isarch, err := DeserializeArchive(archSer)
		assert.Nil(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsPath())
		assert.Equal(t, file, archDes.Path)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", archDes.Hash)
	}
	{
		file, err := tempArchive("test")
		assert.Nil(t, err)
		defer func() { contract.IgnoreError(os.Remove(file)) }()
		url := "file:///" + file
		arch, err := NewURIArchive(url)
		assert.Nil(t, err)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", arch.Hash)
		archSer := arch.Serialize()
		archDes, isarch, err := DeserializeArchive(archSer)
		assert.Nil(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsURI())
		assert.Equal(t, url, archDes.URI)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", archDes.Hash)
	}
}

func TestDeserializeMissingHash(t *testing.T) {
	assetSer := (&Asset{Text: "asset"}).Serialize()
	assetDes, isasset, err := DeserializeAsset(assetSer)
	assert.Nil(t, err)
	assert.True(t, isasset)
	assert.Equal(t, "asset", assetDes.Text)
}

func tempArchive(prefix string) (string, error) {
	for {
		path := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%x.tar", prefix, rand.Uint32()))
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if !os.IsExist(err) {
			if err != nil {
				defer contract.IgnoreClose(f)
				// write out an empty tar file.
				w := tar.NewWriter(f)
				contract.IgnoreClose(w)
			}
			return path, err
		}
	}
}
