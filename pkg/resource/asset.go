// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Asset is a serialized asset reference.  It is a union: thus, only one of its fields will be non-nil.  Several helper
// routines exist as members in order to easily interact with the assets referenced by an instance of this type.
//nolint: lll
type Asset struct {
	Sig  string `json:"4dabf18193072939515e22adb298388d" yaml:"4dabf18193072939515e22adb298388d"` // the unique asset signature (see properties.go).
	Text string `json:"text,omitempty" yaml:"text,omitempty"`                                     // a textual asset.
	Path string `json:"path,omitempty" yaml:"path,omitempty"`                                     // a file on the current filesystem.
	URI  string `json:"uri,omitempty" yaml:"uri,omitempty"`                                       // a URI (file://, http://, https://, or custom).
}

const (
	AssetSig          = "c44067f5952c0a294b673a41bacd8c17" // a randomly assigned type hash for assets.
	AssetTextProperty = "text"                             // the dynamic property for an asset's text.
	AssetPathProperty = "path"                             // the dynamic property for an asset's path.
	AssetURIProperty  = "uri"                              // the dynamic property for an asset's URI.
)

func NewTextAsset(text string) Asset { return Asset{Sig: AssetSig, Text: text} }
func NewPathAsset(path string) Asset { return Asset{Sig: AssetSig, Path: path} }
func NewURIAsset(uri string) Asset   { return Asset{Sig: AssetSig, URI: uri} }

func (a Asset) IsText() bool { return a.Text != "" }
func (a Asset) IsPath() bool { return a.Path != "" }
func (a Asset) IsURI() bool  { return a.URI != "" }

func (a Asset) GetText() (string, bool) {
	if a.IsText() {
		return a.Text, true
	}
	return "", false
}

func (a Asset) GetPath() (string, bool) {
	if a.IsPath() {
		return a.Path, true
	}
	return "", false
}

func (a Asset) GetURI() (string, bool) {
	if a.IsURI() {
		return a.URI, true
	}
	return "", false
}

// GetURIURL returns the underlying URI as a parsed URL, provided it is one.  If there was an error parsing the URI, it
// will be returned as a non-nil error object.
func (a Asset) GetURIURL() (*url.URL, bool, error) {
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
func (a Asset) Equals(other Asset) bool {
	return a.Text == other.Text && a.Path == other.Path && a.URI == other.URI
}

// Serialize returns a weakly typed map that contains the right signature for serialization purposes.
func (a Asset) Serialize() map[string]interface{} {
	return map[string]interface{}{
		string(SigKey):    AssetSig,
		AssetTextProperty: a.Text,
		AssetPathProperty: a.Path,
		AssetURIProperty:  a.URI,
	}
}

// DeserializeAsset checks to see if the map contains an asset, using its signature, and if so deserializes it.
func DeserializeAsset(obj map[string]interface{}) (Asset, bool) {
	if obj[string(SigKey)] != AssetSig {
		return Asset{}, false
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
	return Asset{
		Text: text,
		Path: path,
		URI:  uri,
	}, true
}

// Read reads an asset's contents into memory.
func (a Asset) Read() (*Blob, error) {
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

func (a Asset) readText() (*Blob, error) {
	text, istext := a.GetText()
	contract.Assertf(istext, "Expected a text-based asset")
	return NewByteBlob([]byte(text)), nil
}

func (a Asset) readPath() (*Blob, error) {
	path, ispath := a.GetPath()
	contract.Assertf(ispath, "Expected a path-based asset")
	byts, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewByteBlob(byts), nil
}

func (a Asset) readURI() (*Blob, error) {
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
		contract.Assert(url.Host == "")
		contract.Assert(url.User == nil)
		contract.Assert(url.RawQuery == "")
		contract.Assert(url.Fragment == "")
		f, err := os.Open(url.Path)
		if err != nil {
			return nil, err
		}
		return NewFileBlob(f)
	default:
		return nil, errors.Errorf("Unrecognized or unsupported URI scheme: %v", s)
	}
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
	Sig    string           `json:"4dabf18193072939515e22adb298388d" yaml:"4dabf18193072939515e22adb298388d"` // the unique asset signature (see properties.go).
	Assets map[string]Asset `json:"assets,omitempty" yaml:"assets,omitempty"`                                 // a collection of other assets.
	Path   string           `json:"path,omitempty" yaml:"path,omitempty"`                                     // a file on the current filesystem.
	URI    string           `json:"uri,omitempty" yaml:"uri,omitempty"`                                       // a remote URI (file://, http://, https://, etc).
}

const (
	ArchiveSig            = "0def7320c3a5731c473e5ecbe6d01bc7" // a randomly assigned archive type signature.
	ArchiveAssetsProperty = "assets"                           // the dynamic property for an archive's assets.
	ArchivePathProperty   = "path"                             // the dynamic property for an archive's path.
	ArchiveURIProperty    = "uri"                              // the dynamic property for an archive's URI.
)

func NewAssetArchive(assets map[string]Asset) Archive { return Archive{Sig: ArchiveSig, Assets: assets} }
func NewPathArchive(path string) Archive              { return Archive{Sig: ArchiveSig, Path: path} }
func NewURIArchive(uri string) Archive                { return Archive{Sig: ArchiveSig, URI: uri} }

func (a Archive) IsAssets() bool { return a.Assets != nil }
func (a Archive) IsPath() bool   { return a.Path != "" }
func (a Archive) IsURI() bool    { return a.URI != "" }

func (a Archive) GetAssets() (map[string]Asset, bool) {
	if a.IsAssets() {
		return a.Assets, true
	}
	return nil, false
}

func (a Archive) GetPath() (string, bool) {
	if a.IsPath() {
		return a.Path, true
	}
	return "", false
}

func (a Archive) GetURI() (string, bool) {
	if a.IsURI() {
		return a.URI, true
	}
	return "", false
}

// GetURIURL returns the underlying URI as a parsed URL, provided it is one.  If there was an error parsing the URI, it
// will be returned as a non-nil error object.
func (a Archive) GetURIURL() (*url.URL, bool, error) {
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
func (a Archive) Equals(other Archive) bool {
	if a.Assets != nil {
		if other.Assets == nil {
			return false
		}
		if len(a.Assets) != len(other.Assets) {
			return false
		}
		for key, value := range a.Assets {
			if other.Assets[key] != value {
				return false
			}
		}
	} else if other.Assets != nil {
		return false
	}
	return a.Path == other.Path && a.URI == other.URI
}

// Serialize returns a weakly typed map that contains the right signature for serialization purposes.
func (a Archive) Serialize() map[string]interface{} {
	var assets map[string]interface{}
	if a.Assets != nil {
		assets = make(map[string]interface{})
		for k, v := range a.Assets {
			assets[k] = v.Serialize()
		}
	}
	return map[string]interface{}{
		string(SigKey):        ArchiveSig,
		ArchiveAssetsProperty: assets,
		ArchivePathProperty:   a.Path,
		ArchiveURIProperty:    a.URI,
	}
}

// DeserializeArchive checks to see if the map contains an archive, using its signature, and if so deserializes it.
func DeserializeArchive(obj map[string]interface{}) (Archive, bool) {
	if obj[string(SigKey)] != ArchiveSig {
		return Archive{}, false
	}
	var assets map[string]Asset
	if v, has := obj[ArchiveAssetsProperty]; has {
		assets = make(map[string]Asset)
		for k, v := range v.(map[string]interface{}) {
			switch t := v.(type) {
			case Asset:
				assets[k] = t
			case map[string]interface{}:
				a, isa := DeserializeAsset(t)
				if !isa {
					return Archive{}, false
				}
				assets[k] = a
			default:
				return Archive{}, false
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
	return Archive{
		Assets: assets,
		Path:   path,
		URI:    uri,
	}, true
}

// Read returns a map of asset name to its associated reader object (which can be used to perform reads/IO).
func (a Archive) Read() (map[string]*Blob, error) {
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

func (a Archive) readAssets() (map[string]*Blob, error) {
	// To read a map-based archive, just produce a map from each asset to its associated reader.
	m, isassets := a.GetAssets()
	contract.Assertf(isassets, "Expected an asset map-based archive")
	result := map[string]*Blob{}
	for name, asset := range m {
		// TODO[pulumi/pulumi#240]: It would be better to treat folders as a first class concept intead
		//  of reusing a path Asset for this purpose.
		path, isPath := asset.GetPath()
		if isPath {
			if fi, err := os.Stat(path); err == nil && fi.IsDir() {
				// Asset is a folder, expand it
				if walkerr := filepath.Walk(path, func(filePath string, f os.FileInfo, fileerr error) error {
					if fileerr != nil || f.IsDir() || f.Mode()&os.ModeSymlink != 0 {
						return fileerr
					}

					var err error
					result[filePath], err = NewPathAsset(filePath).Read()
					return err
				}); walkerr != nil {
					return nil, walkerr
				}
				continue
			}
		}
		var err error
		if result[name], err = asset.Read(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a Archive) readPath() (map[string]*Blob, error) {
	// To read a path-based archive, read that file and use its extension to ascertain what format to use.
	path, ispath := a.GetPath()
	contract.Assertf(ispath, "Expected a path-based asset")

	format, err := detectArchiveFormat(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return readArchive(file, format)
}

func (a Archive) readURI() (map[string]*Blob, error) {
	// To read a URI-based archive, fetch the contents remotely and use the extension to pick the format to use.
	url, isurl, err := a.GetURIURL()
	if err != nil {
		return nil, err
	}
	contract.Assertf(isurl, "Expected a URI-based asset")

	format, err := detectArchiveFormat(url.Path)
	if err != nil {
		// IDEA: support (a) hints and (b) custom providers that default to certain formats.
		return nil, err
	}

	ar, err := a.openURLStream(url)
	if err != nil {
		return nil, err
	}
	return readArchive(ar, format)
}

func (a Archive) openURLStream(url *url.URL) (io.ReadCloser, error) {
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
func (a Archive) Bytes(format ArchiveFormat) ([]byte, error) {
	var data bytes.Buffer
	if err := a.Archive(format, &data); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// Archive produces a single archive stream in the desired format.  It prefers to return the archive with as little
// copying as is feasible, however if the desired format is different from the source, it will need to translate.
func (a Archive) Archive(format ArchiveFormat, w io.Writer) error {
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

func (a Archive) archiveTar(w io.Writer) error {
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

func (a Archive) archiveTarGZIP(w io.Writer) error {
	z := gzip.NewWriter(w)
	return a.archiveTar(z)
}

func (a Archive) archiveZIP(w io.Writer) error {
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

// ReadSourceArchive returns a stream to the underlying archive, if there eis one.
func (a Archive) ReadSourceArchive() (ArchiveFormat, io.ReadCloser, error) {
	if path, ispath := a.GetPath(); ispath {
		if format, archerr := detectArchiveFormat(path); archerr != nil && format != NotArchive {
			f, err := os.Open(path)
			return format, f, err
		}
	} else if url, isurl, urlerr := a.GetURIURL(); urlerr == nil && isurl {
		if format, archerr := detectArchiveFormat(url.Path); archerr == nil && format != NotArchive {
			s, err := a.openURLStream(url)
			return format, s, err
		}
	}
	return NotArchive, nil, nil
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
func detectArchiveFormat(path string) (ArchiveFormat, error) {
	ext := filepath.Ext(path)
	if moreext := filepath.Ext(strings.TrimRight(path, ext)); moreext != "" {
		ext = moreext + ext // this ensures we detect ".tar.gz" correctly.
	}
	format, has := ArchiveExts[ext]
	if !has {
		return NotArchive, errors.Errorf("unrecognized archive format '%v'", ext)
	}
	return format, nil
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
