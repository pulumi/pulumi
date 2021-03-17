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

package pulumi

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"golang.org/x/net/context"
)

// AssetOrArchive represents either an Asset or an Archive.
type AssetOrArchive interface {
	isAssetOrArchive()
}

// Asset represents a file that is managed in conjunction with Pulumi resources.  An Asset may be backed by a number
// of sources, including local filesystem paths, in-memory blobs of text, or remote files referenced by a URL.
type Asset interface {
	AssetOrArchive
	AssetInput

	ToAssetOrArchiveOutput() AssetOrArchiveOutput
	ToAssetOrArchiveOutputWithContext(ctx context.Context) AssetOrArchiveOutput

	// Path returns the filesystem path, for file-based assets.
	Path() string
	// Text returns an in-memory blob of text, for string-based assets.
	Text() string
	// URI returns a URI, for remote network-based assets.
	URI() string

	isAsset()
}

type asset struct {
	path string
	text string
	uri  string
}

// NewFileAsset creates an asset backed by a file and specified by that file's path.
func NewFileAsset(path string) Asset {
	return &asset{path: path}
}

// NewStringAsset creates an asset backed by a piece of in-memory text.
func NewStringAsset(text string) Asset {
	return &asset{text: text}
}

// NewRemoteAsset creates an asset backed by a remote file and specified by that file's URL.
func NewRemoteAsset(uri string) Asset {
	return &asset{uri: uri}
}

// Path returns the asset's file path, if this is a file asset, or an empty string otherwise.
func (a *asset) Path() string { return a.path }

// Text returns the asset's textual string, if this is a string asset, or an empty string otherwise.
func (a *asset) Text() string { return a.text }

// URI returns the asset's URL, if this is a remote asset, or an empty string otherwise.
func (a *asset) URI() string { return a.uri }

func (a *asset) isAsset() {}

func (a *asset) isAssetOrArchive() {}

// Archive represents a collection of Assets.
type Archive interface {
	AssetOrArchive
	ArchiveInput

	ToAssetOrArchiveOutput() AssetOrArchiveOutput
	ToAssetOrArchiveOutputWithContext(ctx context.Context) AssetOrArchiveOutput

	// Assets returns a map of named assets or archives, for collections.
	Assets() map[string]interface{}
	// Path returns the filesystem path, for file-based archives.
	Path() string
	// URI returns a URI, for remote network-based archives.
	URI() string

	isArchive()
}

type archive struct {
	assets map[string]interface{}
	path   string
	uri    string
}

// NewAssetArchive creates a new archive from an in-memory collection of named assets or other archives.
func NewAssetArchive(assets map[string]interface{}) Archive {
	for k, a := range assets {
		if _, ok := a.(Asset); !ok {
			if _, ok2 := a.(Archive); !ok2 {
				contract.Failf(
					"expected asset map to contain Assets and/or Archives; %s is %v", k, reflect.TypeOf(a))
			}
		}
	}
	return &archive{assets: assets}
}

// NewFileArchive creates an archive backed by a file and specified by that file's path.
func NewFileArchive(path string) Archive {
	return &archive{path: path}
}

// NewRemoteArchive creates an archive backed by a remote file and specified by that file's URL.
func NewRemoteArchive(uri string) Archive {
	return &archive{uri: uri}
}

// Assets returns the archive's asset map, if this is a collection of archives/assets, or nil otherwise.
func (a *archive) Assets() map[string]interface{} { return a.assets }

// Path returns the archive's file path, if this is a file archive, or an empty string otherwise.
func (a *archive) Path() string { return a.path }

// URI returns the archive's URL, if this is a remote archive, or an empty string otherwise.
func (a *archive) URI() string { return a.uri }

func (a *archive) isArchive() {}

func (a *archive) isAssetOrArchive() {}
