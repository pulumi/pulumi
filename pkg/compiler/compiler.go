// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package compiler

import (
	"os"

	"github.com/golang/glog"
	goerr "github.com/pkg/errors"

	"github.com/pulumi/lumi/pkg/compiler/binder"
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/metadata"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/workspace"
)

// Compiler provides an interface into the many phases of the Lumi compilation process.
type Compiler interface {
	core.Phase

	Ctx() *core.Context     // the shared context object.
	Workspace() workspace.W // the workspace that this compielr is using.

	// Compile detects a package from the workspace and compiles it.
	Compile() (binder.Binder, *symbols.Package)
	// CompilePath compiles a package in the given path.
	CompilePath(path string) (binder.Binder, *symbols.Package)
	// CompilePackage compiles a given pre-loaded package object.
	CompilePackage(pkg *pack.Package) (binder.Binder, *symbols.Package)
	// Verify detects a package from the workspace and validates its LumiIL contents.
	Verify() bool
	// VerifyPath verifies a package given its path, validating that its LumiIL contents are correct.
	VerifyPath(path string) bool
	// VerifyPackage verifies a given package object, validating that its LumiIL contents are correct.
	VerifyPackage(pkg *pack.Package) bool
}

// compiler is the canonical implementation of the Lumi compiler.
type compiler struct {
	w      workspace.W
	ctx    *core.Context
	reader metadata.Reader
}

// New creates a new instance of the Lumi compiler, along with a new workspace, from the given path.  If options
// is nil, the default compiler options will be used instead.  If any IO errors occur, they will be output in the usual
// diagnostics ways, and the compiler return value will be nil while the error will be non-nil.
func New(path string, opts *core.Options) (Compiler, error) {
	// Ensure the options and diagnostics sink have been initialized.
	if opts == nil {
		opts = core.DefaultOptions()
	}
	d := opts.Diag
	if d == nil {
		d = core.DefaultSink(path)
	}

	// Now create a new context to share amongst the compiler and workspace.
	ctx := core.NewContext(path, d, opts)

	// Create a metadata reader for workspaces and packages (both the root one and dependencies).
	reader := metadata.NewReader(ctx)

	// Allocate the workspace object.
	w, err := workspace.New(ctx)
	if err != nil {
		d.Errorf(errors.ErrorIO.AtFile(path), err)
		return nil, goerr.Errorf("cannot proceed without a workspace")
	}

	// And finally return the freshly allocated compiler object.
	return &compiler{
		w:      w,
		ctx:    ctx,
		reader: reader,
	}, nil
}

// Newwd creates a new instance of the Lumi compiler, along with a new workspace, from the current working directory.
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
func (c *compiler) Compile() (binder.Binder, *symbols.Package) {
	if path := c.detectPackage(); path != "" {
		return c.CompilePath(path)
	}
	return nil, nil
}

// CompilePath loads a package at the given path and compiles it into a graph.
func (c *compiler) CompilePath(path string) (binder.Binder, *symbols.Package) {
	if pkg := c.readPackage(path); pkg != nil {
		return c.CompilePackage(pkg)
	}
	return nil, nil
}

// CompilePackage compiles the given package into a graph.
func (c *compiler) CompilePackage(pkg *pack.Package) (binder.Binder, *symbols.Package) {
	contract.Requiref(pkg != nil, "pkg", "!= nil")
	glog.Infof("Compiling package '%v' (w=%v)", pkg.Name, c.w.Root())
	if glog.V(2) {
		defer glog.V(2).Infof("Building package '%v' completed w/ %v warnings and %v errors",
			pkg.Name, c.Diag().Warnings(), c.Diag().Errors())
	}

	// To compile a package, we require a decoded LumiPack object; this has already been done, and is presented to us
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
	// we are able to find all tokens in these maps.  If we ever cannot find a token in a map, it means the LumiPack
	// file is invalid.  We require that LumiLang compilers produce valid, verifiable LumiIL, and this is a requriement.

	b := binder.New(c.w, c.ctx, c.reader)
	return b, b.BindPackage(pkg)
}

// Verify detects a package from the workspace and validates its LumiIL contents.
func (c *compiler) Verify() bool {
	if path := c.detectPackage(); path != "" {
		return c.VerifyPath(path)
	}
	return false
}

// VerifyPath verifies a package given its path, validating that its LumiIL contents are correct.
func (c *compiler) VerifyPath(path string) bool {
	if pkg := c.readPackage(path); pkg != nil {
		return c.VerifyPackage(pkg)
	}
	return false
}

// VerifyPackage verifies a given package object, validating that its LumiIL contents are correct.
func (c *compiler) VerifyPackage(pkg *pack.Package) bool {
	// To verify a package, simply run the binder aspects of it.
	b := binder.New(c.w, c.ctx, c.reader)
	b.BindPackage(pkg)
	return c.Diag().Success()
}

// detectPackage detects a package in the current workspace.
func (c *compiler) detectPackage() string {
	path, err := c.w.DetectPackage()
	if err != nil {
		c.Diag().Errorf(errors.ErrorIO, err)
		return ""
	}
	if path == "" {
		c.Diag().Errorf(errors.ErrorMissingProject, c.ctx.Path)
		return ""
	}
	return path
}

// readPackage loads a package from the given path, issuing errors if anything bad happens.
func (c *compiler) readPackage(path string) *pack.Package {
	doc, err := diag.ReadDocument(path)
	if err != nil {
		c.Diag().Errorf(errors.ErrorCouldNotReadProject.AtFile(path), err)
		return nil
	}
	return c.reader.ReadPackage(doc)
}
