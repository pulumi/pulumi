// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package encoding can unmarshal LumiPack and LumiIL metadata formats.  Because of their complex structure, we cannot
// rely on the standard JSON  marshaling and unmarshaling routines.  Instead, we will need to do it mostly "by hand".
package encoding

import (
	"github.com/pulumi/pulumi/pkg/pack"
)

// Decode unmarshals the entire contents of the given byte array into a Package object.
func Decode(m Marshaler, b []byte) (*pack.Package, error) {
	var pack pack.Package
	if err := m.Unmarshal(b, &pack); err != nil {
		return nil, err
	}
	return &pack, nil
}
