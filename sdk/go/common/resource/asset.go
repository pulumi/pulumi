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

// The contents of this file have been moved. The logic behind assets and archives now
// lives in "github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset" and
// "github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive", respectively. This file
// exists to fulfill backwards-compatibility requirements. No new declarations should be
// added here.

import (
	"io"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
)

const (
	// BookkeepingDir is the name of our bookkeeping folder, we store state here (like .git for git).
	// Copied from workspace.BookkeepingDir to break import cycle.
	BookkeepingDir = ".pulumi"
)

type (
	Asset         = asset.Asset
	Blob          = asset.Blob
	Archive       = archive.Archive
	ArchiveFormat = archive.Format
	Reader        = archive.Reader
)

const (
	AssetSig          = asset.AssetSig
	AssetHashProperty = asset.AssetHashProperty
	AssetTextProperty = asset.AssetTextProperty
	AssetPathProperty = asset.AssetPathProperty
	AssetURIProperty  = asset.AssetURIProperty

	ArchiveSig            = archive.ArchiveSig
	ArchiveHashProperty   = archive.ArchiveHashProperty   // the dynamic property for an archive's hash.
	ArchiveAssetsProperty = archive.ArchiveAssetsProperty // the dynamic property for an archive's assets.
	ArchivePathProperty   = archive.ArchivePathProperty   // the dynamic property for an archive's path.
	ArchiveURIProperty    = archive.ArchiveURIProperty    // the dynamic property for an archive's URI.
)

// NewTextAsset produces a new asset and its corresponding SHA256 hash from the given text.
func NewTextAsset(text string) (*Asset, error) { return asset.FromText(text) }

// NewPathAsset produces a new asset and its corresponding SHA256 hash from the given filesystem path.
func NewPathAsset(path string) (*Asset, error) { return asset.FromPath(path) }

// NewPathAsset produces a new asset and its corresponding SHA256 hash from the given filesystem path.
func NewPathAssetWithWD(path string, cwd string) (*Asset, error) {
	return asset.FromPathWithWD(path, cwd)
}

// NewURIAsset produces a new asset and its corresponding SHA256 hash from the given network URI.
func NewURIAsset(uri string) (*Asset, error) { return asset.FromURI(uri) }

// DeserializeAsset checks to see if the map contains an asset, using its signature, and if so deserializes it.
func DeserializeAsset(obj map[string]interface{}) (*Asset, bool, error) {
	return asset.Deserialize(obj)
}

// NewByteBlob creates a new byte blob.
func NewByteBlob(data []byte) *Blob { return asset.NewByteBlob(data) }

// NewFileBlob creates a new asset blob whose size is known thanks to stat.
func NewFileBlob(f *os.File) (*Blob, error) { return asset.NewFileBlob(f) }

// NewReadCloserBlob turn any old ReadCloser into an Blob, usually by making a copy.
func NewReadCloserBlob(r io.ReadCloser) (*Blob, error) { return asset.NewReadCloserBlob(r) }

func NewAssetArchive(assets map[string]interface{}) (*Archive, error) {
	return archive.FromAssets(assets)
}

func NewAssetArchiveWithWD(assets map[string]interface{}, wd string) (*Archive, error) {
	return archive.FromAssetsWithWD(assets, wd)
}

func NewPathArchive(path string) (*Archive, error) {
	return archive.FromPath(path)
}

func NewPathArchiveWithWD(path string, wd string) (*Archive, error) {
	return archive.FromPathWithWD(path, wd)
}

func NewURIArchive(uri string) (*Archive, error) {
	return archive.FromURI(uri)
}

// DeserializeArchive checks to see if the map contains an archive, using its signature, and if so deserializes it.
func DeserializeArchive(obj map[string]interface{}) (*Archive, bool, error) {
	return archive.Deserialize(obj)
}

const (
	NotArchive = archive.NotArchive // not an archive.
	TarArchive = archive.TarArchive // a POSIX tar archive.
	// a POSIX tar archive that has been subsequently compressed using GZip.
	TarGZIPArchive = archive.TarGZIPArchive
	ZIPArchive     = archive.ZIPArchive // a multi-file ZIP archive.
	JARArchive     = archive.JARArchive // a Java JAR file
)

// ArchiveExts maps from a file extension and its associated archive and/or compression format.
var ArchiveExts = archive.ArchiveExts
