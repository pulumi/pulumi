// Copyright 2016 Marapongo, Inc. All rights reserved.

package diag

import (
	"io/ioutil"
	"path/filepath"
)

// Document is a file used during compilation, for which advanced diagnostics, such as line/column numbers, may be
// required.  It stores the contents of the entire file so that precise errors can be given; Forget discards them.
type Document struct {
	File   string
	Body   []byte
	Parent *Document // if this document was generated from another, this will point to it.
}

var _ Diagable = (*Document)(nil) // compile-time assertion that *Document implements Diagable.

func NewDocument(file string) *Document {
	return &Document{File: file}
}

func ReadDocument(file string) (*Document, error) {
	body, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return &Document{File: file, Body: body}, nil
}

func (doc *Document) Ext() string {
	return filepath.Ext(doc.File)
}

func (doc *Document) Forget() {
	doc.Body = nil
}

func (doc *Document) Where() (*Document, *Location) {
	return doc, nil
}
