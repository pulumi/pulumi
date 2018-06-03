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

package asset

import (
	"reflect"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Asset represents a file that is managed in conjunction with Pulumi resources.  An Asset may be backed by a number
// of sources, including local filesystem paths, in-memory blobs of text, or remote files referenced by a URL.
type Asset interface {
	// Path returns the filesystem path, for file-based assets.
	Path() string
	// Text returns an in-memory blob of text, for string-based assets.
	Text() string
	// URI returns a URI, for remote network-based assets.
	URI() string
}

type asset struct {
	path string
	text string
	uri  string
}

func NewFileAsset(path string) Asset {
	return &asset{path: path}
}

func NewStringAsset(text string) Asset {
	return &asset{text: text}
}

func NewRemoteAsset(uri string) Asset {
	return &asset{uri: uri}
}

func (a *asset) Path() string { return a.path }
func (a *asset) Text() string { return a.text }
func (a *asset) URI() string  { return a.uri }

// Archive represents a collection of Assets.
type Archive interface {
	// Assets returns a map of named assets or archives, for collections.
	Assets() map[string]interface{}
	// Path returns the filesystem path, for file-based archives.
	Path() string
	// URI returns a URI, for remote network-based archives.
	URI() string
}

type archive struct {
	assets map[string]interface{}
	path   string
	uri    string
}

// NewAssetArchive creates a new archive from an in-memory collection of named assets or other archives.
func NewAssetArchive(assets map[string]interface{}) Archive {
	for k, a := range assets {
		if _, ok := a.(*Asset); !ok {
			if _, ok2 := a.(*Archive); !ok2 {
				contract.Failf(
					"expected asset map to contain *Assets and/or *Archives; %s is %v", k, reflect.TypeOf(a))
			}
		}
	}
	return &archive{assets: assets}
}

func NewPathArchive(path string) Archive {
	return &archive{path: path}
}

func NewURIArchive(uri string) Archive {
	return &archive{uri: uri}
}

func (a *archive) Assets() map[string]interface{} { return a.assets }
func (a *archive) Path() string                   { return a.path }
func (a *archive) URI() string                    { return a.uri }
