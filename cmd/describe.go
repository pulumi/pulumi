// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/pack"
)

func newDescribeCmd() *cobra.Command {
	var printExports bool
	var printIL bool
	var printSymbols bool
	var cmd = &cobra.Command{
		Use:   "describe [packages]",
		Short: "Describe a MuPackage",
		Long:  "Describe prints package, symbol, and IL information from one or more MuPackages.",
		Run: func(cmd *cobra.Command, args []string) {
			// Enumerate the list of packages, deserialize them, and print information.
			for _, arg := range args {
				fmt.Printf("%v\n", arg)

				// Lookup the marshaler for this format.
				ext := filepath.Ext(arg)
				marshaler, has := encoding.Marshalers[ext]
				if !has {
					fmt.Fprintf(os.Stderr, "error: no marshaler found for file format '%v'\n", ext)
					return
				}

				// Read the contents.
				b, err := ioutil.ReadFile(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: a problem occurred when reading file '%v'\n", arg)
					fmt.Fprintf(os.Stderr, "       %v\n", err)
					return
				}

				// Unmarshal the contents into a fresh package.
				var pkg pack.Package
				if err := marshaler.Unmarshal(b, &pkg); err != nil {
					fmt.Fprintf(os.Stderr, "error: a problem occurred when unmarshaling file '%v'\n", arg)
					fmt.Fprintf(os.Stderr, "       %v\n", err)
					return
				}

				// TODO: pretty-print.
				// TODO: respect printExports.
				// TODO: respect printIL.
				// TODO: respect printSymbols.
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&printExports, "exports", "e", false,
		"Print just the exported symbols")
	cmd.PersistentFlags().BoolVarP(
		&printIL, "il", "i", false,
		"Pretty-print the MuIL")
	cmd.PersistentFlags().BoolVarP(
		&printSymbols, "symbols", "s", false,
		"Print a complete listing of all symbols, exported or otherwise")

	return cmd
}
