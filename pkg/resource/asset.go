// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Asset is a serialized asset reference.  It is a union: thus, only one of its fields will be non-nil.  Several helper
// routines exist as members in order to easily interact with the assets referenced by an instance of this type.
//nolint: lll
type Asset struct {
	Sig  string `json:"4dabf18193072939515e22adb298388d" yaml:"4dabf18193072939515e22adb298388d"` // the unique asset signature (see properties.go).
	Hash string `json:"hash,omitempty" yaml:"hash,omitempty"`                                     // the SHA1 hash of the asset contents.
	Text string `json:"text,omitempty" yaml:"text,omitempty"`                                     // a textual asset.
	Path string `json:"path,omitempty" yaml:"path,omitempty"`                                     // a file on the current filesystem.
	URI  string `json:"uri,omitempty" yaml:"uri,omitempty"`                                       // a URI (file://, http://, https://, or custom).
}

const (
	AssetSig          = "c44067f5952c0a294b673a41bacd8c17" // a randomly assigned type hash for assets.
	AssetHashProperty = "hash"                             // the dynamic property for an asset's hash.
	AssetTextProperty = "text"                             // the dynamic property for an asset's text.
	AssetPathProperty = "path"                             // the dynamic property for an asset's path.
	AssetURIProperty  = "uri"                              // the dynamic property for an asset's URI.
)

// NewTextAsset produces a new asset and its corresponding SHA1 hash from the given text.
func NewTextAsset(text string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, Text: text}
	err := a.EnsureHash()
	return a, err
}

// NewPathAsset produces a new asset and its corresponding SHA1 hash from the given filesystem path.
func NewPathAsset(path string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, Path: path}
	err := a.EnsureHash()
	return a, err
}

// NewURIAsset produces a new asset and its corresponding SHA1 hash from the given network URI.
func NewURIAsset(uri string) (*Asset, error) {
	a := &Asset{Sig: AssetSig, URI: uri}
	err := a.EnsureHash()
	return a, err
}

func (a *Asset) IsText() bool { return a.Text != "" }
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

// Equals returns true if a is value-equal to other.
func (a *Asset) Equals(other *Asset) bool {
	if a == nil {
		return other == nil
	} else if other == nil {
		return false
	}
	return a.Hash == other.Hash && a.Text == other.Text && a.Path == other.Path && a.URI == other.URI
}

// Serialize returns a weakly typed map that contains the right signature for serialization purposes.
func (a *Asset) Serialize() map[string]interface{} {
	result := map[string]interface{}{
		string(SigKey): AssetSig,
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
	if obj[string(SigKey)] != AssetSig {
		return &Asset{}, false, nil
	}

	// Else, deserialize the possible fields.
	var hash string
	if v, has := obj[AssetHashProperty]; has {
		hash = v.(string)
	}
	var text string
	if v, has := obj[AssetTextProperty]; has {
		text = v.(string)
	}
	var path string
	if v, has := obj[AssetPathProperty]; has {
		path = v.(string)
	}
	var uri string
	if v, has := obj[AssetURIProperty]; has {
		uri = v.(string)
	}
	if text == "" && path == "" && uri == "" {
		return &Asset{}, false, errors.New("asset is missing one of text, path, or URI")
	}

	return &Asset{Hash: hash, Text: text, Path: path, URI: uri}, true, nil
}

// Read reads an asset's contents into memory.
func (a *Asset) Read() (*Blob, error) {
	if a.IsText() {
		return a.readText()
	} else if a.IsPath() {
		return a.readPath()
	} else if a.IsURI() {
		return a.readURI()
	}
	contract.Failf("Invalid asset; one of Text, Path, or URI must be non-nil")
	return nil, nil
}

func (a *Asset) readText() (*Blob, error) {
	text, istext := a.GetText()
	contract.Assertf(istext, "Expected a text-based asset")
	return NewByteBlob([]byte(text)), nil
}

func (a *Asset) readPath() (*Blob, error) {
	path, ispath := a.GetPath()
	contract.Assertf(ispath, "Expected a path-based asset")

	// Do a quick check to make sure it's a file, so we can fail gracefully if someone passes a directory.
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open asset file '%v'", path)
	} else if info.IsDir() {
		return nil, errors.Errorf("asset path '%v' is a directory; try using an archive", path)
	}

	byts, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read asset file '%v'", path)
	}
	return NewByteBlob(byts), nil
}

func (a *Asset) readURI() (*Blob, error) {
	url, isurl, err := a.GetURIURL()
	if err != nil {
		return nil, err
	}
	contract.Assertf(isurl, "Expected a URI-based asset")
	switch s := url.Scheme; s {
	case "http", "https":
		resp, err := http.Get(url.String())
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

// EnsureHash computes the SHA1 hash of the asset's contents and stores it on the object.
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

// SeekableReadCloser combines Read, Close, and Seek functionality into one interface.
type SeekableReadCloser interface {
	io.Seeker
	io.ReadCloser
}

// Blob is a blob that implements ReadCloser, Seek, and offers Len functionality.
type Blob struct {
	rd SeekableReadCloser // an underlying reader.
	sz int64              // the size of the blob.
}

func (blob *Blob) Close() error                                 { return blob.rd.Close() }
func (blob *Blob) Read(p []byte) (int, error)                   { return blob.rd.Read(p) }
func (blob *Blob) Reader() SeekableReadCloser                   { return blob.rd }
func (blob *Blob) Seek(offset int64, whence int) (int64, error) { return blob.rd.Seek(offset, whence) }
func (blob *Blob) Size() int64                                  { return blob.sz }

// NewByteBlob creates a new byte blob.
func NewByteBlob(data []byte) *Blob {
	return &Blob{
		rd: newBytesReader(data),
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

// bytesReader turns a *bytes.Reader into a SeekableReadCloser by adding an empty Close method.
type bytesReader struct {
	*bytes.Reader
}

func newBytesReader(b []byte) SeekableReadCloser {
	return bytesReader{
		Reader: bytes.NewReader(b),
	}
}

func (b bytesReader) Close() error {
	return nil // intentionally blank
}

// Archive is a serialized archive reference.  It is a union: thus, only one of its fields will be non-nil.  Several
// helper routines exist as members in order to easily interact with archives of different kinds.
//nolint: lll
type Archive struct {
	Sig    string                 `json:"4dabf18193072939515e22adb298388d" yaml:"4dabf18193072939515e22adb298388d"` // the unique asset signature (see properties.go).
	Hash   string                 `json:"hash,omitempty" yaml:"hash,omitempty"`                                     // the SHA1 hash of the archive contents.
	Assets map[string]interface{} `json:"assets,omitempty" yaml:"assets,omitempty"`                                 // a collection of other assets/archives.
	Path   string                 `json:"path,omitempty" yaml:"path,omitempty"`                                     // a file on the current filesystem.
	URI    string                 `json:"uri,omitempty" yaml:"uri,omitempty"`                                       // a remote URI (file://, http://, https://, etc).
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

// Equals returns true if a is value-equal to other.
func (a *Archive) Equals(other *Archive) bool {
	if a == nil {
		return other == nil
	} else if other == nil {
		return false
	}
	if a.Assets != nil {
		if other.Assets == nil {
			return false
		}
		if len(a.Assets) != len(other.Assets) {
			return false
		}
		for key, value := range a.Assets {
			otherv := other.Assets[key]
			switch valuet := value.(type) {
			case *Asset:
				if othera, isAsset := otherv.(*Asset); isAsset {
					if !valuet.Equals(othera) {
						return false
					}
				} else {
					return false
				}
			case *Archive:
				if othera, isArchive := otherv.(*Archive); isArchive {
					if !valuet.Equals(othera) {
						return false
					}
				} else {
					return false
				}
			default:
				return false
			}
		}
	} else if other.Assets != nil {
		return false
	}
	return a.Hash == other.Hash && a.Path == other.Path && a.URI == other.URI
}

// Serialize returns a weakly typed map that contains the right signature for serialization purposes.
func (a *Archive) Serialize() map[string]interface{} {
	result := map[string]interface{}{
		string(SigKey): ArchiveSig,
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
	if obj[string(SigKey)] != ArchiveSig {
		return &Archive{}, false, nil
	}

	var hash string
	if v, has := obj[ArchiveHashProperty]; has {
		hash = v.(string)
	}

	var assets map[string]interface{}
	if v, has := obj[ArchiveAssetsProperty]; has {
		assets = make(map[string]interface{})
		if v != nil {
			for k, elem := range v.(map[string]interface{}) {
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
					return &Archive{}, false, nil
				}
			}
		}
	}
	var path string
	if v, has := obj[ArchivePathProperty]; has {
		path = v.(string)
	}
	var uri string
	if v, has := obj[ArchiveURIProperty]; has {
		uri = v.(string)
	}
	if assets == nil && path == "" && uri == "" {
		return &Archive{}, false, errors.New("archive is missing one of assets, path, or URI")
	}

	return &Archive{Hash: hash, Assets: assets, Path: path, URI: uri}, true, nil
}

// Read returns a map of asset name to its associated reader object (which can be used to perform reads/IO).
func (a *Archive) Read() (map[string]*Blob, error) {
	if a.IsAssets() {
		return a.readAssets()
	} else if a.IsPath() {
		return a.readPath()
	} else if a.IsURI() {
		return a.readURI()
	}
	contract.Failf("Invalid archive; one of Assets, Path, or URI must be non-nil")
	return nil, nil
}

func (a *Archive) readAssets() (map[string]*Blob, error) {
	// To read a map-based archive, just produce a map from each asset to its associated reader.
	m, isassets := a.GetAssets()
	contract.Assertf(isassets, "Expected an asset map-based archive")

	result := map[string]*Blob{}
	for name, asset := range m {
		switch t := asset.(type) {
		case *Asset:
			// An asset can be added directly to the result.
			blob, err := t.Read()
			if err != nil {
				return nil, errors.Wrapf(err, "failed to expand archive asset '%v'", name)
			}
			result[name] = blob
		case *Archive:
			// An archive must be recursively walked in order to turn it into a flat result map.
			subs, err := t.Read()
			if err != nil {
				return nil, errors.Wrapf(err, "failed to expand sub-archive '%v'", name)
			}
			for sub, blob := range subs {
				result[filepath.Join(name, sub)] = blob
			}
		}
	}
	return result, nil
}

func (a *Archive) readPath() (map[string]*Blob, error) {
	// To read a path-based archive, read that file and use its extension to ascertain what format to use.
	path, ispath := a.GetPath()
	contract.Assertf(ispath, "Expected a path-based asset")

	format := detectArchiveFormat(path)

	if format == NotArchive {
		// If not an archive, it could be a directory; if so, simply expand it out uncompressed as an archive.
		info, err := os.Stat(path)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't read archive path '%v'", path)
		} else if info.IsDir() {
			return nil, errors.Wrapf(err, "'%v' is neither a recognized archive type nor a directory", path)
		}
		results := make(map[string]*Blob)
		if walkerr := filepath.Walk(path, func(filePath string, f os.FileInfo, fileerr error) error {
			// If there was an error, exit.
			if fileerr != nil {
				return fileerr
			}
			// If this was a directory or a symlink, skip it.
			if f.IsDir() || f.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			// Finally, if this was a .pulumi directory, we will skip this by default.
			// TODO[pulumi/pulumi#122]: when we support .pulumiignore, this will be customizable.
			if !f.IsDir() && f.Name() == workspace.Dir {
				return filepath.SkipDir
			}

			// Otherwise, add this asset to the list of blobs, and keep going.
			file := &Asset{Path: filePath}
			blob, err := file.Read()
			if err != nil {
				return err
			}
			results[filePath] = blob
			return nil
		}); walkerr != nil {
			return nil, walkerr
		}
		return results, nil
	}

	// Otherwise, it's an archive file, and we will go ahead and open it up and read it.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return readArchive(file, format)
}

func (a *Archive) readURI() (map[string]*Blob, error) {
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
		resp, err := http.Get(url.String())
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

func (a *Archive) archiveTar(w io.Writer) error {
	// Read the archive.
	arch, err := a.Read()
	if err != nil {
		return err
	}
	defer (func() {
		// Ensure we close all files before exiting this function, no matter the outcome.
		for _, blob := range arch {
			contract.IgnoreClose(blob)
		}
	})()

	// Sort the file names so we emit in a deterministic order.
	var files []string
	for file := range arch {
		files = append(files, file)
	}
	sort.Strings(files)

	// Now actually emit the contents, file by file.
	tw := tar.NewWriter(w)
	for _, file := range files {
		data := arch[file]
		sz := data.Size()
		if err := tw.WriteHeader(&tar.Header{
			Name: file,
			Mode: 0600,
			Size: sz,
		}); err != nil {
			return err
		}
		n, err := io.Copy(tw, data)
		if err != nil {
			return err
		}
		contract.Assert(n == sz)
	}

	return tw.Close()
}

func (a *Archive) archiveTarGZIP(w io.Writer) error {
	z := gzip.NewWriter(w)
	return a.archiveTar(z)
}

func (a *Archive) archiveZIP(w io.Writer) error {
	// Read the archive.
	arch, err := a.Read()
	if err != nil {
		return err
	}
	defer (func() {
		// Ensure we close all files before exiting this function, no matter the outcome.
		for _, blob := range arch {
			contract.IgnoreClose(blob)
		}
	})()

	// Sort the file names so we emit in a deterministic order.
	var files []string
	for file := range arch {
		files = append(files, file)
	}
	sort.Strings(files)

	// Now actually emit the contents, file by file.
	zw := zip.NewWriter(w)
	for _, file := range files {
		fw, err := zw.Create(file)
		if err != nil {
			return err
		}
		if _, err = io.Copy(fw, arch[file]); err != nil {
			return err
		}
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

// EnsureHash computes the SHA1 hash of the archive's contents and stores it on the object.
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
	ext := filepath.Ext(path)
	if moreext := filepath.Ext(strings.TrimRight(path, ext)); moreext != "" {
		ext = moreext + ext // this ensures we detect ".tar.gz" correctly.
	}
	format, has := ArchiveExts[ext]
	if !has {
		return NotArchive
	}
	return format
}

// readArchive takes a stream to an existing archive and returns a map of names to readers for the inner assets.
// The routine returns an error if something goes wrong and, no matter what, closes the stream before returning.
func readArchive(ar io.ReadCloser, format ArchiveFormat) (map[string]*Blob, error) {
	defer contract.IgnoreClose(ar) // consume the input stream

	switch format {
	case TarArchive:
		return readTarArchive(ar)
	case TarGZIPArchive:
		return readTarGZIPArchive(ar)
	case ZIPArchive:
		// Unfortunately, the ZIP archive reader requires ReaderAt functionality.  If it's a file, we can recovera this
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

func readTarArchive(ar io.ReadCloser) (map[string]*Blob, error) {
	defer contract.IgnoreClose(ar) // consume the input stream

	// Create a tar reader and walk through each file, adding each one to the map.
	assets := make(map[string]*Blob)
	tr := tar.NewReader(ar)
	for {
		file, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		switch file.Typeflag {
		case tar.TypeDir:
			continue // skip directories
		case tar.TypeReg:
			data := make([]byte, file.Size)
			n, err := tr.Read(data)
			if err != nil {
				return nil, err
			}
			contract.Assert(int64(n) == file.Size)
			assets[file.Name] = NewByteBlob(data)
		default:
			contract.Failf("Unrecognized tar header typeflag: %v", file.Typeflag)
		}
	}

	return assets, nil
}

func readTarGZIPArchive(ar io.ReadCloser) (map[string]*Blob, error) {
	defer contract.IgnoreClose(ar) // consume the input stream

	// First decompress the GZIP stream.
	gz, err := gzip.NewReader(ar)
	if err != nil {
		return nil, err
	}

	// Now read the tarfile.
	return readTarArchive(gz)
}

func readZIPArchive(ar io.ReaderAt, size int64) (map[string]*Blob, error) {
	// Create a ZIP reader and iterate over the files inside of it, adding each one.
	assets := make(map[string]*Blob)
	z, err := zip.NewReader(ar, size)
	if err != nil {
		return nil, err
	}
	for _, file := range z.File {
		body, err := file.Open()
		if err != nil {
			return nil, err
		}
		size := file.UncompressedSize64
		data := make([]byte, size)
		n, err := body.Read(data)
		if err != nil {
			return nil, err
		}
		contract.Assert(uint64(n) == size)
		assets[file.Name] = NewByteBlob(data)
	}
	return assets, nil
}
