// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/compiler"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/util/cmdutil"
	"github.com/marapongo/mu/pkg/util/contract"
)

// compile just uses the standard logic to parse arguments, options, and to locate/compile a package.  It returns the
// MuGL graph that is produced, or nil if an error occurred (in which case, we would expect non-0 errors).
func compile(cmd *cobra.Command, args []string) graph.Graph {
	// If there's a --, we need to separate out the command args from the stack args.
	flags := cmd.Flags()
	dashdash := flags.ArgsLenAtDash()
	var packArgs []string
	if dashdash != -1 {
		packArgs = args[dashdash:]
		args = args[0:dashdash]
	}

	// Create a compiler options object and map any flags and arguments to settings on it.
	opts := core.DefaultOptions()
	opts.Args = dashdashArgsToMap(packArgs)

	// In the case of an argument, load that specific package and new up a compiler based on its base path.
	// Otherwise, use the default workspace and package logic (which consults the current working directory).
	var mugl graph.Graph
	if len(args) == 0 {
		comp, err := compiler.Newwd(opts)
		if err != nil {
			contract.Failf("fatal: %v", err)
		}
		mugl = comp.Compile()
	} else {
		fn := args[0]
		if pkg := cmdutil.ReadPackageFromArg(fn); pkg != nil {
			var comp compiler.Compiler
			var err error
			if fn == "-" {
				comp, err = compiler.Newwd(opts)
			} else {
				comp, err = compiler.New(filepath.Dir(fn), opts)
			}
			if err != nil {
				contract.Failf("fatal: %v", err)
			}
			mugl = comp.CompilePackage(pkg)
		}
	}
	return mugl
}
