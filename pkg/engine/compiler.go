package engine

import (
	"github.com/pulumi/pulumi-fabric/pkg/compiler"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/binder"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/core"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/errors"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/symbols"
	"github.com/pulumi/pulumi-fabric/pkg/pack"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// TODO[pulumi/pulumi-fabric#88]: enable arguments to flow to the package itself.  In that case, we want to split the
//     arguments at the --, if any, so we can still pass arguments to the compiler itself in these cases.
func prepareCompiler(pkgarg string) (compiler.Compiler, *pack.Package) {
	// Create a compiler options object and map any flags and arguments to settings on it.
	opts := core.DefaultOptions()

	// If a package argument is present, try to load that package (either via stdin or a path).
	var pkg *pack.Package
	var root string
	if pkgarg != "" {
		pkg, root = readPackageFromArg(pkgarg)
	}

	// Now create a compiler object based on whether we loaded a package or just have a root to deal with.
	var comp compiler.Compiler
	var err error
	if root == "" {
		comp, err = compiler.Newwd(opts)
	} else {
		comp, err = compiler.New(root, opts)
	}
	if err != nil {
		cmdutil.Diag().Errorf(errors.ErrorCantCreateCompiler, err)
	}

	return comp, pkg
}

// compile just uses the standard logic to parse arguments, options, and to locate/compile a package.  It returns the
// compilation result, or nil if an error occurred (in which case, we would expect diagnostics to have been output).
func compile(pkgarg string) *compileResult {
	// Prepare the compiler info and, provided it succeeds, perform the compilation.
	if comp, pkg := prepareCompiler(pkgarg); comp != nil {
		var b binder.Binder
		var pkgsym *symbols.Package
		if pkg == nil {
			b, pkgsym = comp.Compile()
		} else {
			b, pkgsym = comp.CompilePackage(pkg)
		}
		contract.Assert(b != nil)
		contract.Assert(pkgsym != nil)
		return &compileResult{
			C:   comp,
			B:   b,
			Pkg: pkgsym,
		}
	}

	return nil
}

type compileResult struct {
	C   compiler.Compiler
	B   binder.Binder
	Pkg *symbols.Package
}
