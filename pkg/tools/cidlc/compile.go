// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"golang.org/x/tools/go/loader"
)

type CompileOptions struct {
	Name        string // the package name.
	Root        string // the root package.
	OutPack     string // the package output location.
	OutProvider string // the provider output location.
}

// Compile runs the Go compiler against an IDL project and then generates code for the resulting program.
func Compile(opts CompileOptions, paths ...string) error {
	// First point the Go compiler at the target paths and compile them.  Note that this runs both parsing and semantic
	// analysis, and will yield an error if anything with the Go program is wrong.
	var conf loader.Config
	if _, err := conf.FromArgs(paths, false); err != nil {
		return err
	}
	prog, err := conf.Load()
	if err != nil {
		return err
	}

	// Now create in-memory IDL packages, validating contents as we go.  The result contains classified elements
	// such as resources, structs, enum types, and anything required in order to perform subsequent code-generation.
	chk := NewChecker(opts.Root, prog)
	var packgen *PackGenerator
	if out := opts.OutPack; out != "" {
		packgen = NewPackGenerator(opts.Root, out)
	}
	var rpcgen *ProviderGenerator
	if out := opts.OutProvider; out != "" {
		rpcgen = NewProviderGenerator(opts.Root, out)
	}
	for _, pkginfo := range prog.Created {
		pkg, err := chk.Check(opts.Name, pkginfo)
		if err != nil {
			return err
		}

		// Now generate the package output.
		if packgen != nil {
			if err = packgen.Generate(pkg); err != nil {
				return err
			}
		}

		// Next generate the RPC stubs output.
		if rpcgen != nil {
			if err = rpcgen.Generate(pkg); err != nil {
				return err
			}
		}
	}

	return nil
}
