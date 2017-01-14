// Copyright 2016 Marapongo, Inc. All rights reserved.

// Because of the complex structure of the MuPack and MuIL metadata formats, we cannot rely on the standard JSON
// marshaling and unmarshaling routines.  Instead, we will need to do it mostly "by hand".  This package does that.
package encoding

import (
	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/pack"
)

// Decode unmarshals the entire contents of the given byte array into a Package object.
func Decode(m encoding.Marshaler, b []byte) (*pack.Package, error) {
	// First convert the whole contents of the metadata into a map.  Although it would be more efficient to walk the
	// token stream, token by token, this allows us to reuse existing YAML packages in addition to JSON ones.
	var tree object
	if err := m.Unmarshal(b, &tree); err != nil {
		return nil, err
	}
	return decodePackage(tree)
}

func decodePackage(tree object) (*pack.Package, error) {
	var pack pack.Package

	// First use tag-directed decoding for the simple parts of the struct.
	if err := decode(tree, &pack); err != nil {
		return nil, err
	}

	return &pack, nil
}
