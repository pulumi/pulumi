// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"github.com/marapongo/mu/pkg/compiler"
	"github.com/spf13/cobra"
)

// defaultIn is where the Mu compiler looks for inputs by default.
const defaultInp = "."

// defaultOutput is where the Mu compiler places build artifacts by default.
const defaultOutp = ".mu"

func newBuildCmd() *cobra.Command {
	var outp string
	var cmd = &cobra.Command{
		Use:   "build [source]",
		Short: "Compile a Mu Stack",
		Run: func(cmd *cobra.Command, args []string) {
			inp := defaultInp
			if len(args) > 0 {
				inp = args[0]
			}

			mup := compiler.NewCompiler(compiler.DefaultOpts())
			mup.Build(inp, outp)
		},
	}

	cmd.PersistentFlags().StringVar(
		&outp, "out", defaultOutp,
		"The directory in which to place build artifacts",
	)

	return cmd
}
