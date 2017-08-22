// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/compiler"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/binder"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/core"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/errors"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/symbols"
	"github.com/pulumi/pulumi-fabric/pkg/pack"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// NewLumiCmd creates a new Lumi Cmd instance.
func NewLumiCmd() *cobra.Command {
	var logFlow bool
	var logToStderr bool
	var verbose int
	cmd := &cobra.Command{
		Use:   "lumi",
		Short: "Lumi is a framework and toolset for reusable stacks of services",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmdutil.InitLogging(logToStderr, verbose, logFlow)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
		},
	}

	cmd.PersistentFlags().BoolVar(&logFlow, "logflow", false, "Flow log settings to child processes (like plugins)")
	cmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDeployCmd())
	cmd.AddCommand(newDestroyCmd())
	cmd.AddCommand(newEnvCmd())
	cmd.AddCommand(newPackCmd())
	cmd.AddCommand(newPlanCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func pkgargFromArgs(args []string) string {
	if len(args) > 0 {
		return args[0]
	}

	return ""
}

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
