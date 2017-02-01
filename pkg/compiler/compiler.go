// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"os"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/binder"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/eval"
	"github.com/marapongo/mu/pkg/compiler/metadata"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/graph/graphgen"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/workspace"
)

// Compiler provides an interface into the many phases of the Mu compilation process.
type Compiler interface {
	core.Phase

	Ctx() *core.Context     // the shared context object.
	Workspace() workspace.W // the workspace that this compielr is using.

	// Compile detects a package from the workspace and compiles it into a graph.
	Compile() graph.Graph
	// CompilePath compiles a package given its path into its graph form.
	CompilePath(path string) graph.Graph
	// CompilePackage compiles a given package object into its associated graph form.
	CompilePackage(pkg *pack.Package) graph.Graph
}

// compiler is the canonical implementation of the Mu compiler.
type compiler struct {
	w      workspace.W
	ctx    *core.Context
	reader metadata.Reader
}

// New creates a new instance of the Mu compiler, along with a new workspace, from the given path.  If options
// is nil, the default compiler options will be used instead.  If any IO errors occur, they will be output in the usual
// diagnostics ways, and the compiler return value will be nil while the error will be non-nil.
func New(path string, opts *core.Options) (Compiler, error) {
	// Ensure the options and diagnostics sink have been initialized.
	if opts == nil {
		opts = core.DefaultOptions()
	}
	d := opts.Diag
	if d == nil {
		d = diag.DefaultSink(path)
	}

	// Now create a new context to share amongst the compiler and workspace.
	ctx := core.NewContext(path, d, opts)

	// Create a metadata reader for workspaces and packages (both the root one and dependencies).
	reader := metadata.NewReader(ctx)

	// Allocate the workspace object.
	w, err := workspace.New(ctx)
	if err != nil {
		d.Errorf(errors.ErrorIO.AtFile(path), err)
		return nil, fmt.Errorf("cannot proceed without a workspace")
	}

	// If there's a workspace-wide settings file available, load it and parse it.
	wdoc, err := w.ReadSettings()
	if err != nil {
		d.Errorf(errors.ErrorIO, err)
	} else if wdoc != nil {
		*w.Settings() = *reader.ReadWorkspace(wdoc)
	}

	// And finally return the freshly allocated compiler object.
	return &compiler{
		w:      w,
		ctx:    ctx,
		reader: reader,
	}, nil
}

// Newwd creates a new instance of the Mu compiler, along with a new workspace, from the current working directory.
// If options is nil, the default compiler options will be used instead.  If any IO errors occur, they will be output in
// the usual diagnostics ways, and the compiler return value will be nil while the error will be non-nil.
func Newwd(opts *core.Options) (Compiler, error) {
	pwd, err := os.Getwd()
	contract.Assertf(err == nil, "Unexpected os.Getwd error: %v", err)
	return New(pwd, opts)
}

func (c *compiler) Ctx() *core.Context     { return c.ctx }
func (c *compiler) Diag() diag.Sink        { return c.ctx.Diag }
func (c *compiler) Workspace() workspace.W { return c.w }

// Compile attempts to detect the package from the current working directory and, provided that succeeds, compiles it.
func (c *compiler) Compile() graph.Graph {
	path, err := c.w.DetectPackage()
	if err != nil {
		c.Diag().Errorf(errors.ErrorIO, err)
		return nil
	} else if path == "" {
		c.Diag().Errorf(errors.ErrorMissingMufile, c.ctx.Path)
		return nil
	} else {
		return c.CompilePath(path)
	}
}

// CompilePath loads a package at the given path and compiles it into a graph.
func (c *compiler) CompilePath(path string) graph.Graph {
	doc, err := diag.ReadDocument(path)
	if err != nil {
		c.Diag().Errorf(errors.ErrorCouldNotReadMufile.AtFile(path), err)
		return nil
	}
	pkg := c.reader.ReadPackage(doc)
	if pkg == nil {
		return nil
	}
	return c.CompilePackage(pkg)
}

// CompilePackage compiles the given package into a graph.
func (c *compiler) CompilePackage(pkg *pack.Package) graph.Graph {
	contract.Requiref(pkg != nil, "pkg", "!= nil")
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

	// First, bind.
	b := binder.New(c.w, c.ctx, c.reader)
	pkgsym := b.BindPackage(pkg)
	if !c.Diag().Success() {
		return nil
	}

	// Now, create the machinery we need to generate a graph.
	gg := graphgen.New(c.ctx)

	// Now, evaluate.
	e := eval.New(b.Ctx(), gg)
	e.EvaluatePackage(pkgsym, c.ctx.Opts.Args)

	// Finally ask the graph generator to return what it has seen in graph form.
	return gg.Graph()
}
