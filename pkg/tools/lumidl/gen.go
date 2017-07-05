// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package lumidl

import (
	"go/types"
	"path/filepath"

	"github.com/pulumi/lumi/pkg/tools"
)

const lumidl = "the Lumi IDL Compiler (LUMIDL)" // used in generated files.

// mirrorDirLayout ensures a target output directory contains the same layout as the input package.
func mirrorDirLayout(pkg *Package, out string) error {
	for relpath := range pkg.Files {
		// Make the target file by concatening the output with the relative path, and ensure the directory exists.
		path := filepath.Join(out, relpath)
		if err := tools.EnsureFileDir(path); err != nil {
			return err
		}
	}
	return nil
}

func forEachField(t TypeMember, action func(*types.Var, PropertyOptions)) int {
	return forEachStructField(t.Struct(), t.PropertyOptions(), action)
}

func forEachStructField(s *types.Struct, opts []PropertyOptions, action func(*types.Var, PropertyOptions)) int {
	n := 0
	for i, j := 0, 0; i < s.NumFields(); i++ {
		fld := s.Field(i)
		if fld.Anonymous() {
			// For anonymous types, recurse.
			named := fld.Type().(*types.Named)
			embedded := named.Underlying().(*types.Struct)
			k := forEachStructField(embedded, opts[j:], action)
			j += k
			n += k
		} else {
			// For actual fields, invoke the action, and bump the counters.
			if action != nil {
				action(s.Field(i), opts[j])
			}
			j++
			n++
		}
	}
	return n
}
