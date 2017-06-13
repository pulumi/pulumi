// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lumidl

import (
	"fmt"
	"go/parser"
	"go/types"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"

	"github.com/pulumi/lumi/pkg/tokens"
)

type CompileOptions struct {
	Name       tokens.PackageName // the package name.
	PkgBaseIDL string             // the base Go package URL for the IDL input.
	PkgBaseRPC string             // the base Go package URL for the RPC output.
	OutPack    string             // the package output location.
	OutRPC     string             // the RPC output location.
	Quiet      bool               // true to suppress innocuous output messages.
	Recursive  bool               // true to generate code for all sub-packages.
}

// Compile runs the Go compiler against an IDL project and then generates code for the resulting program.
func Compile(opts CompileOptions, path string) error {
	// Ensure we are generating *something*.
	if opts.OutPack == "" && opts.OutRPC == "" {
		return errors.New("neither --out-pack nor --out-rpc specified; no code to generate")
	}

	// Adjust settings to their defaults and adjust any paths to be absolute.
	if path == "" {
		if wd, err := os.Getwd(); err == nil {
			path = wd
		}
	} else {
		if absPath, err := filepath.Abs(path); err == nil {
			path = absPath
		}
	}
	if opts.PkgBaseIDL == "" {
		// The default IDL package base is just the GOPATH package path for the target IDL path.
		pkgpath, err := goPackagePath(path)
		if err != nil {
			return err
		}
		opts.PkgBaseIDL = pkgpath
	}
	if opts.OutPack != "" {
		if outpack, err := filepath.Abs(opts.OutPack); err == nil {
			opts.OutPack = outpack
		}
	}
	if opts.OutRPC != "" {
		if outrpc, err := filepath.Abs(opts.OutRPC); err == nil {
			opts.OutRPC = outrpc
		}

		// If there is no package base, pick a default based on GOPATH.
		if opts.PkgBaseRPC == "" {
			// The default RPC package base, like the IDL package base, defaults to the GOPATH package path.
			pkgpath, err := goPackagePath(opts.OutRPC)
			if err != nil {
				return err
			}
			opts.PkgBaseRPC = pkgpath
		}
	}

	var inputs []string
	if opts.Recursive {
		inp, err := gatherGoPackages(path)
		if err != nil {
			return err
		}
		inputs = inp
	} else {
		inputs = []string{opts.PkgBaseIDL}
	}

	// First point the Go compiler at the target packages to compile.  Note that this runs both parsing and semantic
	// analysis, and will yield an error if anything with the Go program is wrong.
	var conf loader.Config
	if _, err := conf.FromArgs(inputs, false); err != nil {
		return err
	}
	conf.ParserMode |= parser.ParseComments // ensure doc comments are retained.
	prog, err := conf.Load()
	if err != nil {
		return err
	}

	// Now create in-memory IDL packages, validating contents as we go.  The result contains classified elements
	// such as resources, structs, enum types, and anything required in order to perform subsequent code-generation.
	chk := NewChecker(path, prog)
	var packgen *PackGenerator
	if out := opts.OutPack; out != "" {
		packgen = NewPackGenerator(prog, path, opts.PkgBaseIDL, out)
	}
	var rpcgen *RPCGenerator
	if out := opts.OutRPC; out != "" {
		rpcgen = NewRPCGenerator(path, opts.PkgBaseIDL, opts.PkgBaseRPC, out)
	}

	// Enumerate all packages (in a deterministic order).
	var pkgs []*types.Package
	for pkg := range prog.AllPackages {
		pkgs = append(pkgs, pkg)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Path() < pkgs[j].Path()
	})
	for _, pkg := range pkgs {
		// Only emit packages that are underneath the base IDL package.
		if !strings.HasPrefix(pkg.Path(), opts.PkgBaseIDL) {
			continue
		}

		pkginfo := prog.AllPackages[pkg]
		if !opts.Quiet {
			fmt.Printf("Processing package %v\n", pkginfo.Pkg.Path())
		}

		outpkg, err := chk.Check(opts.Name, pkginfo)
		if err != nil {
			return err
		}

		// Now generate the package output.
		if packgen != nil {
			if err = packgen.Generate(outpkg); err != nil {
				return err
			}
		}

		// Next generate the RPC stubs output.
		if rpcgen != nil {
			if err = rpcgen.Generate(outpkg); err != nil {
				return err
			}
		}
	}

	return nil
}

// goPackagePath takes a path to a filesystem location and returns its Go package path, based on GOPATH.  Given a path
// referring to a source location of the form, `$GOPATH/src/...`, the function returns the `...` part.
func goPackagePath(path string) (string, error) {
	// Fetch the GOPATH; it must be set, else we bail out.
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		return "", errors.New("GOPATH is not set, so package paths cannot be inferred (see --pkg-base-x)")
	}
	gopath = filepath.Join(gopath, "src")

	// Now ensure that the package path is a proper subset within it.
	if !filepath.HasPrefix(path, gopath) {
		return "", errors.Errorf(
			"Package root '%v' is not underneath $GOPATH/src, so its package cannot be inferred", path)
	}

	// Finally, strip off the GOPATH/src prefix, and return the remainder.
	return path[len(gopath)+1:], nil
}

// gatherGoPackages recurses into a given path and fetches all of its inferred Go packages.  The algorithm considers
// any sub-directory containing a *.go file, recursively, to be a package.  It could, of course, be wrong.
func gatherGoPackages(path string) ([]string, error) {
	var pkgs []string

	// First, if this path contains Go files, append it.
	var dirs []string
	hasGoFiles := false
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			dirs = append(dirs, file.Name())
		} else if filepath.Ext(file.Name()) == ".go" {
			hasGoFiles = true
		}
	}
	if hasGoFiles {
		pkg, err := goPackagePath(path)
		if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, pkg)
	}

	// Next, enumerate all directories recursively, to find all Go sub-packages.
	for _, dir := range dirs {
		subpkgs, err := gatherGoPackages(filepath.Join(path, dir))
		if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, subpkgs...)
	}

	return pkgs, nil
}
