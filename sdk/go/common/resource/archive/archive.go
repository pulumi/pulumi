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

package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
)

const (
	// BookkeepingDir is the name of our bookkeeping folder, we store state here (like .git for git).
	// Copied from workspace.BookkeepingDir to break import cycle.
	BookkeepingDir = ".pulumi"
)

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
	ArchiveSig            = sig.ArchiveSig
	ArchiveHashProperty   = "hash"   // the dynamic property for an archive's hash.
	ArchiveAssetsProperty = "assets" // the dynamic property for an archive's assets.
	ArchivePathProperty   = "path"   // the dynamic property for an archive's path.
	ArchiveURIProperty    = "uri"    // the dynamic property for an archive's URI.
)

func FromAssetsWithWD(assets map[string]interface{}, wd string) (*Archive, error) {
	if assets == nil {
		// when provided assets are nil, create an empty archive
		assets = make(map[string]interface{})
	}

	// Ensure all elements are either assets or archives.
	for _, a := range assets {
		switch t := a.(type) {
		case *asset.Asset, *Archive:
			// ok
		default:
			return &Archive{}, fmt.Errorf("type %v is not a valid archive element", t)
		}
	}
	a := &Archive{Sig: ArchiveSig, Assets: assets}
	err := a.EnsureHashWithWD(wd)
	return a, err
}

func FromAssets(assets map[string]interface{}) (*Archive, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return FromAssetsWithWD(assets, wd)
}

func FromPath(path string) (*Archive, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return FromPathWithWD(path, wd)
}

func FromPathWithWD(path string, wd string) (*Archive, error) {
	if path == "" {
		return nil, errors.New("path cannot be empty when constructing a path archive")
	}
	a := &Archive{Sig: ArchiveSig, Path: path}
	err := a.EnsureHashWithWD(wd)
	return a, err
}

func FromURI(uri string) (*Archive, error) {
	if uri == "" {
		return nil, errors.New("uri cannot be empty when constructing a URI archive")
	}
	a := &Archive{Sig: ArchiveSig, URI: uri}
	err := a.EnsureHash()
	return a, err
}

func (a *Archive) IsAssets() bool {
	if a.IsPath() || a.IsURI() {
		return false
	}
	if len(a.Assets) > 0 {
		return true
	}
	// We can't easily tell the difference between an Archive that really is empty and one that has no contents at all.
	// If we have a hash we can check if that's the "zero hash" and if so then we know the assets is just empty. If the
	// hash does not equal the empty hash then we know this is a _placeholder_ archive where the contents are just
	// currently not known. If we don't have a hash then we can't tell the difference and assume it's just empty.
	if a.Hash == "" || a.Hash == "5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef" {
		return true
	}
	return false
}
func (a *Archive) IsPath() bool { return a.Path != "" }
func (a *Archive) IsURI() bool  { return a.URI != "" }

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
		sig.Key: ArchiveSig,
	}
	if a.Hash != "" {
		result[ArchiveHashProperty] = a.Hash
	}
	if a.Assets != nil {
		assets := make(map[string]interface{})
		for k, v := range a.Assets {
			switch t := v.(type) {
			case *asset.Asset:
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
func Deserialize(obj map[string]interface{}) (*Archive, bool, error) {
	// If not an archive, return false immediately.
	if obj[sig.Key] != ArchiveSig {
		return &Archive{}, false, nil
	}

	var hash string
	if v, has := obj[ArchiveHashProperty]; has {
		h, ok := v.(string)
		if !ok {
			return &Archive{}, false, fmt.Errorf("unexpected archive hash of type %T", v)
		}
		hash = h
	}

	if path, has := obj[ArchivePathProperty]; has {
		pathValue, ok := path.(string)
		if !ok {
			return &Archive{}, false, fmt.Errorf("unexpected archive path of type %T", path)
		}

		if pathValue != "" {
			pathArchive := &Archive{Sig: ArchiveSig, Path: pathValue, Hash: hash}
			return pathArchive, true, nil
		}
	}

	if uri, has := obj[ArchiveURIProperty]; has {
		uriValue, ok := uri.(string)
		if !ok {
			return &Archive{}, false, fmt.Errorf("unexpected archive URI of type %T", uri)
		}

		if uriValue != "" {
			uriArchive := &Archive{Sig: ArchiveSig, URI: uriValue, Hash: hash}
			return uriArchive, true, nil
		}
	}

	if assetsMap, has := obj[ArchiveAssetsProperty]; has {
		m, ok := assetsMap.(map[string]interface{})
		if !ok {
			return &Archive{}, false, fmt.Errorf("unexpected archive contents of type %T", assetsMap)
		}

		assets := make(map[string]interface{})
		for k, elem := range m {
			switch t := elem.(type) {
			case *asset.Asset:
				assets[k] = t
			case *Archive:
				assets[k] = t
			case map[string]interface{}:
				a, isa, err := asset.Deserialize(t)
				if err != nil {
					return &Archive{}, false, err
				} else if isa {
					assets[k] = a
				} else {
					arch, isarch, err := Deserialize(t)
					if err != nil {
						return &Archive{}, false, err
					} else if !isarch {
						return &Archive{}, false, fmt.Errorf("archive member '%v' is not an asset or archive", k)
					}
					assets[k] = arch
				}
			default:
				return &Archive{}, false, fmt.Errorf("archive member '%v' is not an asset or archive", k)
			}
		}

		assetArchive := &Archive{Sig: ArchiveSig, Assets: assets, Hash: hash}
		return assetArchive, true, nil
	}

	// if we reached here, it means the archive is empty,
	// we didn't find a non-zero path, non-zero uri nor non-nil assets
	// we will consider this to be an empty assets archive then with zero assets
	return &Archive{Sig: ArchiveSig, Assets: make(map[string]interface{}), Hash: hash}, true, nil
}

// HasContents indicates whether or not an archive's contents can be read.
func (a *Archive) HasContents() bool {
	return a.IsAssets() || a.IsPath() || a.IsURI()
}

// Reader presents the contents of an archive as a stream of named blobs.
type Reader interface {
	// Next returns the name and contents of the next member of the archive. If there are no more members in the
	// archive, this function returns ("", nil, io.EOF). The blob returned by a call to Next() must be read in full
	// before the next call to Next().
	Next() (string, *asset.Blob, error)

	// Close terminates the stream.
	Close() error
}

// Open returns an ArchiveReader that can be used to iterate over the named blobs that comprise the archive.
func (a *Archive) Open() (Reader, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return a.OpenWithWD(wd)
}

// Open returns an ArchiveReader that can be used to iterate over the named blobs that comprise the archive.
func (a *Archive) OpenWithWD(wd string) (Reader, error) {
	contract.Assertf(a.HasContents(), "cannot read an archive that has no contents")
	if a.IsAssets() {
		return a.readAssets(wd)
	} else if a.IsPath() {
		return a.readPath(wd)
	} else if a.IsURI() {
		return a.readURI()
	}
	return nil, errors.New("unrecognized archive type")
}

// assetsArchiveReader is used to read an Assets archive.
type assetsArchiveReader struct {
	assets      map[string]interface{}
	keys        []string
	archive     Reader
	archiveRoot string
	wd          string
}

func (r *assetsArchiveReader) Next() (string, *asset.Blob, error) {
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

		switch t := r.assets[name].(type) {
		case *asset.Asset:
			// An asset can be produced directly.
			blob, err := t.ReadWithWD(r.wd)
			if err != nil {
				return "", nil, fmt.Errorf("failed to expand archive asset '%v': %w", name, err)
			}
			return name, blob, nil
		case *Archive:
			// An archive must be flattened into its constituent blobs. Open the archive for reading and loop.
			archive, err := t.OpenWithWD(r.wd)
			if err != nil {
				return "", nil, fmt.Errorf("failed to expand sub-archive '%v': %w", name, err)
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

func (a *Archive) readAssets(wd string) (Reader, error) {
	// To read a map-based archive, just produce a map from each asset to its associated reader.
	m, isassets := a.GetAssets()
	contract.Assertf(isassets, "Expected an asset map-based archive")

	// Calculate and sort the list of member names s.t. it is deterministically orderered.
	keys := slice.Prealloc[string](len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	r := &assetsArchiveReader{
		assets: m,
		keys:   keys,
		wd:     wd,
	}
	return r, nil
}

// directoryArchiveReader is used to read an archive that is represented by a directory in the host filesystem.
type directoryArchiveReader struct {
	directoryPath string
	assetPaths    []string
}

func (r *directoryArchiveReader) Next() (string, *asset.Blob, error) {
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
	blob, err := (&asset.Asset{Path: assetPath}).Read()
	if err != nil {
		return "", nil, err
	}
	return name, blob, nil
}

func (r *directoryArchiveReader) Close() error {
	return nil
}

func (a *Archive) readPath(wd string) (Reader, error) {
	// To read a path-based archive, read that file and use its extension to ascertain what format to use.
	path, ispath := a.GetPath()
	contract.Assertf(ispath, "Expected a path-based asset")

	if !filepath.IsAbs(path) {
		path = filepath.Join(wd, path)
	}

	format := detectArchiveFormat(path)

	if format == NotArchive {
		// If not an archive, it could be a directory; if so, simply expand it out uncompressed as an archive.
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("couldn't read archive path '%v': %w", path, err)
		} else if !info.IsDir() {
			return nil, fmt.Errorf("'%v' is neither a recognized archive type nor a directory", path)
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

func (a *Archive) readURI() (Reader, error) {
	// To read a URI-based archive, fetch the contents remotely and use the extension to pick the format to use.
	url, isURL, err := a.GetURIURL()
	if err != nil {
		return nil, err
	}
	contract.Assertf(isURL, "Expected a URI-based asset")

	format := detectArchiveFormat(url.Path)
	if format == NotArchive {
		// IDEA: support (a) hints and (b) custom providers that default to certain formats.
		return nil, fmt.Errorf("file at URL '%v' is not a recognized archive format", url)
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
		contract.Assertf(url.Host == "", "file:// URIs cannot have a host: %v", url)
		contract.Assertf(url.User == nil, "file:// URIs cannot have a user: %v", url)
		contract.Assertf(url.RawQuery == "", "file:// URIs cannot have a query string: %v", url)
		contract.Assertf(url.Fragment == "", "file:// URIs cannot have a fragment: %v", url)
		return os.Open(url.Path)
	default:
		return nil, fmt.Errorf("Unrecognized or unsupported URI scheme: %v", s)
	}
}

// Bytes fetches the archive contents as a byte slices.  This is almost certainly the least efficient way to deal with
// the underlying streaming capabilities offered by assets and archives, but can be used in a pinch to interact with
// APIs that demand []bytes.
func (a *Archive) Bytes(format Format) ([]byte, error) {
	var data bytes.Buffer
	if err := a.Archive(format, &data); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// Archive produces a single archive stream in the desired format.  It prefers to return the archive with as little
// copying as is feasible, however if the desired format is different from the source, it will need to translate.
func (a *Archive) Archive(format Format, w io.Writer) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	return a.ArchiveWithWD(format, w, wd)
}

// Archive produces a single archive stream in the desired format.  It prefers to return the archive with as little
// copying as is feasible, however if the desired format is different from the source, it will need to translate.
func (a *Archive) ArchiveWithWD(format Format, w io.Writer, wd string) error {
	// If the source format is the same, just return that.
	if sf, ss, err := a.ReadSourceArchiveWithWD(wd); sf != NotArchive && sf == format {
		if err != nil {
			return err
		}
		_, err := io.Copy(w, ss)
		return err
	}

	switch format {
	case TarArchive:
		return a.archiveTar(w, wd)
	case TarGZIPArchive:
		return a.archiveTarGZIP(w, wd)
	case ZIPArchive:
		return a.archiveZIP(w, wd)
	default:
		contract.Failf("Illegal archive type: %v", format)
		return nil
	}
}

// addNextFileToTar adds the next file in the given archive to the given tar file. Returns io.EOF if the archive
// contains no more files.
func addNextFileToTar(r Reader, tw *tar.Writer, seenFiles map[string]bool) error {
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
		Mode: 0o600,
		Size: sz,
	}); err != nil {
		return err
	}
	n, err := io.Copy(tw, data)
	if err == tar.ErrWriteTooLong {
		return fmt.Errorf("incorrect blob size for %v: expected %v, got %v: %w", file, sz, n, err)
	}
	// tar expect us to write all the bytes we said we would write. If copy doesn't write Size bytes to the
	// tar file then we'll get a "missed writing X bytes" error on the next call to WriteHeader or Flush.
	if n != sz {
		return fmt.Errorf("incorrect blob size for %v: expected %v, got %v", file, sz, n)
	}
	return err
}

func (a *Archive) archiveTar(w io.Writer, wd string) error {
	// Open the archive.
	reader, err := a.OpenWithWD(wd)
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

func (a *Archive) archiveTarGZIP(w io.Writer, wd string) error {
	z := gzip.NewWriter(w)
	return a.archiveTar(z, wd)
}

// addNextFileToZIP adds the next file in the given archive to the given ZIP file. Returns io.EOF if the archive
// contains no more files.
func addNextFileToZIP(r Reader, zw *zip.Writer, seenFiles map[string]bool) error {
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
	fh := &zip.FileHeader{
		// These are the two fields set by zw.Create()
		Name:   file,
		Method: zip.Deflate,
	}

	// Set a nonzero -- but constant -- modification time. Otherwise, some agents (e.g. Azure
	// websites) can't extract the resulting archive. The date is comfortably after 1980 because
	// the ZIP format includes a date representation that starts at 1980.
	fh.Modified = time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC)

	fw, err := zw.CreateHeader(fh)
	if err != nil {
		return err
	}
	n, err := io.Copy(fw, data)
	if err != nil {
		return err
	}
	if n != sz {
		return fmt.Errorf("incorrect blob size for %v: expected %v, got %v", file, sz, n)
	}
	return nil
}

func (a *Archive) archiveZIP(w io.Writer, wd string) error {
	// Open the archive.
	reader, err := a.OpenWithWD(wd)
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
func (a *Archive) ReadSourceArchive() (Format, io.ReadCloser, error) {
	wd, err := os.Getwd()
	if err != nil {
		return NotArchive, nil, err
	}
	return a.ReadSourceArchiveWithWD(wd)
}

// ReadSourceArchiveWithWD returns a stream to the underlying archive, if there is one.
func (a *Archive) ReadSourceArchiveWithWD(wd string) (Format, io.ReadCloser, error) {
	if path, ispath := a.GetPath(); ispath {
		if !filepath.IsAbs(path) {
			path = filepath.Join(wd, path)
		}
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
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	return a.EnsureHashWithWD(wd)
}

// EnsureHash computes the SHA256 hash of the archive's contents and stores it on the object.
func (a *Archive) EnsureHashWithWD(wd string) error {
	contract.Requiref(wd != "", "wd", "must not be empty")

	if a.Hash == "" {
		hash := sha256.New()

		// Attempt to compute the hash in the most efficient way.  First try to open the archive directly and copy it
		// to the hash.  This avoids traversing any of the contents and just treats it as a byte stream.
		f, r, err := a.ReadSourceArchiveWithWD(wd)
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
			err := a.ArchiveWithWD(TarArchive, hash, wd)
			if err != nil {
				return err
			}
		}

		// Finally, encode the resulting hash as a string and we're done.
		a.Hash = hex.EncodeToString(hash.Sum(nil))
	}
	return nil
}

// Format indicates what archive and/or compression format an archive uses.
type Format int

const (
	NotArchive     = iota // not an archive.
	TarArchive            // a POSIX tar archive.
	TarGZIPArchive        // a POSIX tar archive that has been subsequently compressed using GZip.
	ZIPArchive            // a multi-file ZIP archive.
	JARArchive            // a Java JAR file
)

// ArchiveExts maps from a file extension and its associated archive and/or compression format.
var ArchiveExts = map[string]Format{
	".tar":    TarArchive,
	".tgz":    TarGZIPArchive,
	".tar.gz": TarGZIPArchive,
	".zip":    ZIPArchive,
	".jar":    JARArchive,
}

// detectArchiveFormat takes a path and infers its archive format based on the file extension.
func detectArchiveFormat(path string) Format {
	for ext, typ := range ArchiveExts {
		if strings.HasSuffix(path, ext) {
			return typ
		}
	}

	return NotArchive
}

// readArchive takes a stream to an existing archive and returns a map of names to readers for the inner assets.
// The routine returns an error if something goes wrong and, no matter what, closes the stream before returning.
func readArchive(ar io.ReadCloser, format Format) (Reader, error) {
	switch format {
	case TarArchive:
		return readTarArchive(ar)
	case TarGZIPArchive:
		return readTarGZIPArchive(ar)
	case ZIPArchive, JARArchive:
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
		} else if data, err := io.ReadAll(ar); err != nil {
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

func (r *tarArchiveReader) Next() (string, *asset.Blob, error) {
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
			data := asset.NewRawBlob(io.NopCloser(r.tr), file.Size)
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

func readTarArchive(ar io.ReadCloser) (Reader, error) {
	r := &tarArchiveReader{
		ar: ar,
		tr: tar.NewReader(ar),
	}
	return r, nil
}

func readTarGZIPArchive(ar io.ReadCloser) (Reader, error) {
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

func (r *zipArchiveReader) Next() (string, *asset.Blob, error) {
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
			return "", nil, fmt.Errorf("failed to read ZIP inner file %v: %w", file.Name, err)
		}

		if file.UncompressedSize64 > math.MaxInt64 {
			return "", nil, fmt.Errorf("file %v is too large to read", file.Name)
		}
		//nolint:gosec // uint64 -> int64 overflow is checked above.
		blob := asset.NewRawBlob(body, int64(file.UncompressedSize64))
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

func readZIPArchive(ar io.ReaderAt, size int64) (Reader, error) {
	zr, err := zip.NewReader(ar, size)
	if err != nil {
		return nil, fmt.Errorf("failed to read ZIP: %w", err)
	}

	r := &zipArchiveReader{
		ar: ar,
		zr: zr,
	}
	return r, nil
}
