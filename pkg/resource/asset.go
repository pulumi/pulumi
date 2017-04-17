// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

// Asset is a serialized asset reference.  This is a union: thus, only one of its fields will be non-nil.
type Asset struct {
	Text *string `json:"text,omitempty"` // a textual asset.
	Path *string `json:"text,omitempty"` // a file on the current filesystem.
	URI  *string `json:"uri,omitempty"`  // a URI to a reference fetched (file://, http://, https://, or custom).
}

func (a Asset) IsText() bool { return a.Text != nil }
func (a Asset) IsPath() bool { return a.Path != nil }
func (a Asset) IsURI() bool  { return a.URI != nil }

func (a Asset) AsText() (string, bool) {
	if a.IsText() {
		return *a.Text, true
	}
	return "", false
}

func (a Asset) AsPath() (string, bool) {
	if a.IsPath() {
		return *a.Path, true
	}
	return "", false
}

func (a Asset) AsURI() (string, bool) {
	if a.IsURI() {
		return *a.URI, true
	}
	return "", false
}
