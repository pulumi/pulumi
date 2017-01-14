// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/encoding"
	decode "github.com/marapongo/mu/pkg/pack/encoding"
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
				// Lookup the marshaler for this format.
				ext := filepath.Ext(arg)
				m, has := encoding.Marshalers[ext]
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
				pkg, err := decode.Decode(m, b)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: a problem occurred when unmarshaling file '%v'\n", arg)
					fmt.Fprintf(os.Stderr, "       %v\n", err)
					return
				}

				// Pretty-print the package metadata:
				fmt.Printf("Package %v\n", pkg.Name)
				fmt.Printf("\tpath = %v\n", arg)
				if pkg.Description != "" {
					fmt.Printf("\tdescription = %v\n", pkg.Description)
				}
				if pkg.Author != "" {
					fmt.Printf("\tauthor = %v\n", pkg.Author)
				}
				if pkg.Website != "" {
					fmt.Printf("\twebsite = %v\n", pkg.Website)
				}
				if pkg.License != "" {
					fmt.Printf("\tlicense = %v\n", pkg.License)
				}

				// Print the dependencies:
				fmt.Printf("\tdependencies = [")
				if pkg.Dependencies != nil && len(*pkg.Dependencies) > 0 {
					fmt.Printf("\n")
					for _, dep := range *pkg.Dependencies {
						fmt.Printf("\t\t%v", dep)
					}
					fmt.Printf("\n\t")
				}
				fmt.Printf("]\n")

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
