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

package resource

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const (
	go19Version = "go1.9"
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

		// check that an empty asset also works correctly.
		empty, err := NewTextAsset("")
		assert.Nil(t, err)
		assert.Equal(t, "", empty.Text)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", empty.Hash)
		emptySer := empty.Serialize()
		emptyDes, isasset, err := DeserializeAsset(emptySer)
		assert.Nil(t, err)
		assert.True(t, isasset)
		assert.True(t, emptyDes.IsText())
		assert.Equal(t, "", emptyDes.Text)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", emptyDes.Hash)

		// now a map of nested assets and/or archives.
		arch, err := NewAssetArchive(map[string]interface{}{"foo": asset})
		assert.Nil(t, err)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", arch.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "27ab4a14a617df10cff3e1cf4e30cf510302afe56bf4cc91f84041c9f7b62fd8", arch.Hash)
		}
		archSer := arch.Serialize()
		archDes, isarch, err := DeserializeArchive(archSer)
		assert.Nil(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].(*Asset).IsText())
		assert.Equal(t, text, archDes.Assets["foo"].(*Asset).Text)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "d8ce0142b3b10300c7c76487fad770f794c1e84e1b0c73a4b2e1503d4fbac093", archDes.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "27ab4a14a617df10cff3e1cf4e30cf510302afe56bf4cc91f84041c9f7b62fd8", archDes.Hash)
		}
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
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", arch.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "d2587a875f82cdf3d3e6cfe9f8c6e6032be5dde8c344466e664e628da15757b0", arch.Hash)
		}
		archSer := arch.Serialize()
		archDes, isarch, err := DeserializeArchive(archSer)
		assert.Nil(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].(*Asset).IsPath())
		assert.Equal(t, file, archDes.Assets["foo"].(*Asset).Path)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", archDes.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "d2587a875f82cdf3d3e6cfe9f8c6e6032be5dde8c344466e664e628da15757b0", archDes.Hash)
		}
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
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", arch.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "d2587a875f82cdf3d3e6cfe9f8c6e6032be5dde8c344466e664e628da15757b0", arch.Hash)
		}
		archSer := arch.Serialize()
		archDes, isarch, err := DeserializeArchive(archSer)
		assert.Nil(t, err)
		assert.True(t, isarch)
		assert.True(t, archDes.IsAssets())
		assert.Equal(t, 1, len(archDes.Assets))
		assert.True(t, archDes.Assets["foo"].(*Asset).IsURI())
		assert.Equal(t, url, archDes.Assets["foo"].(*Asset).URI)
		switch runtime.Version() {
		case go19Version:
			assert.Equal(t, "23f6c195eb154be262216cd97209f2dcc8a40038ac8ec18ca6218d3e3dfacd4e", archDes.Hash)
		default:
			// Go 1.10 introduced breaking changes to archive/zip and archive/tar headers
			assert.Equal(t, "d2587a875f82cdf3d3e6cfe9f8c6e6032be5dde8c344466e664e628da15757b0", archDes.Hash)
		}
	}
	{
		file, err := tempArchive("test", false)
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
		file, err := tempArchive("test", false)
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
	{
		file1, err := tempArchive("test", false)
		assert.Nil(t, err)
		defer func() { contract.IgnoreError(os.Remove(file1)) }()
		file2, err := tempArchive("test2", false)
		assert.Nil(t, err)
		defer func() { contract.IgnoreError(os.Remove(file2)) }()
		arch1, err := NewPathArchive(file1)
		assert.Nil(t, err)
		arch2, err := NewPathArchive(file2)
		assert.Nil(t, err)
		assert.True(t, arch1.Equals(arch2))
		url := "file:///" + file1
		arch3, err := NewURIArchive(url)
		assert.Nil(t, err)
		assert.True(t, arch1.Equals(arch3))
	}
	{
		file, err := tempArchive("test", true)
		assert.Nil(t, err)
		defer func() { contract.IgnoreError(os.Remove(file)) }()
		arch1, err := NewPathArchive(file)
		assert.Nil(t, err)
		assert.Nil(t, arch1.EnsureHash())
		url := "file:///" + file
		arch2, err := NewURIArchive(url)
		assert.Nil(t, err)
		assert.Nil(t, arch2.EnsureHash())

		assert.Nil(t, os.Truncate(file, 0))
		arch3, err := NewPathArchive(file)
		assert.Nil(t, err)
		assert.Nil(t, arch3.EnsureHash())
		assert.False(t, arch1.Equals(arch3))
		arch4, err := NewURIArchive(url)
		assert.Nil(t, err)
		assert.Nil(t, arch4.EnsureHash())
		assert.False(t, arch2.Equals(arch4))
	}
}

func tempArchive(prefix string, fill bool) (string, error) {
	for {
		path := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%x.tar", prefix, rand.Uint32())) //nolint:gosec
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
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
					Mode: 0600,
					Size: 0,
				})
			}
			return path, err
		}
	}
}

func TestDeserializeMissingHash(t *testing.T) {
	assetSer := (&Asset{Text: "asset"}).Serialize()
	assetDes, isasset, err := DeserializeAsset(assetSer)
	assert.Nil(t, err)
	assert.True(t, isasset)
	assert.Equal(t, "asset", assetDes.Text)
}

func TestAssetFile(t *testing.T) {
	asset, err := NewPathAsset("../../../../pkg/resource/testdata/Fox.txt")
	assert.Nil(t, err)
	assert.Equal(t, "85e5f2698ac92d10d50e2f2802ed0d51a13e7c81d0d0a5998a75349469e774c5", asset.Hash)
	assertAssetTextEquals(t, asset,
		`The quick brown 🦊 jumps over
the lazy 🐶.  The quick brown
asset jumps over the archive.

`)
}

func TestArchiveDir(t *testing.T) {
	arch, err := NewPathArchive("../../../../pkg/resource/testdata/test_dir")
	assert.Nil(t, err)
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
	// Note that test data was generated using the Go 1.9 headers
	arch, err := NewPathArchive("../../../../pkg/resource/testdata/test_dir.tar")
	assert.Nil(t, err)
	assert.Equal(t, "c618d74a40f87de3092ca6a6c4cca834aa5c6a3956c6ceb2054b40d04bb4cd76", arch.Hash)
	validateTestDirArchive(t, arch, 3)
}

func TestArchiveTgz(t *testing.T) {
	// Note that test data was generated using the Go 1.9 headers
	arch, err := NewPathArchive("../../../../pkg/resource/testdata/test_dir.tgz")
	assert.Nil(t, err)
	assert.Equal(t, "f9b33523b6a3538138aff0769ff9e7d522038e33c5cfe28b258332b3f15790c8", arch.Hash)
	validateTestDirArchive(t, arch, 3)
}

func TestArchiveZip(t *testing.T) {
	// Note that test data was generated using the Go 1.9 headers
	arch, err := NewPathArchive("../../../../pkg/resource/testdata/test_dir.zip")
	assert.Nil(t, err)
	assert.Equal(t, "343da72cec1302441efd4a490d66f861d393fb270afb3ced27f92a0d96abc068", arch.Hash)
	validateTestDirArchive(t, arch, 3)
}

func TestArchiveJar(t *testing.T) {
	arch, err := NewPathArchive("../../../../pkg/resource/testdata/test_dir.jar")
	assert.Nil(t, err)
	assert.Equal(t, "dfb9eb69f433564b07df524068621c5ac65c08868e6094b8fa4ee388a5ee66e7", arch.Hash)
	validateTestDirArchive(t, arch, 4)
}

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
	repoRoot, err := findRepositoryRoot()
	assert.Nil(t, err)

	arch, err := NewPathArchive(repoRoot)
	assert.Nil(t, err)

	err = arch.Archive(TarArchive, ioutil.Discard)
	assert.Nil(t, err)
}

func TestArchiveZipFiles(t *testing.T) {
	repoRoot, err := findRepositoryRoot()
	assert.Nil(t, err)

	arch, err := NewPathArchive(repoRoot)
	assert.Nil(t, err)

	err = arch.Archive(ZIPArchive, ioutil.Discard)
	assert.Nil(t, err)
}

//nolint: gosec
func TestNestedArchive(t *testing.T) {
	// Create temp dir and place some files.
	dirName, err := ioutil.TempDir("", "")
	assert.Nil(t, err)
	assert.NoError(t, os.MkdirAll(filepath.Join(dirName, "foo", "bar"), 0777))
	assert.NoError(t, ioutil.WriteFile(filepath.Join(dirName, "foo", "a.txt"), []byte("a"), 0777))
	assert.NoError(t, ioutil.WriteFile(filepath.Join(dirName, "foo", "bar", "b.txt"), []byte("b"), 0777))
	assert.NoError(t, ioutil.WriteFile(filepath.Join(dirName, "c.txt"), []byte("c"), 0777))

	// Construct an AssetArchive with a nested PathArchive.
	innerArch, err := NewPathArchive(filepath.Join(dirName, "./foo"))
	assert.Nil(t, err)
	textAsset, err := NewTextAsset("hello world")
	assert.Nil(t, err)
	arch, err := NewAssetArchive(map[string]interface{}{
		"./foo":    innerArch,
		"fake.txt": textAsset,
	})
	assert.Nil(t, err)

	// Write a ZIP of the AssetArchive to disk.
	tmpFile, err := ioutil.TempFile("", "")
	fileName := tmpFile.Name()
	assert.Nil(t, err)
	err = arch.Archive(ZIPArchive, tmpFile)
	assert.Nil(t, err)
	tmpFile.Close()

	// Read the ZIP back into memory, and validate its contents.
	zipReader, err := zip.OpenReader(fileName)
	defer contract.IgnoreClose(zipReader)
	assert.Nil(t, err)
	files := zipReader.File
	assert.Len(t, files, 3)
	assert.Equal(t, "foo/a.txt", files[0].Name)
	assert.Equal(t, "foo/bar/b.txt", files[1].Name)
	assert.Equal(t, "fake.txt", files[2].Name)
}

//nolint: gosec
func TestFileReferencedThroughMultiplePaths(t *testing.T) {
	// Create temp dir and place some files.
	dirName, err := ioutil.TempDir("", "")
	assert.Nil(t, err)
	assert.NoError(t, os.MkdirAll(filepath.Join(dirName, "foo", "bar"), 0777))
	assert.NoError(t, ioutil.WriteFile(filepath.Join(dirName, "foo", "bar", "b.txt"), []byte("b"), 0777))

	// Construct an AssetArchive with a nested PathArchive.
	outerArch, err := NewPathArchive(filepath.Join(dirName, "./foo"))
	assert.Nil(t, err)
	innerArch, err := NewPathArchive(filepath.Join(dirName, "./foo/bar"))
	assert.Nil(t, err)
	arch, err := NewAssetArchive(map[string]interface{}{
		"./foo":     outerArch,
		"./foo/bar": innerArch,
	})
	assert.Nil(t, err)

	// Write a ZIP of the AssetArchive to disk.
	tmpFile, err := ioutil.TempFile("", "")
	fileName := tmpFile.Name()
	assert.Nil(t, err)
	err = arch.Archive(ZIPArchive, tmpFile)
	assert.Nil(t, err)
	tmpFile.Close()

	// Read the ZIP back into memory, and validate its contents.
	zipReader, err := zip.OpenReader(fileName)
	defer contract.IgnoreClose(zipReader)
	assert.Nil(t, err)
	files := zipReader.File
	assert.Len(t, files, 1)
	assert.Equal(t, "foo/bar/b.txt", files[0].Name)
}

func TestFileExtentionSniffing(t *testing.T) {
	assert.Equal(t, ArchiveFormat(ZIPArchive), detectArchiveFormat("./some/path/my.zip"))
	assert.Equal(t, ArchiveFormat(TarArchive), detectArchiveFormat("./some/path/my.tar"))
	assert.Equal(t, ArchiveFormat(TarGZIPArchive), detectArchiveFormat("./some/path/my.tar.gz"))
	assert.Equal(t, ArchiveFormat(TarGZIPArchive), detectArchiveFormat("./some/path/my.tgz"))
	assert.Equal(t, ArchiveFormat(JARArchive), detectArchiveFormat("./some/path/my.jar"))
	assert.Equal(t, ArchiveFormat(NotArchive), detectArchiveFormat("./some/path/who.knows"))

	// In #2589 we had cases where a file would look like it had an longer extension, because the suffix would include
	// some stuff after a dot. i.e. we failed to treat "my.file.zip" as a ZIPArchive.
	assert.Equal(t, ArchiveFormat(ZIPArchive), detectArchiveFormat("./some/path/my.file.zip"))
	assert.Equal(t, ArchiveFormat(TarArchive), detectArchiveFormat("./some/path/my.file.tar"))
	assert.Equal(t, ArchiveFormat(TarGZIPArchive), detectArchiveFormat("./some/path/my.file.tar.gz"))
	assert.Equal(t, ArchiveFormat(TarGZIPArchive), detectArchiveFormat("./some/path/my.file.tgz"))
	assert.Equal(t, ArchiveFormat(JARArchive), detectArchiveFormat("./some/path/my.file.jar"))
	assert.Equal(t, ArchiveFormat(NotArchive), detectArchiveFormat("./some/path/who.even.knows"))
}

func TestInvalidPathArchive(t *testing.T) {
	// Create a temp file that is not an asset.
	tmpFile, err := ioutil.TempFile("", "")
	fileName := tmpFile.Name()
	assert.NoError(t, err)
	fmt.Fprintf(tmpFile, "foo\n")
	tmpFile.Close()

	// Attempt to construct a PathArchive with the temp file.
	_, err = NewPathArchive(fileName)
	assert.Error(t, err)
}

func validateTestDirArchive(t *testing.T, arch *Archive, expected int) {
	r, err := arch.Open()
	assert.Nil(t, err)
	defer func() {
		assert.Nil(t, r.Close())
	}()

	subs := make(map[string]string)
	for {
		name, blob, err := r.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		assert.NotNil(t, blob)

		// Check for duplicates
		_, ok := subs[name]
		assert.False(t, ok)

		// Read the blob
		var text bytes.Buffer
		_, err = io.Copy(&text, blob)
		assert.Nil(t, err)
		err = blob.Close()
		assert.Nil(t, err)

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

func assertAssetTextEquals(t *testing.T, asset *Asset, expect string) {
	blob, err := asset.Read()
	assert.Nil(t, err)
	assert.NotNil(t, blob)
	assertAssetBlobEquals(t, blob, expect)
}

func assertAssetBlobEquals(t *testing.T, blob *Blob, expect string) {
	var text bytes.Buffer
	_, err := io.Copy(&text, blob)
	assert.Nil(t, err)
	assert.Equal(t, expect, text.String())
	err = blob.Close()
	assert.Nil(t, err)
}
