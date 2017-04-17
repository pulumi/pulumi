// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/pulumi/coconut/pkg/util/contract"
)

// Asset is a serialized asset reference.  It is a union: thus, only one of its fields will be non-nil.  Several helper
// routines exist as members in order to easily interact with the assets referenced by an instance of this type.
type Asset struct {
	Text *string `json:"text,omitempty"` // a textual asset.
	Path *string `json:"text,omitempty"` // a file on the current filesystem.
	URI  *string `json:"uri,omitempty"`  // a URI to a reference fetched (file://, http://, https://, or custom).
}

func (a Asset) IsText() bool { return a.Text != nil }
func (a Asset) IsPath() bool { return a.Path != nil }
func (a Asset) IsURI() bool  { return a.URI != nil }

func (a Asset) GetText() (string, bool) {
	if a.IsText() {
		return *a.Text, true
	}
	return "", false
}

func (a Asset) GetPath() (string, bool) {
	if a.IsPath() {
		return *a.Path, true
	}
	return "", false
}

func (a Asset) GetURI() (string, bool) {
	if a.IsURI() {
		return *a.URI, true
	}
	return "", false
}

func (a Asset) Read() (AssetReader, error) {
	if text, istext := a.GetText(); istext {
		return newBytesReader([]byte(text)), nil
	} else if path, ispath := a.GetPath(); ispath {
		return os.Open(path)
	} else if uri, isuri := a.GetURI(); isuri {
		url, err := url.Parse(uri)
		if err != nil {
			return nil, err
		}
		switch s := url.Scheme; s {
		case "http", "https":
			resp, err := http.Get(uri)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			return newBytesReader(b), nil
		case "file":
			contract.Assert(url.Host == "")
			contract.Assert(url.User == nil)
			contract.Assert(url.RawQuery == "")
			contract.Assert(url.Fragment == "")
			return os.Open(url.Path)
		default:
			return nil, fmt.Errorf("Unrecognized or unsupported URI scheme: %v", s)
		}

		return nil, nil
	}
	contract.Failf("Invalid asset; one of Text, Path, or URI must be non-nil")
	return nil, nil
}

// AssetReader reads an asset's contents, offering Read, Seek, and Close functionality.
type AssetReader interface {
	io.Reader
	io.Seeker
	io.Closer
}

// bytesReader turns a *bytes.Reader into an AssetReader by adding an empty Close method.
type bytesReader struct {
	*bytes.Reader
}

func newBytesReader(b []byte) AssetReader {
	return bytesReader{
		Reader: bytes.NewReader(b),
	}
}

func (b bytesReader) Close() error {
	return nil // intentionally blank
}
