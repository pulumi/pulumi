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

package resource

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rarchive "github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	rasset "github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const (
	go19Version = "go1.9"
)

// TODO[pulumi/pulumi#8647]
func skipWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipped on Windows: TODO handle Windows local paths in URIs")
	}
}

func TestAssetSerialize(t *testing.T) {
	t.Parallel()

	// Ensure that asset and archive serialization round trips.
	t.Run("text asset", func(t *testing.T) {
		t.Parallel()

		text := "a test asset"
		asset, err := rasset.FromText(text)
		require.NoError(t, err)
		assert.Equal(t, text, asset.Text)
		assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", asset.Hash)
		assetSer := asset.Serialize()
		assetDes, isasset, err := rasset.Deserialize(assetSer)
		require.NoError(t, err)
		assert.True(t, isasset)
		assert.True(t, assetDes.IsText())
		assert.Equal(t, text, assetDes.Text)
		assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", assetDes.Hash)
		assert.Equal(t, AssetSig, assetDes.Sig)

		// another text asset with the same contents, should hash the same way.
		text2 := "a test asset"
		asset2, err := rasset.FromText(text2)
		require.NoError(t, err)
		assert.Equal(t, text2, asset2.Text)
		assert.Equal(t, "e34c74529110661faae4e121e57165ff4cb4dbdde1ef9770098aa3695e6b6704", asset2.Hash)

		// another text asset, but with different contents, should be a different hash.
		text3 := "a very different and special test asset"
		asset3, err := rasset.FromText(text3)
		require.NoError(t, err)
		assert.Equal(t, text3, asset3.Text)
		assert.Equal(t, "9a6ed070e1ff834427105844ffd8a399a634753ce7a60ec5aae541524bbe7036", asset3.Hash)

		// check that an empty text asset also works correctly.
		empty, err := rasset.FromText("")
		require.NoError(t, err)
		assert.Equal(t, "", empty.Text)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", empty.Hash)
		emptySer := empty.Serialize()
		emptyDes, isasset, err := rasset.Deserialize(emptySer)
		require.NoError(t, err)
		assert.True(t, isasset)
		assert.True(t, emptyDes.IsText())
		assert.Equal(t, "", emptyDes.Text)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", emptyDes.Hash)

		// now a map of nested assets and/or archives.
		arch, err := rarchive.FromAssets(map[string]interface{}{"foo": asset})
		require.NoError(t, err)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", arch.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "27ab4a14a617df10cff3e1cf4e30cf510302afe56bf4cc91f84041c9f7b62fd8", arch.Hash)
		}
		archSer := arch.Serialize()
		archDes, isarch, err := rarchive.Deserialize(archSer)
		require.NoError(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].(*rasset.Asset).IsText())
		assert.Equal(t, text, archDes.Assets["foo"].(*rasset.Asset).Text)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", archDes.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "27ab4a14a617df10cff3e1cf4e30cf510302afe56bf4cc91f84041c9f7b62fd8", archDes.Hash)
		}
	})
	t.Run("path asset", func(t *testing.T) {
		t.Parallel()

		f, err := os.CreateTemp(t.TempDir(), "")
		require.NoError(t, err)
		file := f.Name()
		asset, err := rasset.FromPath(file)
		require.NoError(t, err)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", asset.Hash)
		assetSer := asset.Serialize()
		assetDes, isasset, err := rasset.Deserialize(assetSer)
		require.NoError(t, err)
		assert.True(t, isasset)
		assert.True(t, assetDes.IsPath())
		assert.Equal(t, file, assetDes.Path)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", assetDes.Hash)
		assert.Equal(t, AssetSig, assetDes.Sig)

		arch, err := rarchive.FromAssets(map[string]interface{}{"foo": asset})
		require.NoError(t, err)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", arch.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "d2587a875f82cdf3d3e6cfe9f8c6e6032be5dde8c344466e664e628da15757b0", arch.Hash)
		}
		archSer := arch.Serialize()
		archDes, isarch, err := rarchive.Deserialize(archSer)
		require.NoError(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].(*rasset.Asset).IsPath())
		assert.Equal(t, file, archDes.Assets["foo"].(*rasset.Asset).Path)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", archDes.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "d2587a875f82cdf3d3e6cfe9f8c6e6032be5dde8c344466e664e628da15757b0", archDes.Hash)
		}
	})
	t.Run("local uri asset", func(t *testing.T) {
		t.Parallel()

		skipWindows(t)
		url := "file:///dev/null"
		asset, err := rasset.FromURI(url)
		require.NoError(t, err)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", asset.Hash)
		assetSer := asset.Serialize()
		assetDes, isasset, err := rasset.Deserialize(assetSer)
		require.NoError(t, err)
		assert.True(t, isasset)
		assert.True(t, assetDes.IsURI())
		assert.Equal(t, url, assetDes.URI)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", assetDes.Hash)
		assert.Equal(t, AssetSig, assetDes.Sig)

		arch, err := rarchive.FromAssets(map[string]interface{}{"foo": asset})
		require.NoError(t, err)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", arch.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "d2587a875f82cdf3d3e6cfe9f8c6e6032be5dde8c344466e664e628da15757b0", arch.Hash)
		}
		archSer := arch.Serialize()
		archDes, isarch, err := rarchive.Deserialize(archSer)
		require.NoError(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].(*rasset.Asset).IsURI())
		assert.Equal(t, url, archDes.Assets["foo"].(*rasset.Asset).URI)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", archDes.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "d2587a875f82cdf3d3e6cfe9f8c6e6032be5dde8c344466e664e628da15757b0", archDes.Hash)
		}
	})
	t.Run("empty asset", func(t *testing.T) {
		t.Parallel()

		// Check that a _fully_ empty asset is treated as an empty text asset.
		empty := &rasset.Asset{}
		assert.True(t, empty.IsText())
		assert.Equal(t, "", empty.Text)
		emptySer := empty.Serialize()
		emptyDes, isasset, err := rasset.Deserialize(emptySer)
		require.NoError(t, err)
		assert.True(t, isasset)
		assert.True(t, emptyDes.IsText())
		assert.Equal(t, "", emptyDes.Text)
		assert.Equal(t, AssetSig, emptyDes.Sig)

		// Check that a text asset with it's text removed shows as "no content".
		asset, err := rasset.FromText("a very different and special test asset")
		require.NoError(t, err)
		asset.Text = ""
		assert.False(t, asset.IsText())
		assert.False(t, asset.HasContents())
		asset, isasset, err = rasset.Deserialize(asset.Serialize())
		require.NoError(t, err)
		assert.True(t, isasset)
		assert.False(t, asset.IsText())
		assert.False(t, asset.HasContents())
	})
}

func TestArchiveSerialize(t *testing.T) {
	t.Parallel()

	t.Run("path archive", func(t *testing.T) {
		t.Parallel()

		file, err := tempArchive("test", false)
		require.NoError(t, err)
		defer func() { contract.IgnoreError(os.Remove(file)) }()
		arch, err := rarchive.FromPath(file)
		require.NoError(t, err)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", arch.Hash)
		archSer := arch.Serialize()
		archDes, isarch, err := rarchive.Deserialize(archSer)
		require.NoError(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsPath())
		assert.Equal(t, file, archDes.Path)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", archDes.Hash)
	})
	t.Run("uri archive", func(t *testing.T) {
		t.Parallel()

		skipWindows(t)
		file, err := tempArchive("test", false)
		require.NoError(t, err)
		defer func() { contract.IgnoreError(os.Remove(file)) }()
		url := "file:///" + file
		arch, err := rarchive.FromURI(url)
		require.NoError(t, err)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", arch.Hash)
		archSer := arch.Serialize()
		archDes, isarch, err := rarchive.Deserialize(archSer)
		require.NoError(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsURI())
		assert.Equal(t, url, archDes.URI)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", archDes.Hash)
	})
	t.Run("local uri archive", func(t *testing.T) {
		t.Parallel()

		skipWindows(t)
		file1, err := tempArchive("test", false)
		require.NoError(t, err)
		defer func() { contract.IgnoreError(os.Remove(file1)) }()
		file2, err := tempArchive("test2", false)
		require.NoError(t, err)
		defer func() { contract.IgnoreError(os.Remove(file2)) }()
		arch1, err := rarchive.FromPath(file1)
		require.NoError(t, err)
		arch2, err := rarchive.FromPath(file2)
		require.NoError(t, err)
		assert.True(t, arch1.Equals(arch2))
		url := "file:///" + file1
		arch3, err := rarchive.FromURI(url)
		require.NoError(t, err)
		assert.True(t, arch1.Equals(arch3))
	})
	t.Run("nested archives", func(t *testing.T) {
		t.Parallel()

		skipWindows(t)
		file, err := tempArchive("test", true)
		require.NoError(t, err)
		defer func() { contract.IgnoreError(os.Remove(file)) }()
		arch1, err := rarchive.FromPath(file)
		require.NoError(t, err)
		assert.Nil(t, arch1.EnsureHash())
		url := "file:///" + file
		arch2, err := rarchive.FromURI(url)
		require.NoError(t, err)
		assert.Nil(t, arch2.EnsureHash())

		assert.Nil(t, os.Truncate(file, 0))
		arch3, err := rarchive.FromPath(file)
		require.NoError(t, err)
		assert.Nil(t, arch3.EnsureHash())
		assert.False(t, arch1.Equals(arch3))
		arch4, err := rarchive.FromURI(url)
		require.NoError(t, err)
		assert.Nil(t, arch4.EnsureHash())
		assert.False(t, arch2.Equals(arch4))
	})
	t.Run("nested archives", func(t *testing.T) {
		t.Parallel()

		skipWindows(t)
		file, err := tempArchive("test", true)
		require.NoError(t, err)
		defer func() { contract.IgnoreError(os.Remove(file)) }()
		arch1, err := rarchive.FromPath(file)
		require.NoError(t, err)
		assert.Nil(t, arch1.EnsureHash())
		url := "file:///" + file
		arch2, err := rarchive.FromURI(url)
		require.NoError(t, err)
		assert.Nil(t, arch2.EnsureHash())

		assert.Nil(t, os.Truncate(file, 0))
		arch3, err := rarchive.FromPath(file)
		require.NoError(t, err)
		assert.Nil(t, arch3.EnsureHash())
		assert.False(t, arch1.Equals(arch3))
		arch4, err := rarchive.FromURI(url)
		require.NoError(t, err)
		assert.Nil(t, arch4.EnsureHash())
		assert.False(t, arch2.Equals(arch4))
	})
	t.Run("assets archive", func(t *testing.T) {
		t.Parallel()

		archive, err := rarchive.FromAssets(map[string]interface{}{})
		require.NoError(t, err)
		assert.True(t, archive.IsAssets())
		assert.Equal(t, "5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef", archive.Hash)
		archiveSer := archive.Serialize()
		archiveDes, isarchive, err := rarchive.Deserialize(archiveSer)
		require.NoError(t, err)
		assert.True(t, isarchive)
		assert.True(t, archiveDes.IsAssets())
		assert.Equal(t, 0, len(archiveDes.Assets))
	})
	t.Run("empty archive", func(t *testing.T) {
		t.Parallel()

		// Check that a fully empty archive is treated as an empty assets archive.
		empty := &rarchive.Archive{}
		assert.True(t, empty.IsAssets())
		assert.Equal(t, 0, len(empty.Assets))
		emptySer := empty.Serialize()
		emptyDes, isarchive, err := rarchive.Deserialize(emptySer)
		require.NoError(t, err)
		assert.True(t, isarchive)
		assert.True(t, emptyDes.IsAssets())

		// Check that an assets archive with it's assets removed shows as "no content".
		asset, err := rasset.FromText("hello world")
		require.NoError(t, err)
		archive, err := rarchive.FromAssets(map[string]interface{}{"foo": asset})
		require.NoError(t, err)
		archive.Assets = nil
		assert.False(t, archive.IsAssets())
		assert.False(t, archive.HasContents())
		archive, isarchive, err = rarchive.Deserialize(archive.Serialize())
		require.NoError(t, err)
		assert.True(t, isarchive)
		assert.False(t, archive.IsAssets())
		assert.False(t, archive.HasContents())
	})
}

func tempArchive(prefix string, fill bool) (string, error) {
	for {
		path := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%x.tar", prefix, rand.Uint32())) //nolint:gosec
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		switch {
		case os.IsExist(err):
			continue
		case err != nil:
			return "", err
		default:
			defer contract.IgnoreClose(f)

			// write out a tar file. if `fill` is true, add a single empty file.
			if fill {
				w := tar.NewWriter(f)
				defer contract.IgnoreClose(w)

				err = w.WriteHeader(&tar.Header{
					Name: "file",
					Mode: 0o600,
					Size: 0,
				})
			}
			return path, err
		}
	}
}

func TestDeserializeMissingHash(t *testing.T) {
	t.Parallel()

	assetSer := (&rasset.Asset{Text: "asset"}).Serialize()
	assetDes, isasset, err := rasset.Deserialize(assetSer)
	require.NoError(t, err)
	assert.True(t, isasset)
	assert.Equal(t, "asset", assetDes.Text)
}

func TestAssetFile(t *testing.T) {
	t.Parallel()

	asset, err := rasset.FromPath("../../../../pkg/resource/testdata/Fox.txt")
	require.NoError(t, err)
	assert.Equal(t, "85e5f2698ac92d10d50e2f2802ed0d51a13e7c81d0d0a5998a75349469e774c5", asset.Hash)
	assertAssetTextEquals(t, asset,
		`The quick brown 🦊 jumps over
the lazy 🐶.  The quick brown
asset jumps over the archive.

`)
}

func TestArchiveDir(t *testing.T) {
	t.Parallel()

	arch, err := rarchive.FromPath("../../../../pkg/resource/testdata/test_dir")
	require.NoError(t, err)
	switch runtime.Version() {
	case go19Version:
		assert.Equal(t, "35ddf9c48ce6ac5ba657573d388db6ce41f3ed6965346a3086fb70a550fe0864", arch.Hash)
	default:
		// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
		assert.Equal(t, "489e9a9dad271922ecfbda590efc40e48788286a06bd406a357ab8d13f0b6abf", arch.Hash)
	}
	validateTestDirArchive(t, arch, 3)
}

func TestArchiveTar(t *testing.T) {
	t.Parallel()

	// Note that test data was generated using the Go 1.9 headers
	arch, err := rarchive.FromPath("../../../../pkg/resource/testdata/test_dir.tar")
	require.NoError(t, err)
	assert.Equal(t, "c618d74a40f87de3092ca6a6c4cca834aa5c6a3956c6ceb2054b40d04bb4cd76", arch.Hash)
	validateTestDirArchive(t, arch, 3)
}

func TestArchiveTgz(t *testing.T) {
	t.Parallel()

	// Note that test data was generated using the Go 1.9 headers
	arch, err := rarchive.FromPath("../../../../pkg/resource/testdata/test_dir.tgz")
	require.NoError(t, err)
	assert.Equal(t, "f9b33523b6a3538138aff0769ff9e7d522038e33c5cfe28b258332b3f15790c8", arch.Hash)
	validateTestDirArchive(t, arch, 3)
}

func TestArchiveZip(t *testing.T) {
	t.Parallel()

	// Note that test data was generated using the Go 1.9 headers
	arch, err := rarchive.FromPath("../../../../pkg/resource/testdata/test_dir.zip")
	require.NoError(t, err)
	assert.Equal(t, "343da72cec1302441efd4a490d66f861d393fb270afb3ced27f92a0d96abc068", arch.Hash)
	validateTestDirArchive(t, arch, 3)
}

func TestArchiveJar(t *testing.T) {
	t.Parallel()

	arch, err := rarchive.FromPath("../../../../pkg/resource/testdata/test_dir.jar")
	require.NoError(t, err)
	assert.Equal(t, "dfb9eb69f433564b07df524068621c5ac65c08868e6094b8fa4ee388a5ee66e7", arch.Hash)
	validateTestDirArchive(t, arch, 4)
}

//nolint:unused // Used by tests that are currently skipped
func findRepositoryRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for d := wd; ; d = filepath.Dir(d) {
		if d == "" || d == "." || d[len(d)-1] == filepath.Separator {
			return "", errors.New("could not find repository root")
		}
		gitDir := filepath.Join(d, ".git")
		_, err := os.Lstat(gitDir)
		switch {
		case err == nil:
			return d, nil
		case !os.IsNotExist(err):
			return "", err
		}
	}
}

func TestArchiveTarFiles(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#7976] flaky
	t.Skip("Disabled due to flakiness. See #7976.")

	repoRoot, err := findRepositoryRoot()
	require.NoError(t, err)

	arch, err := rarchive.FromPath(repoRoot)
	require.NoError(t, err)

	err = arch.Archive(rarchive.TarArchive, io.Discard)
	require.NoError(t, err)
}

func TestArchiveZipFiles(t *testing.T) {
	t.Parallel()

	t.Skip() // TODO[pulumi/pulumi#7147]
	repoRoot, err := findRepositoryRoot()
	require.NoError(t, err)

	arch, err := rarchive.FromPath(repoRoot)
	require.NoError(t, err)

	err = arch.Archive(rarchive.ZIPArchive, io.Discard)
	require.NoError(t, err)
}

//nolint:gosec
func TestNestedArchive(t *testing.T) {
	t.Parallel()

	// Create temp dir and place some files.
	dirName := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dirName, "foo", "bar"), 0o777))
	require.NoError(t, os.WriteFile(filepath.Join(dirName, "foo", "a.txt"), []byte("a"), 0o777))
	require.NoError(t, os.WriteFile(filepath.Join(dirName, "foo", "bar", "b.txt"), []byte("b"), 0o777))
	require.NoError(t, os.WriteFile(filepath.Join(dirName, "c.txt"), []byte("c"), 0o777))

	// Construct an AssetArchive with a nested PathArchive.
	innerArch, err := rarchive.FromPath(filepath.Join(dirName, "./foo"))
	require.NoError(t, err)
	textAsset, err := rasset.FromText("hello world")
	require.NoError(t, err)
	arch, err := rarchive.FromAssets(map[string]interface{}{
		"./foo":    innerArch,
		"fake.txt": textAsset,
	})
	require.NoError(t, err)

	// Write a ZIP of the AssetArchive to disk.
	tmpFile, err := os.CreateTemp(t.TempDir(), "")
	fileName := tmpFile.Name()
	require.NoError(t, err)
	err = arch.Archive(rarchive.ZIPArchive, tmpFile)
	require.NoError(t, err)
	tmpFile.Close()

	// Read the ZIP back into memory, and validate its contents.
	zipReader, err := zip.OpenReader(fileName)
	defer contract.IgnoreClose(zipReader)
	require.NoError(t, err)
	files := zipReader.File
	assert.Len(t, files, 3)

	assert.Equal(t, "foo/a.txt", filepath.ToSlash(files[0].Name))
	assert.Equal(t, "foo/bar/b.txt", filepath.ToSlash(files[1].Name))
	assert.Equal(t, "fake.txt", filepath.ToSlash(files[2].Name))
}

//nolint:gosec
func TestFileReferencedThroughMultiplePaths(t *testing.T) {
	t.Parallel()

	// Create temp dir and place some files.
	dirName := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dirName, "foo", "bar"), 0o777))
	require.NoError(t, os.WriteFile(filepath.Join(dirName, "foo", "bar", "b.txt"), []byte("b"), 0o777))

	// Construct an AssetArchive with a nested PathArchive.
	outerArch, err := rarchive.FromPath(filepath.Join(dirName, "./foo"))
	require.NoError(t, err)
	innerArch, err := rarchive.FromPath(filepath.Join(dirName, "./foo/bar"))
	require.NoError(t, err)
	arch, err := rarchive.FromAssets(map[string]interface{}{
		"./foo":     outerArch,
		"./foo/bar": innerArch,
	})
	require.NoError(t, err)

	// Write a ZIP of the AssetArchive to disk.
	tmpFile, err := os.CreateTemp(t.TempDir(), "")
	fileName := tmpFile.Name()
	require.NoError(t, err)
	err = arch.Archive(rarchive.ZIPArchive, tmpFile)
	require.NoError(t, err)
	tmpFile.Close()

	// Read the ZIP back into memory, and validate its contents.
	zipReader, err := zip.OpenReader(fileName)
	defer contract.IgnoreClose(zipReader)
	require.NoError(t, err)
	files := zipReader.File
	assert.Len(t, files, 1)
	assert.Equal(t, "foo/bar/b.txt", filepath.ToSlash(files[0].Name))
}

func TestEmptyArchiveRoundTrip(t *testing.T) {
	t.Parallel()

	emptyArchive, err := rarchive.FromAssets(nil)
	require.NoError(t, err, "Creating an empty archive should work")
	assert.True(t, emptyArchive.IsAssets(), "even empty archives should be have empty assets")
	serialized := emptyArchive.Serialize()
	deserialized, ok, err := rarchive.Deserialize(serialized)
	require.NoError(t, err, "Deserializing an empty archive should work")
	assert.True(t, ok, "Deserializing an empty archive should return true")
	assert.True(t, deserialized.IsAssets(), "Deserialized archive should be an AssetsArchive")
}

func TestInvalidPathArchive(t *testing.T) {
	t.Parallel()

	// Create a temp file that is not an asset.
	tmpFile, err := os.CreateTemp(t.TempDir(), "")
	fileName := tmpFile.Name()
	require.NoError(t, err)
	fmt.Fprintf(tmpFile, "foo\n")
	tmpFile.Close()

	// Attempt to construct a PathArchive with the temp file.
	_, err = rarchive.FromPath(fileName)
	assert.EqualError(t, err, fmt.Sprintf("'%s' is neither a recognized archive type nor a directory", fileName))
}

func validateTestDirArchive(t *testing.T, arch *rarchive.Archive, expected int) {
	r, err := arch.Open()
	require.NoError(t, err)
	defer func() {
		assert.Nil(t, r.Close())
	}()

	subs := make(map[string]string)
	for {
		name, blob, err := r.Next()
		name = filepath.ToSlash(name)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		require.NotNil(t, blob)

		// Check for duplicates
		_, ok := subs[name]
		assert.False(t, ok)

		// Read the blob
		var text bytes.Buffer
		n, err := io.Copy(&text, blob)
		require.NoError(t, err)
		assert.Equal(t, blob.Size(), n)
		err = blob.Close()
		require.NoError(t, err)

		// Store its contents in subs
		subs[name] = text.String()
	}

	assert.Equal(t, expected, len(subs))

	lorem := subs["Lorem_ipsum.txt"]
	assert.Equal(t, lorem,
		`Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna
aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis
aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint
occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

`)

	butimust := subs["sub_dir/But_I_must"]
	assert.Equal(t, butimust,
		`Sed ut perspiciatis, unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem
aperiam eaque ipsa, quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt, explicabo. Nemo enim
ipsam voluptatem, quia voluptas sit, aspernatur aut odit aut fugit, sed quia consequuntur magni dolores eos, qui ratione
voluptatem sequi nesciunt, neque porro quisquam est, qui dolorem ipsum, quia dolor sit amet consectetur adipisci[ng]
velit, sed quia non numquam [do] eius modi tempora inci[di]dunt, ut labore et dolore magnam aliquam quaerat voluptatem.
Ut enim ad minima veniam, quis nostrum exercitationem ullam corporis suscipit laboriosam, nisi ut aliquid ex ea commodi
consequatur? Quis autem vel eum iure reprehenderit, qui in ea voluptate velit esse, quam nihil molestiae consequatur,
vel illum, qui dolorem eum fugiat, quo voluptas nulla pariatur?

`)

	ontheother := subs["sub_dir/On_the_other_hand.md"]
	assert.Equal(t, ontheother,
		`At vero eos et accusamus et iusto odio dignissimos ducimus, qui blanditiis praesentium voluptatum deleniti atque
corrupti, quos dolores et quas molestias excepturi sint, obcaecati cupiditate non provident, similique sunt in culpa,
qui officia deserunt mollitia animi, id est laborum et dolorum fuga. Et harum quidem rerum facilis est et expedita
distinctio. Nam libero tempore, cum soluta nobis est eligendi optio, cumque nihil impedit, quo minus id, quod maxime
placeat, facere possimus, omnis voluptas assumenda est, omnis dolor repellendus. Temporibus autem quibusdam et aut
officiis debitis aut rerum necessitatibus saepe eveniet, ut et voluptates repudiandae sint et molestiae non recusandae.
Itaque earum rerum hic tenetur a sapiente delectus, ut aut reiciendis voluptatibus maiores alias consequatur aut
perferendis doloribus asperiores repellat…

`)
}

func assertAssetTextEquals(t *testing.T, asset *rasset.Asset, expect string) {
	blob, err := asset.Read()
	require.NoError(t, err)
	require.NotNil(t, blob)
	assertAssetBlobEquals(t, blob, expect)
}

func assertAssetBlobEquals(t *testing.T, blob *rasset.Blob, expect string) {
	var text bytes.Buffer
	n, err := io.Copy(&text, blob)
	require.NoError(t, err)
	assert.Equal(t, blob.Size(), n)
	assert.Equal(t, expect, text.String())
	err = blob.Close()
	require.NoError(t, err)
}
