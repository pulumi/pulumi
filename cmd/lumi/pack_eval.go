// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/core"
	"github.com/pulumi/pulumi-fabric/pkg/engine"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

func newPackEvalCmd() *cobra.Command {
	var configEnv string
	var dotOutput bool
	var cmd = &cobra.Command{
		Use:   "eval [package] [-- [args]]",
		Short: "Evaluate a package and print the resulting objects",
		Long: "Evaluate a package and print the resulting objects\n" +
			"\n" +
			"A graph is a topologically sorted directed-acyclic-graph (DAG), representing a\n" +
			"collection of resources that may be used in a deployment operation like plan or apply.\n" +
			"This graph is produced by evaluating the contents of a blueprint package, and does not\n" +
			"actually perform any updates to the target environment.\n" +
			"\n" +
			"By default, a blueprint package is loaded from the current directory.  Optionally,\n" +
			"a path to a package elsewhere can be provided as the [package] argument.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			contract.Assertf(!dotOutput, "TODO[pulumi/pulumi-fabric#235]: DOT files not yet supported")

			pkgArg := pkgargFromArgs(args)
			evalArgs := make(core.Args)

			if len(args) > 1 {
				evalArgs = dashdashArgsToMap(args[1:])
			}

			return engine.PackEval(configEnv, pkgArg, evalArgs)
		}),
	}

	cmd.PersistentFlags().StringVar(
		&configEnv, "config-env", "",
		"Apply configuration from the specified environment before evaluating the package")
	cmd.PersistentFlags().BoolVar(
		&dotOutput, "dot", false,
		"Output the graph as a DOT digraph (graph description language)")

	return cmd
}

// dashdashArgsToMap is a simple args parser that places incoming key/value pairs into a map.  These are then used
// during package compilation as inputs to the main entrypoint function.
// IDEA: this is fairly rudimentary; we eventually want to support arrays, maps, and complex types.
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
