// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
)

func newPackInfoCmd() *cobra.Command {
	var printAll bool
	var printIL bool
	var printSymbols bool
	var printExportedSymbols bool
	var cmd = &cobra.Command{
		Use:   "info [packages...]",
		Short: "Print information about one or more packages",
		Long: "Print information about one or more packages\n" +
			"\n" +
			"This command prints metadata, symbol, and/or IL from one or more packages.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// If printAll is true, flip all the flags.
			if printAll {
				printIL = true
				printSymbols = true
				printExportedSymbols = true
			}

			return lumiEngine.PackInfo(printExportedSymbols, printIL, printSymbols, args)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&printSymbols, "all", "a", false,
		"Print everything: the package, symbols, and IL")
	cmd.PersistentFlags().BoolVarP(
		&printExportedSymbols, "exports", "e", false,
		"Print just the exported symbols")
	cmd.PersistentFlags().BoolVarP(
		&printIL, "il", "i", false,
		"Pretty-print the package's IL")
	cmd.PersistentFlags().BoolVarP(
		&printSymbols, "symbols", "s", false,
		"Print a complete listing of all symbols, exported or otherwise")

	return cmd
}
