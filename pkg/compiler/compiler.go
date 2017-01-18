// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"os"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/options"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/workspace"
)

// Compiler provides an interface into the many phases of the Mu compilation process.
type Compiler interface {
	core.Phase

	Options() *options.Options // the options this compiler is using.
	Workspace() workspace.W    // the workspace that this compielr is using.

	// Compile takes a MuPackage as input and compiles it into a MuGL graph.
	Compile(pkg *pack.Package) graph.Graph
}

// compiler is the canonical implementation of the Mu compiler.
type compiler struct {
	w    workspace.W
	deps map[symbols.Ref]*diag.Document // a cache of mapping names to loaded dependencies.
}

// New creates a new instance of the Mu compiler with the given workspace and options.
func New(w workspace.W) Compiler {
	contract.Requiref(w != nil, "w", "!= nil")
	return &compiler{
		w:    w,
		deps: make(map[symbols.Ref]*diag.Document),
	}
}

// NewDefault creates a new instance of the Mu compiler, along with a new workspace, from the given path.  If options
// is nil, the default compiler options will be used instead.  If any IO errors occur, they will be output in the usual
// diagnostics ways, and the compiler return value will be nil while the error will be non-nil.
func NewDefault(path string, opts *options.Options) (Compiler, error) {
	if opts == nil {
		opts = options.Default(path)
	} else {
		opts.Pwd = path
	}

	w, err := workspace.New(opts)
	if err != nil {
		opts.Diag.Errorf(errors.ErrorIO.AtFile(path), err)
		return nil, fmt.Errorf("cannot proceed without a workspace")
	}

	return New(w), nil
}

// NewDefaultwd creates a new instance of the Mu compiler, along with a new workspace, from the current working
// directory.  If options is nil, the default compiler options will be used instead.  If any IO errors occur, they will
// be output in the usual diagnostics ways, and the compiler return value will be nil while the error will be non-nil.
func NewDefaultwd(opts *options.Options) (Compiler, error) {
	pwd, err := os.Getwd()
	contract.Assertf(err == nil, "Unexpected os.Getwd error: %v", err)
	return NewDefault(pwd, opts)
}

func (c *compiler) Diag() diag.Sink           { return c.Options().Diag }
func (c *compiler) Options() *options.Options { return c.w.Options() }
func (c *compiler) Workspace() workspace.W    { return c.w }

func (c *compiler) Compile(pkg *pack.Package) graph.Graph {
	contract.Requiref(pkg != nil, "pkg", "!= nil")
	return c.compilePackage(pkg)
}

func (c *compiler) compilePackage(pkg *pack.Package) graph.Graph {
	glog.Infof("Compiling package '%v' (w=%v)", pkg.Name, c.w.Root())
	if glog.V(2) {
		defer glog.V(2).Infof("Building package '%v' completed w/ %v warnings and %v errors",
			pkg.Name, c.Diag().Warnings(), c.Diag().Errors())
	}

	// To compile a package, we require a decoded MuPackage object; this has already been done, and is presented to us
	// an argument.  Next, we must bind it's contents.  To bind its contents, we must:
	//
	//     * Resolve all dependency packages and inject them into a package map (just a map of names to symbols).
	//     * Bind each dependency package, in order, by recursing into the present algorithm.
	//     * Enumerate all modules, and for each:
	//         + Inject a module symbol into an export map associated with the package symbol.
	//         + Enumerate all module members, and for each:
	//             - Inject a symbol of the appropriate kind into the module's associated member map.
	//             - Enumerate any class symbols resulting from this process, and for each:
	//                 . Inject a symbol of the appropriate kind into the class's associated member map.
	//
	// Essentially, all we are doing is mapping names to concrete symbols.  This ensures that as we compile a package,
	// we are able to find all tokens in these maps.  If we ever cannot find a token in a map, it means the MuPackage
	// file is invalid.  We require all MetaMu compilers to produce valid, verifiable MuIL, and this is a requriement.
	//
	// Afterwards, we can safely evaluate the MuIL entrypoint, using our MuIL AST interpreter.

	return nil
}
