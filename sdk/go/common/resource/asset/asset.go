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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
)

// Asset is a serialized asset reference. It is a union: thus, at most one of its fields will be non-nil. Several helper
// routines exist as members in order to easily interact with the assets referenced by an instance of this type.
type Asset struct {
	// Sig is the unique asset type signature (see properties.go).
	Sig string `json:"4dabf18193072939515e22adb298388d" yaml:"4dabf18193072939515e22adb298388d"`
	// Hash is the SHA256 hash of the asset's contents.
	Hash string `json:"hash,omitempty" yaml:"hash,omitempty"`
	// Text is set to a non-empty value for textual assets.
	Text string `json:"text,omitempty" yaml:"text,omitempty"`
	// Path will contain a non-empty path to the file on the current filesystem for file assets.
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	// URI will contain a non-empty URI (file://, http://, https://, or custom) for URI-backed assets.
	URI string `json:"uri,omitempty" yaml:"uri,omitempty"`
}

const (
	AssetSig          = sig.AssetSig
	AssetHashProperty = "hash" // the dynamic property for an asset's hash.
	AssetTextProperty = "text" // the dynamic property for an asset's text.
	AssetPathProperty = "path" // the dynamic property for an asset's path.
	AssetURIProperty  = "uri"  // the dynamic property for an asset's URI.
)

// FromText produces a new asset and its corresponding SHA256 hash from the given text.
func FromText(text string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, Text: text}
	// Special case the empty string otherwise EnsureHash will fail.
	if text == "" {
		a.Hash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
	err := a.EnsureHash()
	return a, err
}

// FromPath produces a new asset and its corresponding SHA256 hash from the given filesystem path.
func FromPath(path string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, Path: path}
	err := a.EnsureHash()
	return a, err
}

// FromPathWithWD produces a new asset and its corresponding SHA256 hash from the given filesystem path.
func FromPathWithWD(path string, wd string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, Path: path}
	err := a.EnsureHashWithWD(wd)
	return a, err
}

// FromURI produces a new asset and its corresponding SHA256 hash from the given network URI.
func FromURI(uri string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, URI: uri}
	err := a.EnsureHash()
	return a, err
}

func (a *Asset) IsText() bool {
	if a.IsPath() || a.IsURI() {
		return false
	}
	if a.Text != "" {
		return true
	}
	// We can't easily tell the difference between an Asset that really has the empty string as its text and one that
	// has no text at all. If we have a hash we can check if that's the "zero hash" and if so then we know the text is
	// just empty. If the hash does not equal the empty hash then we know this is a _placeholder_ asset where the text is
	// just currently not known. If we don't have a hash then we can't tell the difference and assume it's just empty.
	if a.Hash == "" || a.Hash == "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		return true
	}
	return false
}
func (a *Asset) IsPath() bool { return a.Path != "" }
func (a *Asset) IsURI() bool  { return a.URI != "" }

func (a *Asset) GetText() (string, bool) {
	if a.IsText() {
		return a.Text, true
	}
	return "", false
}

func (a *Asset) GetPath() (string, bool) {
	if a.IsPath() {
		return a.Path, true
	}
	return "", false
}

func (a *Asset) GetURI() (string, bool) {
	if a.IsURI() {
		return a.URI, true
	}
	return "", false
}

// GetURIURL returns the underlying URI as a parsed URL, provided it is one.  If there was an error parsing the URI, it
// will be returned as a non-nil error object.
func (a *Asset) GetURIURL() (*url.URL, bool, error) {
	if uri, isuri := a.GetURI(); isuri {
		url, err := url.Parse(uri)
		if err != nil {
			return nil, true, err
		}
		return url, true, nil
	}
	return nil, false, nil
}

// Equals returns true if a is value-equal to other. In this case, value equality is determined only by the hash: even
// if the contents of two assets come from different sources, they are treated as equal if their hashes match.
// Similarly, if the contents of two assets come from the same source but the assets have different hashes, the assets
// are not equal.
func (a *Asset) Equals(other *Asset) bool {
	if a == nil {
		return other == nil
	} else if other == nil {
		return false
	}

	// If we can't get a hash for both assets, treat them as differing.
	if err := a.EnsureHash(); err != nil {
		return false
	}
	if err := other.EnsureHash(); err != nil {
		return false
	}
	return a.Hash == other.Hash
}

// Serialize returns a weakly typed map that contains the right signature for serialization purposes.
func (a *Asset) Serialize() map[string]interface{} {
	result := map[string]interface{}{
		sig.Key: AssetSig,
	}
	if a.Hash != "" {
		result[AssetHashProperty] = a.Hash
	}
	if a.Text != "" {
		result[AssetTextProperty] = a.Text
	}
	if a.Path != "" {
		result[AssetPathProperty] = a.Path
	}
	if a.URI != "" {
		result[AssetURIProperty] = a.URI
	}
	return result
}

// DeserializeAsset checks to see if the map contains an asset, using its signature, and if so deserializes it.
func Deserialize(obj map[string]interface{}) (*Asset, bool, error) {
	// If not an asset, return false immediately.
	if obj[sig.Key] != AssetSig {
		return &Asset{}, false, nil
	}

	// Else, deserialize the possible fields.
	var hash string
	if v, has := obj[AssetHashProperty]; has {
		h, ok := v.(string)
		if !ok {
			return &Asset{}, false, fmt.Errorf("unexpected asset hash of type %T", v)
		}
		hash = h
	}
	var text string
	if v, has := obj[AssetTextProperty]; has {
		t, ok := v.(string)
		if !ok {
			return &Asset{}, false, fmt.Errorf("unexpected asset text of type %T", v)
		}
		text = t
	}
	var path string
	if v, has := obj[AssetPathProperty]; has {
		p, ok := v.(string)
		if !ok {
			return &Asset{}, false, fmt.Errorf("unexpected asset path of type %T", v)
		}
		path = p
	}
	var uri string
	if v, has := obj[AssetURIProperty]; has {
		u, ok := v.(string)
		if !ok {
			return &Asset{}, false, fmt.Errorf("unexpected asset URI of type %T", v)
		}
		uri = u
	}

	return &Asset{Sig: AssetSig, Hash: hash, Text: text, Path: path, URI: uri}, true, nil
}

// HasContents indicates whether or not an asset's contents can be read.
func (a *Asset) HasContents() bool {
	return a.IsText() || a.IsPath() || a.IsURI()
}

// Bytes returns the contents of the asset as a byte slice.
func (a *Asset) Bytes() ([]byte, error) {
	// If this is a text asset, just return its bytes directly.
	if text, istext := a.GetText(); istext {
		return []byte(text), nil
	}

	blob, err := a.Read()
	if err != nil {
		return nil, err
	}
	return io.ReadAll(blob)
}

// Read begins reading an asset.
func (a *Asset) Read() (*Blob, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return a.ReadWithWD(wd)
}

// ReadWithWD begins reading an asset.
func (a *Asset) ReadWithWD(wd string) (*Blob, error) {
	if a.IsText() {
		return a.readText()
	} else if a.IsPath() {
		return a.readPath(wd)
	} else if a.IsURI() {
		return a.readURI()
	}
	return nil, errors.New("unrecognized asset type")
}

func (a *Asset) readText() (*Blob, error) {
	text, istext := a.GetText()
	contract.Assertf(istext, "Expected a text-based asset")
	return NewByteBlob([]byte(text)), nil
}

func (a *Asset) readPath(wd string) (*Blob, error) {
	path, ispath := a.GetPath()
	contract.Assertf(ispath, "Expected a path-based asset")

	if !filepath.IsAbs(path) {
		path = filepath.Join(wd, path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open asset file '%v': %w", path, err)
	}

	// Do a quick check to make sure it's a file, so we can fail gracefully if someone passes a directory.
	info, err := file.Stat()
	if err != nil {
		contract.IgnoreClose(file)
		return nil, fmt.Errorf("failed to stat asset file '%v': %w", path, err)
	}
	if info.IsDir() {
		contract.IgnoreClose(file)
		return nil, fmt.Errorf("asset path '%v' is a directory; try using an archive", path)
	}

	blob := &Blob{
		rd: file,
		sz: info.Size(),
	}
	return blob, nil
}

func (a *Asset) readURI() (*Blob, error) {
	url, isURL, err := a.GetURIURL()
	if err != nil {
		return nil, err
	}
	contract.Assertf(isURL, "Expected a URI-based asset")
	switch s := url.Scheme; s {
	case "http", "https":
		resp, err := httputil.GetWithRetry(url.String(), http.DefaultClient)
		if err != nil {
			return nil, err
		}
		return NewReadCloserBlob(resp.Body)
	case "file":
		contract.Assertf(url.User == nil, "file:// URIs cannot have a user: %v", url)
		contract.Assertf(url.RawQuery == "", "file:// URIs cannot have a query string: %v", url)
		contract.Assertf(url.Fragment == "", "file:// URIs cannot have a fragment: %v", url)
		if url.Host != "" && url.Host != "localhost" {
			return nil, fmt.Errorf("file:// host '%v' not supported (only localhost)", url.Host)
		}
		f, err := os.Open(url.Path)
		if err != nil {
			return nil, err
		}
		return NewFileBlob(f)
	default:
		return nil, fmt.Errorf("Unrecognized or unsupported URI scheme: %v", s)
	}
}

// EnsureHash computes the SHA256 hash of the asset's contents and stores it on the object.
func (a *Asset) EnsureHash() error {
	if a.Hash == "" {
		blob, err := a.Read()
		if err != nil {
			return err
		}
		defer contract.IgnoreClose(blob)

		hash := sha256.New()
		n, err := io.Copy(hash, blob)
		if err != nil {
			return err
		}
		if n != blob.Size() {
			return fmt.Errorf("incorrect blob size: expected %v, got %v", blob.Size(), n)
		}
		a.Hash = hex.EncodeToString(hash.Sum(nil))
	}
	return nil
}

// EnsureHash computes the SHA256 hash of the asset's contents and stores it on the object.
func (a *Asset) EnsureHashWithWD(wd string) error {
	if a.Hash == "" {
		blob, err := a.ReadWithWD(wd)
		if err != nil {
			return err
		}
		defer contract.IgnoreClose(blob)

		hash := sha256.New()
		n, err := io.Copy(hash, blob)
		if err != nil {
			return err
		}
		if n != blob.Size() {
			return fmt.Errorf("incorrect blob size: expected %v, got %v", blob.Size(), n)
		}
		a.Hash = hex.EncodeToString(hash.Sum(nil))
	}
	return nil
}

// Blob is a blob that implements ReadCloser and offers Len functionality.
type Blob struct {
	rd io.ReadCloser // an underlying reader.
	sz int64         // the size of the blob.
}

func (blob *Blob) Close() error               { return blob.rd.Close() }
func (blob *Blob) Read(p []byte) (int, error) { return blob.rd.Read(p) }
func (blob *Blob) Size() int64                { return blob.sz }

// NewByteBlob creates a new byte blob.
func NewByteBlob(data []byte) *Blob {
	return &Blob{
		rd: io.NopCloser(bytes.NewReader(data)),
		sz: int64(len(data)),
	}
}

// NewFileBlob creates a new asset blob whose size is known thanks to stat.
func NewFileBlob(f *os.File) (*Blob, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return &Blob{
		rd: f,
		sz: stat.Size(),
	}, nil
}

// NewReadCloserBlob turn any old ReadCloser into an Blob, usually by making a copy.
func NewReadCloserBlob(r io.ReadCloser) (*Blob, error) {
	if f, isf := r.(*os.File); isf {
		// If it's a file, we can "fast path" the asset creation without making a copy.
		return NewFileBlob(f)
	}
	// Otherwise, read it all in, and create a blob out of that.
	defer contract.IgnoreClose(r)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return NewByteBlob(data), nil
}

func NewRawBlob(r io.ReadCloser, size int64) *Blob {
	return &Blob{rd: r, sz: size}
}
