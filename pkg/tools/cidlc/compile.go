// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"go/parser"

	"golang.org/x/tools/go/loader"

	"github.com/pulumi/coconut/pkg/tokens"
)

type CompileOptions struct {
	Name    tokens.PackageName // the package name.
	Root    string             // the package root on the filesystem.
	PkgBase string             // the base package name.
	OutPack string             // the package output location.
	OutRPC  string             // the RPC output location.
}

// Compile runs the Go compiler against an IDL project and then generates code for the resulting program.
func Compile(opts CompileOptions, paths ...string) error {
	// First point the Go compiler at the target paths and compile them.  Note that this runs both parsing and semantic
	// analysis, and will yield an error if anything with the Go program is wrong.
	var conf loader.Config
	if _, err := conf.FromArgs(paths, false); err != nil {
		return err
	}
	conf.ParserMode |= parser.ParseComments // ensure doc comments are retained.
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
	var rpcgen *RPCGenerator
	if out := opts.OutRPC; out != "" {
		rpcgen = NewRPCGenerator(opts.Root, opts.PkgBase, out)
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
