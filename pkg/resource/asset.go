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
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/httputil"
)

const (
	// BookkeepingDir is the name of our bookkeeping folder, we store state here (like .git for git).
	// Copied from workspace.BookkeepingDir to break import cycle.
	BookkeepingDir = ".pulumi"
)

// Asset is a serialized asset reference.  It is a union: thus, only one of its fields will be non-nil.  Several helper
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
	AssetSig          = "c44067f5952c0a294b673a41bacd8c17" // a randomly assigned type hash for assets.
	AssetHashProperty = "hash"                             // the dynamic property for an asset's hash.
	AssetTextProperty = "text"                             // the dynamic property for an asset's text.
	AssetPathProperty = "path"                             // the dynamic property for an asset's path.
	AssetURIProperty  = "uri"                              // the dynamic property for an asset's URI.
)

// NewTextAsset produces a new asset and its corresponding SHA256 hash from the given text.
func NewTextAsset(text string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, Text: text}
	err := a.EnsureHash()
	return a, err
}

// NewPathAsset produces a new asset and its corresponding SHA256 hash from the given filesystem path.
func NewPathAsset(path string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, Path: path}
	err := a.EnsureHash()
	return a, err
}

// NewURIAsset produces a new asset and its corresponding SHA256 hash from the given network URI.
func NewURIAsset(uri string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, URI: uri}
	err := a.EnsureHash()
	return a, err
}

func (a *Asset) IsText() bool { return !a.IsPath() && !a.IsURI() }
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

var (
	functionRegexp    = regexp.MustCompile(`function __.*`)
	withRegexp        = regexp.MustCompile(`    with\({ .* }\) {`)
	environmentRegexp = regexp.MustCompile(`  }\).apply\(.*\).apply\(this, arguments\);`)
	preambleRegexp    = regexp.MustCompile(
		`function __.*\(\) {\n  return \(function\(\) {\n    with \(__closure\) {\n\nreturn `)
	postambleRegexp = regexp.MustCompile(
		`;\n\n    }\n  }\).apply\(__environment\).apply\(this, arguments\);\n}`)
)

// IsUserProgramCode checks to see if this is the special asset containing the users's code
func (a *Asset) IsUserProgramCode() bool {
	if !a.IsText() {
		return false
	}

	text := a.Text

	return functionRegexp.MatchString(text) &&
		withRegexp.MatchString(text) &&
		environmentRegexp.MatchString(text)
}

// MassageIfUserProgramCodeAsset takes the text for a function and cleans it up a bit to make the
// user visible diffs less noisy.  Specifically:
//   1. it tries to condense things by changling multiple blank lines into a single blank line.
//   2. it normalizs the sha hashes we emit so that changes to them don't appear in the diff.
//   3. it elides the with-capture headers, as changes there are not generally meaningful.
//
// TODO(https://github.com/pulumi/pulumi/issues/592) this is baking in a lot of knowledge about
// pulumi serialized functions.  We should try to move to an alternative mode that isn't so brittle.
// Options include:
//   1. Have a documented delimeter format that plan.go will look for.  Have the function serializer
//      emit those delimeters around code that should be ignored.
//   2. Have our resource generation code supply not just the resource, but the "user presentable"
//      resource that cuts out a lot of cruft.  We could then just diff that content here.
func MassageIfUserProgramCodeAsset(asset *Asset, debug bool) *Asset {
	if debug {
		return asset
	}

	// Only do this for strings that match our serialized function pattern.
	if !asset.IsUserProgramCode() {
		return asset
	}

	text := asset.Text
	replaceNewlines := func() {
		for {
			newText := strings.Replace(text, "\n\n\n", "\n\n", -1)
			if len(newText) == len(text) {
				break
			}

			text = newText
		}
	}

	replaceNewlines()

	firstFunc := functionRegexp.FindStringIndex(text)
	text = text[firstFunc[0]:]

	text = withRegexp.ReplaceAllString(text, "    with (__closure) {")
	text = environmentRegexp.ReplaceAllString(text, "  }).apply(__environment).apply(this, arguments);")

	text = preambleRegexp.ReplaceAllString(text, "")
	text = postambleRegexp.ReplaceAllString(text, "")

	replaceNewlines()

	return &Asset{Text: text}
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
		SigKey: AssetSig,
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
func DeserializeAsset(obj map[string]interface{}) (*Asset, bool, error) {
	// If not an asset, return false immediately.
	if obj[SigKey] != AssetSig {
		return &Asset{}, false, nil
	}

	// Else, deserialize the possible fields.
	var hash string
	if v, has := obj[AssetHashProperty]; has {
		h, ok := v.(string)
		if !ok {
			return &Asset{}, false, errors.Errorf("unexpected asset hash of type %T", v)
		}
		hash = h
	}
	var text string
	if v, has := obj[AssetTextProperty]; has {
		t, ok := v.(string)
		if !ok {
			return &Asset{}, false, errors.Errorf("unexpected asset text of type %T", v)
		}
		text = t
	}
	var path string
	if v, has := obj[AssetPathProperty]; has {
		p, ok := v.(string)
		if !ok {
			return &Asset{}, false, errors.Errorf("unexpected asset path of type %T", v)
		}
		path = p
	}
	var uri string
	if v, has := obj[AssetURIProperty]; has {
		u, ok := v.(string)
		if !ok {
			return &Asset{}, false, errors.Errorf("unexpected asset URI of type %T", v)
		}
		uri = u
	}

	return &Asset{Hash: hash, Text: text, Path: path, URI: uri}, true, nil
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
	return ioutil.ReadAll(blob)
}

// Read begins reading an asset.
func (a *Asset) Read() (*Blob, error) {
	if a.IsText() {
		return a.readText()
	} else if a.IsPath() {
		return a.readPath()
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

func (a *Asset) readPath() (*Blob, error) {
	path, ispath := a.GetPath()
	contract.Assertf(ispath, "Expected a path-based asset")

	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open asset file '%v'", path)
	}

	// Do a quick check to make sure it's a file, so we can fail gracefully if someone passes a directory.
	info, err := file.Stat()
	if err != nil {
		contract.IgnoreClose(file)
		return nil, errors.Wrapf(err, "failed to stat asset file '%v'", path)
	}
	if info.IsDir() {
		contract.IgnoreClose(file)
		return nil, errors.Errorf("asset path '%v' is a directory; try using an archive", path)
	}

	blob := &Blob{
		rd: file,
		sz: info.Size(),
	}
	return blob, nil
}

func (a *Asset) readURI() (*Blob, error) {
	url, isurl, err := a.GetURIURL()
	if err != nil {
		return nil, err
	}
	contract.Assertf(isurl, "Expected a URI-based asset")
	switch s := url.Scheme; s {
	case "http", "https":
		resp, err := httputil.GetWithRetry(url.String(), http.DefaultClient)
		if err != nil {
			return nil, err
		}
		return NewReadCloserBlob(resp.Body)
	case "file":
		contract.Assert(url.User == nil)
		contract.Assert(url.RawQuery == "")
		contract.Assert(url.Fragment == "")
		if url.Host != "" && url.Host != "localhost" {
			return nil, errors.Errorf("file:// host '%v' not supported (only localhost)", url.Host)
		}
		f, err := os.Open(url.Path)
		if err != nil {
			return nil, err
		}
		return NewFileBlob(f)
	default:
		return nil, errors.Errorf("Unrecognized or unsupported URI scheme: %v", s)
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
		_, err = io.Copy(hash, blob)
		if err != nil {
			return err
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
		rd: ioutil.NopCloser(bytes.NewReader(data)),
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
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return NewByteBlob(data), nil
}

// Archive is a serialized archive reference.  It is a union: thus, only one of its fields will be non-nil.  Several
// helper routines exist as members in order to easily interact with archives of different kinds.
type Archive struct {
	// Sig is the unique archive type signature (see properties.go).
	Sig string `json:"4dabf18193072939515e22adb298388d" yaml:"4dabf18193072939515e22adb298388d"`
	// Hash contains the SHA256 hash of the archive's contents.
	Hash string `json:"hash,omitempty" yaml:"hash,omitempty"`
	// Assets, when non-nil, is a collection of other assets/archives.
	Assets map[string]interface{} `json:"assets,omitempty" yaml:"assets,omitempty"`
	// Path is a non-empty string representing a path to a file on the current filesystem, for file archives.
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	// URI is a non-empty URI (file://, http://, https://, etc), for URI-backed archives.
	URI string `json:"uri,omitempty" yaml:"uri,omitempty"`
}

const (
	ArchiveSig            = "0def7320c3a5731c473e5ecbe6d01bc7" // a randomly assigned archive type signature.
	ArchiveHashProperty   = "hash"                             // the dynamic property for an archive's hash.
	ArchiveAssetsProperty = "assets"                           // the dynamic property for an archive's assets.
	ArchivePathProperty   = "path"                             // the dynamic property for an archive's path.
	ArchiveURIProperty    = "uri"                              // the dynamic property for an archive's URI.
)

func NewAssetArchive(assets map[string]interface{}) (*Archive, error) {
	// Ensure all elements are either assets or archives.
	for _, asset := range assets {
		switch t := asset.(type) {
		case *Asset, *Archive:
			// ok
		default:
			return &Archive{}, errors.Errorf("type %v is not a valid archive element", t)
		}
	}
	a := &Archive{Sig: ArchiveSig, Assets: assets}
	err := a.EnsureHash()
	return a, err
}

func NewPathArchive(path string) (*Archive, error) {
	a := &Archive{Sig: ArchiveSig, Path: path}
	err := a.EnsureHash()
	return a, err
}

func NewURIArchive(uri string) (*Archive, error) {
	a := &Archive{Sig: ArchiveSig, URI: uri}
	err := a.EnsureHash()
	return a, err
}

func (a *Archive) IsAssets() bool { return a.Assets != nil }
func (a *Archive) IsPath() bool   { return a.Path != "" }
func (a *Archive) IsURI() bool    { return a.URI != "" }

func (a *Archive) GetAssets() (map[string]interface{}, bool) {
	if a.IsAssets() {
		return a.Assets, true
	}
	return nil, false
}

func (a *Archive) GetPath() (string, bool) {
	if a.IsPath() {
		return a.Path, true
	}
	return "", false
}

func (a *Archive) GetURI() (string, bool) {
	if a.IsURI() {
		return a.URI, true
	}
	return "", false
}

// GetURIURL returns the underlying URI as a parsed URL, provided it is one.  If there was an error parsing the URI, it
// will be returned as a non-nil error object.
func (a *Archive) GetURIURL() (*url.URL, bool, error) {
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
// if the contents of two archives come from different sources, they are treated as equal if their hashes match.
// Similarly, if the contents of two archives come from the same source but the archives have different hashes, the
// archives are not equal.
func (a *Archive) Equals(other *Archive) bool {
	if a == nil {
		return other == nil
	} else if other == nil {
		return false
	}

	// If we can't get a hash for both archives, treat them as differing.
	if err := a.EnsureHash(); err != nil {
		return false
	}
	if err := other.EnsureHash(); err != nil {
		return false
	}
	return a.Hash == other.Hash
}

// Serialize returns a weakly typed map that contains the right signature for serialization purposes.
func (a *Archive) Serialize() map[string]interface{} {
	result := map[string]interface{}{
		SigKey: ArchiveSig,
	}
	if a.Hash != "" {
		result[ArchiveHashProperty] = a.Hash
	}
	if a.Assets != nil {
		assets := make(map[string]interface{})
		for k, v := range a.Assets {
			switch t := v.(type) {
			case *Asset:
				assets[k] = t.Serialize()
			case *Archive:
				assets[k] = t.Serialize()
			default:
				contract.Failf("Unrecognized asset map type %v", reflect.TypeOf(t))
			}
		}
		result[ArchiveAssetsProperty] = assets
	}
	if a.Path != "" {
		result[ArchivePathProperty] = a.Path
	}
	if a.URI != "" {
		result[ArchiveURIProperty] = a.URI
	}
	return result
}

// DeserializeArchive checks to see if the map contains an archive, using its signature, and if so deserializes it.
func DeserializeArchive(obj map[string]interface{}) (*Archive, bool, error) {
	// If not an archive, return false immediately.
	if obj[SigKey] != ArchiveSig {
		return &Archive{}, false, nil
	}

	var hash string
	if v, has := obj[ArchiveHashProperty]; has {
		h, ok := v.(string)
		if !ok {
			return &Archive{}, false, errors.Errorf("unexpected archive hash of type %T", v)
		}
		hash = h
	}

	var assets map[string]interface{}
	if v, has := obj[ArchiveAssetsProperty]; has {
		assets = make(map[string]interface{})
		if v != nil {
			m, ok := v.(map[string]interface{})
			if !ok {
				return &Archive{}, false, errors.Errorf("unexpected archive contents of type %T", v)
			}

			for k, elem := range m {
				switch t := elem.(type) {
				case *Asset:
					assets[k] = t
				case *Archive:
					assets[k] = t
				case map[string]interface{}:
					a, isa, err := DeserializeAsset(t)
					if err != nil {
						return &Archive{}, false, err
					} else if isa {
						assets[k] = a
					} else {
						arch, isarch, err := DeserializeArchive(t)
						if err != nil {
							return &Archive{}, false, err
						} else if !isarch {
							return &Archive{}, false, errors.Errorf("archive member '%v' is not an asset or archive", k)
						}
						assets[k] = arch
					}
				default:
					return &Archive{}, false, errors.Errorf("archive member '%v' is not an asset or archive", k)
				}
			}
		}
	}
	var path string
	if v, has := obj[ArchivePathProperty]; has {
		p, ok := v.(string)
		if !ok {
			return &Archive{}, false, errors.Errorf("unexpected archive path of type %T", v)
		}
		path = p
	}
	var uri string
	if v, has := obj[ArchiveURIProperty]; has {
		u, ok := v.(string)
		if !ok {
			return &Archive{}, false, errors.Errorf("unexpected archive URI of type %T", v)
		}
		uri = u
	}

	return &Archive{Hash: hash, Assets: assets, Path: path, URI: uri}, true, nil
}

// HasContents indicates whether or not an archive's contents can be read.
func (a *Archive) HasContents() bool {
	return a.IsAssets() || a.IsPath() || a.IsURI()
}

// ArchiveReader presents the contents of an archive as a stream of named blobs.
type ArchiveReader interface {
	// Next returns the name and contents of the next member of the archive. If there are no more members in the
	// archive, this function returns ("", nil, io.EOF). The blob returned by a call to Next() must be read in full
	// before the next call to Next().
	Next() (string, *Blob, error)

	// Close terminates the stream.
	Close() error
}

// Open returns an ArchiveReader that can be used to iterate over the named blobs that comprise the archive.
func (a *Archive) Open() (ArchiveReader, error) {
	contract.Assertf(a.HasContents(), "cannot read an archive that has no contents")
	if a.IsAssets() {
		return a.readAssets()
	} else if a.IsPath() {
		return a.readPath()
	} else if a.IsURI() {
		return a.readURI()
	}
	return nil, errors.New("unrecognized archive type")
}

// assetsArchiveReader is used to read an Assets archive.
type assetsArchiveReader struct {
	assets      map[string]interface{}
	keys        []string
	archive     ArchiveReader
	archiveRoot string
}

func (r *assetsArchiveReader) Next() (string, *Blob, error) {
	for {
		// If we're currently flattening out a subarchive, first check to see if it has any more members. If it does,
		// return the next member.
		if r.archive != nil {
			name, blob, err := r.archive.Next()
			switch {
			case err == io.EOF:
				// The subarchive is complete. Nil it out and continue on.
				r.archive = nil
			case err != nil:
				// The subarchive produced a legitimate error; return it.
				return "", nil, err
			default:
				// The subarchive produced a valid blob. Return it.
				return filepath.Join(r.archiveRoot, name), blob, nil
			}
		}

		// If there are no more members in this archive, return io.EOF.
		if len(r.keys) == 0 {
			return "", nil, io.EOF
		}

		// Fetch the next key in the archive and slice it off of the list.
		name := r.keys[0]
		r.keys = r.keys[1:]

		asset := r.assets[name]
		switch t := asset.(type) {
		case *Asset:
			// An asset can be produced directly.
			blob, err := t.Read()
			if err != nil {
				return "", nil, errors.Wrapf(err, "failed to expand archive asset '%v'", name)
			}
			return name, blob, nil
		case *Archive:
			// An archive must be flattened into its constituent blobs. Open the archive for reading and loop.
			archive, err := t.Open()
			if err != nil {
				return "", nil, errors.Wrapf(err, "failed to expand sub-archive '%v'", name)
			}
			r.archive = archive
			r.archiveRoot = name
		}
	}
}

func (r *assetsArchiveReader) Close() error {
	if r.archive != nil {
		return r.archive.Close()
	}
	return nil
}

func (a *Archive) readAssets() (ArchiveReader, error) {
	// To read a map-based archive, just produce a map from each asset to its associated reader.
	m, isassets := a.GetAssets()
	contract.Assertf(isassets, "Expected an asset map-based archive")

	// Calculate and sort the list of member names s.t. it is deterministically orderered.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	r := &assetsArchiveReader{
		assets: m,
		keys:   keys,
	}
	return r, nil
}

// directoryArchiveReader is used to read an archive that is represented by a directory in the host filesystem.
type directoryArchiveReader struct {
	directoryPath string
	assetPaths    []string
}

func (r *directoryArchiveReader) Next() (string, *Blob, error) {
	// If there are no more members in this archive, return io.EOF.
	if len(r.assetPaths) == 0 {
		return "", nil, io.EOF
	}

	// Fetch the next path in the archive and slice it off of the list.
	assetPath := r.assetPaths[0]
	r.assetPaths = r.assetPaths[1:]

	// Crop the asset's path s.t. it is relative to the directory path.
	name, err := filepath.Rel(r.directoryPath, assetPath)
	if err != nil {
		return "", nil, err
	}
	name = filepath.Clean(name)

	// Replace Windows separators with Linux ones (ToSlash is a no-op on Linux)
	name = filepath.ToSlash(name)

	// Open and return the blob.
	blob, err := (&Asset{Path: assetPath}).Read()
	if err != nil {
		return "", nil, err
	}
	return name, blob, nil
}

func (r *directoryArchiveReader) Close() error {
	return nil
}

func (a *Archive) readPath() (ArchiveReader, error) {
	// To read a path-based archive, read that file and use its extension to ascertain what format to use.
	path, ispath := a.GetPath()
	contract.Assertf(ispath, "Expected a path-based asset")

	format := detectArchiveFormat(path)

	if format == NotArchive {
		// If not an archive, it could be a directory; if so, simply expand it out uncompressed as an archive.
		info, err := os.Stat(path)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't read archive path '%v'", path)
		} else if !info.IsDir() {
			return nil, errors.Errorf("'%v' is neither a recognized archive type nor a directory", path)
		}

		// Accumulate the list of asset paths. This list is ordered deterministically by filepath.Walk.
		assetPaths := []string{}
		if walkerr := filepath.Walk(path, func(filePath string, f os.FileInfo, fileerr error) error {
			// If there was an error, exit.
			if fileerr != nil {
				return fileerr
			}

			// If this is a .pulumi directory, we will skip this by default.
			// TODO[pulumi/pulumi#122]: when we support .pulumiignore, this will be customizable.
			if f.Name() == BookkeepingDir {
				if f.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// If this was a directory, skip it.
			if f.IsDir() {
				return nil
			}

			// If this is a symlink and it points at a directory, skip it. Otherwise continue along. This will mean
			// that the file will be added to the list of files to archive. When you go to read this archive, you'll
			// get a copy of the file (instead of a symlink) to some other file in the archive.
			if f.Mode()&os.ModeSymlink != 0 {
				fileInfo, statErr := os.Stat(filePath)
				if statErr != nil {
					return statErr
				}

				if fileInfo.IsDir() {
					return nil
				}
			}

			// Otherwise, add this asset to the list of paths and keep going.
			assetPaths = append(assetPaths, filePath)
			return nil
		}); walkerr != nil {
			return nil, walkerr
		}

		r := &directoryArchiveReader{
			directoryPath: path,
			assetPaths:    assetPaths,
		}
		return r, nil
	}

	// Otherwise, it's an archive file, and we will go ahead and open it up and read it.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return readArchive(file, format)
}

func (a *Archive) readURI() (ArchiveReader, error) {
	// To read a URI-based archive, fetch the contents remotely and use the extension to pick the format to use.
	url, isurl, err := a.GetURIURL()
	if err != nil {
		return nil, err
	}
	contract.Assertf(isurl, "Expected a URI-based asset")

	format := detectArchiveFormat(url.Path)
	if format == NotArchive {
		// IDEA: support (a) hints and (b) custom providers that default to certain formats.
		return nil, errors.Errorf("file at URL '%v' is not a recognized archive format", url)
	}

	ar, err := a.openURLStream(url)
	if err != nil {
		return nil, err
	}
	return readArchive(ar, format)
}

func (a *Archive) openURLStream(url *url.URL) (io.ReadCloser, error) {
	switch s := url.Scheme; s {
	case "http", "https":
		resp, err := httputil.GetWithRetry(url.String(), http.DefaultClient)
		if err != nil {
			return nil, err
		}
		return resp.Body, nil
	case "file":
		contract.Assert(url.Host == "")
		contract.Assert(url.User == nil)
		contract.Assert(url.RawQuery == "")
		contract.Assert(url.Fragment == "")
		return os.Open(url.Path)
	default:
		return nil, errors.Errorf("Unrecognized or unsupported URI scheme: %v", s)
	}
}

// Bytes fetches the archive contents as a byte slices.  This is almost certainly the least efficient way to deal with
// the underlying streaming capabilities offered by assets and archives, but can be used in a pinch to interact with
// APIs that demand []bytes.
func (a *Archive) Bytes(format ArchiveFormat) ([]byte, error) {
	var data bytes.Buffer
	if err := a.Archive(format, &data); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// Archive produces a single archive stream in the desired format.  It prefers to return the archive with as little
// copying as is feasible, however if the desired format is different from the source, it will need to translate.
func (a *Archive) Archive(format ArchiveFormat, w io.Writer) error {
	// If the source format is the same, just return that.
	if sf, ss, err := a.ReadSourceArchive(); sf != NotArchive && sf == format {
		if err != nil {
			return err
		}
		_, err := io.Copy(w, ss)
		return err
	}

	switch format {
	case TarArchive:
		return a.archiveTar(w)
	case TarGZIPArchive:
		return a.archiveTarGZIP(w)
	case ZIPArchive:
		return a.archiveZIP(w)
	default:
		contract.Failf("Illegal archive type: %v", format)
		return nil
	}
}

// addNextFileToTar adds the next file in the given archive to the given tar file. Returns io.EOF if the archive
// contains no more files.
func addNextFileToTar(r ArchiveReader, tw *tar.Writer, seenFiles map[string]bool) error {
	file, data, err := r.Next()
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(data)

	// It's possible to run into the same file multiple times in the list of archives we're passed.
	// For example, if there is an archive pointing to foo/bar and an archive pointing to
	// foo/bar/baz/quux.  Because of this only include the file the first time we see it.
	if _, has := seenFiles[file]; has {
		return nil
	}
	seenFiles[file] = true

	sz := data.Size()
	if err = tw.WriteHeader(&tar.Header{
		Name: file,
		Mode: 0600,
		Size: sz,
	}); err != nil {
		return err
	}
	n, err := io.Copy(tw, data)
	if err == tar.ErrWriteTooLong {
		return errors.Wrap(err, fmt.Sprintf("incorrect blob size for %v: expected %v, got %v", file, sz, n))
	}
	return err
}

func (a *Archive) archiveTar(w io.Writer) error {
	// Open the archive.
	reader, err := a.Open()
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(reader)

	// Now actually emit the contents, file by file.
	tw := tar.NewWriter(w)
	seenFiles := make(map[string]bool)
	for err == nil {
		err = addNextFileToTar(reader, tw, seenFiles)
	}
	if err != io.EOF {
		return err
	}

	return tw.Close()
}

func (a *Archive) archiveTarGZIP(w io.Writer) error {
	z := gzip.NewWriter(w)
	return a.archiveTar(z)
}

// addNextFileToZIP adds the next file in the given archive to the given ZIP file. Returns io.EOF if the archive
// contains no more files.
func addNextFileToZIP(r ArchiveReader, zw *zip.Writer, seenFiles map[string]bool) error {
	file, data, err := r.Next()
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(data)

	// It's possible to run into the same file multiple times in the list of archives we're passed.
	// For example, if there is an archive pointing to foo/bar and an archive pointing to
	// foo/bar/baz/quux.  Because of this only include the file the first time we see it.
	if _, has := seenFiles[file]; has {
		return nil
	}
	seenFiles[file] = true

	fh := &zip.FileHeader{
		// These are the two fields set by zw.Create()
		Name:   file,
		Method: zip.Deflate,
	}

	// Set a nonzero -- but constant -- modification time. Otherwise, some agents (e.g. Azure
	// websites) can't extract the resulting archive. The date is comfortably after 1980 because
	// the ZIP format includes a date representation that starts at 1980. Use `SetModTime` to
	// remain compatible with Go 1.9.
	// nolint: megacheck
	fh.SetModTime(time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC))

	fw, err := zw.CreateHeader(fh)
	if err != nil {
		return err
	}
	_, err = io.Copy(fw, data)
	return err
}

func (a *Archive) archiveZIP(w io.Writer) error {
	// Open the archive.
	reader, err := a.Open()
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(reader)

	// Now actually emit the contents, file by file.
	zw := zip.NewWriter(w)
	seenFiles := make(map[string]bool)
	for err == nil {
		err = addNextFileToZIP(reader, zw, seenFiles)
	}
	if err != io.EOF {
		return err
	}

	return zw.Close()
}

// ReadSourceArchive returns a stream to the underlying archive, if there is one.
func (a *Archive) ReadSourceArchive() (ArchiveFormat, io.ReadCloser, error) {
	if path, ispath := a.GetPath(); ispath {
		if format := detectArchiveFormat(path); format != NotArchive {
			f, err := os.Open(path)
			return format, f, err
		}
	} else if url, isurl, urlerr := a.GetURIURL(); urlerr == nil && isurl {
		if format := detectArchiveFormat(url.Path); format != NotArchive {
			s, err := a.openURLStream(url)
			return format, s, err
		}
	}
	return NotArchive, nil, nil
}

// EnsureHash computes the SHA256 hash of the archive's contents and stores it on the object.
func (a *Archive) EnsureHash() error {
	if a.Hash == "" {
		hash := sha256.New()

		// Attempt to compute the hash in the most efficient way.  First try to open the archive directly and copy it
		// to the hash.  This avoids traversing any of the contents and just treats it as a byte stream.
		f, r, err := a.ReadSourceArchive()
		if err != nil {
			return err
		}
		if f != NotArchive && r != nil {
			defer contract.IgnoreClose(r)
			_, err = io.Copy(hash, r)
			if err != nil {
				return err
			}
		} else {
			// Otherwise, it's not an archive; we'll need to transform it into one.  Pick tar since it avoids
			// any superfluous compression which doesn't actually help us in this situation.
			err := a.Archive(TarArchive, hash)
			if err != nil {
				return err
			}
		}

		// Finally, encode the resulting hash as a string and we're done.
		a.Hash = hex.EncodeToString(hash.Sum(nil))
	}
	return nil
}

// ArchiveFormat indicates what archive and/or compression format an archive uses.
type ArchiveFormat int

const (
	NotArchive     = iota // not an archive.
	TarArchive            // a POSIX tar archive.
	TarGZIPArchive        // a POSIX tar archive that has been subsequently compressed using GZip.
	ZIPArchive            // a multi-file ZIP archive.
)

// ArchiveExts maps from a file extension and its associated archive and/or compression format.
var ArchiveExts = map[string]ArchiveFormat{
	".tar":    TarArchive,
	".tgz":    TarGZIPArchive,
	".tar.gz": TarGZIPArchive,
	".zip":    ZIPArchive,
}

// detectArchiveFormat takes a path and infers its archive format based on the file extension.
func detectArchiveFormat(path string) ArchiveFormat {
	for ext, typ := range ArchiveExts {
		if strings.HasSuffix(path, ext) {
			return typ
		}
	}

	return NotArchive
}

// readArchive takes a stream to an existing archive and returns a map of names to readers for the inner assets.
// The routine returns an error if something goes wrong and, no matter what, closes the stream before returning.
func readArchive(ar io.ReadCloser, format ArchiveFormat) (ArchiveReader, error) {
	switch format {
	case TarArchive:
		return readTarArchive(ar)
	case TarGZIPArchive:
		return readTarGZIPArchive(ar)
	case ZIPArchive:
		// Unfortunately, the ZIP archive reader requires ReaderAt functionality.  If it's a file, we can recover this
		// with a simple stat.  Otherwise, we will need to go ahead and make a copy in memory.
		var ra io.ReaderAt
		var sz int64
		if f, isf := ar.(*os.File); isf {
			stat, err := f.Stat()
			if err != nil {
				return nil, err
			}
			ra = f
			sz = stat.Size()
		} else if data, err := ioutil.ReadAll(ar); err != nil {
			return nil, err
		} else {
			ra = bytes.NewReader(data)
			sz = int64(len(data))
		}
		return readZIPArchive(ra, sz)
	default:
		contract.Failf("Illegal archive type: %v", format)
		return nil, nil
	}
}

// tarArchiveReader is used to read an archive that is stored in tar format.
type tarArchiveReader struct {
	ar io.ReadCloser
	tr *tar.Reader
}

func (r *tarArchiveReader) Next() (string, *Blob, error) {
	for {
		file, err := r.tr.Next()
		if err != nil {
			return "", nil, err
		}

		switch file.Typeflag {
		case tar.TypeDir:
			continue // skip directories
		case tar.TypeReg:
			// Return the tar reader for this file's contents.
			data := &Blob{
				rd: ioutil.NopCloser(r.tr),
				sz: file.Size,
			}
			name := filepath.Clean(file.Name)
			return name, data, nil
		default:
			contract.Failf("Unrecognized tar header typeflag: %v", file.Typeflag)
		}
	}
}

func (r *tarArchiveReader) Close() error {
	return r.ar.Close()
}

func readTarArchive(ar io.ReadCloser) (ArchiveReader, error) {
	r := &tarArchiveReader{
		ar: ar,
		tr: tar.NewReader(ar),
	}
	return r, nil
}

func readTarGZIPArchive(ar io.ReadCloser) (ArchiveReader, error) {
	// First decompress the GZIP stream.
	gz, err := gzip.NewReader(ar)
	if err != nil {
		return nil, err
	}

	// Now read the tarfile.
	return readTarArchive(gz)
}

// zipArchiveReader is used to read an archive that is stored in ZIP format.
type zipArchiveReader struct {
	ar    io.ReaderAt
	zr    *zip.Reader
	index int
}

func (r *zipArchiveReader) Next() (string, *Blob, error) {
	for r.index < len(r.zr.File) {
		file := r.zr.File[r.index]
		r.index++

		// Skip directories, since they aren't included in TAR and other archives above.
		if file.FileInfo().IsDir() {
			continue
		}

		// Open the next file and return its blob.
		body, err := file.Open()
		if err != nil {
			return "", nil, errors.Wrapf(err, "failed to read ZIP inner file %v", file.Name)
		}
		blob := &Blob{
			rd: body,
			sz: int64(file.UncompressedSize64),
		}
		name := filepath.Clean(file.Name)
		return name, blob, nil
	}
	return "", nil, io.EOF
}

func (r *zipArchiveReader) Close() error {
	if c, ok := r.ar.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func readZIPArchive(ar io.ReaderAt, size int64) (ArchiveReader, error) {
	zr, err := zip.NewReader(ar, size)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read ZIP")
	}

	r := &zipArchiveReader{
		ar: ar,
		zr: zr,
	}
	return r, nil
}
