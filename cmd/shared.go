// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/compiler"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/resource"
	"github.com/marapongo/mu/pkg/util/cmdutil"
)

// compile just uses the standard logic to parse arguments, options, and to locate/compile a package.  It returns the
// MuGL graph that is produced, or nil if an error occurred (in which case, we would expect non-0 errors).
func compile(cmd *cobra.Command, args []string) *compileResult {
	// If there's a --, we need to separate out the command args from the stack args.
	flags := cmd.Flags()
	dashdash := flags.ArgsLenAtDash()
	var packArgs []string
	if dashdash != -1 {
		packArgs = args[dashdash:]
		args = args[0:dashdash]
	}

	// A func to lazily allocate a sink to be used if we can't create a compiler.
	d := func() diag.Sink { return core.DefaultSink("") }

	// Create a compiler options object and map any flags and arguments to settings on it.
	opts := core.DefaultOptions()
	opts.Args = dashdashArgsToMap(packArgs)

	// In the case of an argument, load that specific package and new up a compiler based on its base path.
	// Otherwise, use the default workspace and package logic (which consults the current working directory).
	var comp compiler.Compiler
	var pkg *pack.Package
	var g graph.Graph
	if len(args) == 0 {
		var err error
		comp, err = compiler.Newwd(opts)
		if err != nil {
			// Create a temporary diagnostics sink so that we can issue an error and bail out.
			d().Errorf(errors.ErrorCantCreateCompiler, err)
			return nil
		}
		pkg, g = comp.Compile()
	} else {
		fn := args[0]
		if pkg = cmdutil.ReadPackageFromArg(fn); pkg != nil {
			var err error
			if fn == "-" {
				comp, err = compiler.Newwd(opts)
			} else {
				comp, err = compiler.New(filepath.Dir(fn), opts)
			}
			if err != nil {
				d().Errorf(errors.ErrorCantReadPackage, fn, err)
				return nil
			}
			g = comp.CompilePackage(pkg)
		}
	}
	return &compileResult{comp, pkg, g}
}

type compileResult struct {
	C   compiler.Compiler
	Pkg *pack.Package
	G   graph.Graph
}

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func plan(cmd *cobra.Command, args []string, delete bool) *planResult {
	// Perform the compilation and, if non-nil is returned, create a plan and print it.
	result := compile(cmd, args)
	if result != nil {
		// Create a new context for the plan operations.
		ctx := resource.NewContext()

		// Create a resource snapshot from the object graph.
		snap, err := resource.NewSnapshot(ctx, result.Pkg.Name, result.C.Ctx().Opts.Args, result.G)
		if err != nil {
			result.C.Diag().Errorf(errors.ErrorCantCreateSnapshot, err)
			return nil
		}

		var plan resource.Plan
		if delete {
			// Generate a plan for deleting the entire snapshot.
			plan = resource.NewDeletePlan(ctx, snap)
		} else {
			// Generate a plan for creating the resources from scratch.
			plan = resource.NewCreatePlan(ctx, snap)
		}

		return &planResult{*result, ctx, snap, plan}
	}

	return nil
}

type planResult struct {
	compileResult
	Ctx  *resource.Context
	Snap resource.Snapshot
	Plan resource.Plan
}
