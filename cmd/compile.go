// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/cmdutil"
	"github.com/marapongo/mu/pkg/compiler"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

func newCompileCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "compile [blueprint] [-- [args]]",
		Short: "Compile a MuPackage into its MuGL graph representation",
		Long: "Compile a MuPackage into its MuGL graph representation.\n" +
			"\n" +
			"A graph is a topologically sorted directed-acyclic-graph (DAG), representing a\n" +
			"collection of resources that may be used in a deployment operation like plan or apply.\n" +
			"This graph is produced by evaluating the contents of a Mu blueprint package, and\n" +
			"does not actually perform any updates to the target environment.\n" +
			"\n" +
			"By default, a blueprint package is loaded from the current directory.  Optionally,\n" +
			"a path to a blueprint elsewhere can be provided as the [blueprint] argument.",
		Run: func(cmd *cobra.Command, args []string) {
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
			if mugl == nil {
				return
			}

			// Finally, serialize that MuGL graph so that it's suitable for printing/serializing.
			// TODO: this.
		},
	}

	return cmd
}

// dashdashArgsToMap is a simple args parser that places incoming key/value pairs into a map.  These are then used
// during MuPackage compilation as inputs to the main entrypoint function.
// TODO: this is fairly rudimentary; we eventually want to support arrays, maps, and complex types.
func dashdashArgsToMap(args []string) core.Args {
	mapped := make(core.Args)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Eat - or -- at the start.
		if arg[0] == '-' {
			arg = arg[1:]
			if arg[0] == '-' {
				arg = arg[1:]
			}
		}

		// Now find a k=v, and split the k/v part.
		if eq := strings.IndexByte(arg, '='); eq != -1 {
			// For --k=v, simply store v underneath k's entry.
			mapped[tokens.Name(arg[:eq])] = arg[eq+1:]
		} else {
			if i+1 < len(args) && args[i+1][0] != '-' {
				// If the next arg doesn't start with '-' (i.e., another flag) use its value.
				mapped[tokens.Name(arg)] = args[i+1]
				i++
			} else if arg[0:3] == "no-" {
				// For --no-k style args, strip off the no- prefix and store false underneath k.
				mapped[tokens.Name(arg[3:])] = false
			} else {
				// For all other --k args, assume this is a boolean flag, and set the value of k to true.
				mapped[tokens.Name(arg)] = true
			}
		}
	}

	return mapped
}
