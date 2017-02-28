// Copyright 2016 Pulumi, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"os"

	"github.com/golang/glog"

	"github.com/pulumi/coconut/pkg/compiler/binder"
	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/compiler/errors"
	"github.com/pulumi/coconut/pkg/compiler/metadata"
	"github.com/pulumi/coconut/pkg/compiler/symbols"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/eval"
	"github.com/pulumi/coconut/pkg/eval/heapstate"
	"github.com/pulumi/coconut/pkg/pack"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/workspace"
)

// Compiler provides an interface into the many phases of the Coconut compilation process.
type Compiler interface {
	core.Phase

	Ctx() *core.Context     // the shared context object.
	Workspace() workspace.W // the workspace that this compielr is using.

	// Compile detects a package from the workspace and compiles it into a graph.
	Compile(preexec Preexec) (*symbols.Package, *heapstate.Heap)
	// CompilePath compiles a package given its path into its graph form.
	CompilePath(path string, preexec Preexec) (*symbols.Package, *heapstate.Heap)
	// CompilePackage compiles a given package object into its associated graph form.
	CompilePackage(pkg *pack.Package, preexec Preexec) (*symbols.Package, *heapstate.Heap)
	// Verify detects a package from the workspace and validates its CocoIL contents.
	Verify() bool
	// VerifyPath verifies a package given its path, validating that its CocoIL contents are correct.
	VerifyPath(path string) bool
	// VerifyPackage verifies a given package object, validating that its CocoIL contents are correct.
	VerifyPackage(pkg *pack.Package) bool
}

// Preexec can be used to hook compilation after binding, but before evaluation, for any pre-evaluation steps.
type Preexec func(*binder.Context, *symbols.Package, eval.Interpreter)

// compiler is the canonical implementation of the Coconut compiler.
type compiler struct {
	w      workspace.W
	ctx    *core.Context
	reader metadata.Reader
}

// New creates a new instance of the Coconut compiler, along with a new workspace, from the given path.  If options
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

// Newwd creates a new instance of the Coconut compiler, along with a new workspace, from the current working directory.
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
func (c *compiler) Compile(preexec Preexec) (*symbols.Package, *heapstate.Heap) {
	if path := c.detectPackage(); path != "" {
		return c.CompilePath(path, preexec)
	}
	return nil, nil
}

// CompilePath loads a package at the given path and compiles it into a graph.
func (c *compiler) CompilePath(path string, preexec Preexec) (*symbols.Package, *heapstate.Heap) {
	if pkg := c.readPackage(path); pkg != nil {
		return c.CompilePackage(pkg, preexec)
	}
	return nil, nil
}

// CompilePackage compiles the given package into a graph.
func (c *compiler) CompilePackage(pkg *pack.Package, preexec Preexec) (*symbols.Package, *heapstate.Heap) {
	contract.Requiref(pkg != nil, "pkg", "!= nil")
	glog.Infof("Compiling package '%v' (w=%v)", pkg.Name, c.w.Root())
	if glog.V(2) {
		defer glog.V(2).Infof("Building package '%v' completed w/ %v warnings and %v errors",
			pkg.Name, c.Diag().Warnings(), c.Diag().Errors())
	}

	// To compile a package, we require a decoded NutPack object; this has already been done, and is presented to us
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
	// we are able to find all tokens in these maps.  If we ever cannot find a token in a map, it means the NutPack
	// file is invalid.  We require that CocoLang compilers produce valid, verifiable NutIL, and this is a requriement.
	//
	// Afterwards, we can safely evaluate the CocoIL entrypoint, using our CocoIL AST interpreter.

	// First, bind.
	b := binder.New(c.w, c.ctx, c.reader)
	pkgsym := b.BindPackage(pkg)
	if !c.Diag().Success() {
		return nil, nil
	}

	// Now, create the machinery we need to generate a graph.
	gg := heapstate.New(c.ctx)

	// Create a fresh evaluator; if there are pre-exec hooks, run them now.
	e := eval.New(b.Ctx(), gg)
	if preexec != nil {
		preexec(b.Ctx(), pkgsym, e)
	}
	if !c.Diag().Success() {
		return nil, nil // the preexec functions raised errors; quit.
	}

	// Go ahead and perform the evaluation.
	e.EvaluatePackage(pkgsym, c.ctx.Opts.Args)
	if !c.Diag().Success() {
		return pkgsym, nil
	}

	// Finally ask the graph generator to return what it has seen in graph form.
	return pkgsym, gg.HeapSnapshot()
}

// Verify detects a package from the workspace and validates its CocoIL contents.
func (c *compiler) Verify() bool {
	if path := c.detectPackage(); path != "" {
		return c.VerifyPath(path)
	}
	return false
}

// VerifyPath verifies a package given its path, validating that its CocoIL contents are correct.
func (c *compiler) VerifyPath(path string) bool {
	if pkg := c.readPackage(path); pkg != nil {
		return c.VerifyPackage(pkg)
	}
	return false
}

// VerifyPackage verifies a given package object, validating that its CocoIL contents are correct.
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
		c.Diag().Errorf(errors.ErrorMissingNutfile, c.ctx.Path)
		return ""
	}
	return path
}

// readPackage loads a package from the given path, issuing errors if anything bad happens.
func (c *compiler) readPackage(path string) *pack.Package {
	doc, err := diag.ReadDocument(path)
	if err != nil {
		c.Diag().Errorf(errors.ErrorCouldNotReadNutfile.AtFile(path), err)
		return nil
	}
	return c.reader.ReadPackage(doc)
}
