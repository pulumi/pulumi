// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/cmdutil"
	"github.com/marapongo/mu/pkg/pack"
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

			// Now load up the package.
			var pkg *pack.Package
			if len(args) > 0 {
				// The user has specified a path (or requested Stdin).
				pkg = cmdutil.ReadPackageFromArg(args[0])
			} else {
				// Otherwise, use default Mu package name.
				fmt.Fprintf(os.Stderr, "error: Default package names NYI")
			}
			if pkg == nil {
				return
			}

			// Next, create a compiler object, and use it to generate a MuGL graph.
			// TODO: this.
			_ = packArgs

			// Finally, serialize that MuGL graph so that it's suitable for printing/serializing.
			// TODO: this.
		},
	}

	return cmd
}
