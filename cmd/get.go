// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/util/contract"
)

func newGetCmd() *cobra.Command {
	var global bool
	var save bool
	var cmd = &cobra.Command{
		Use:   "get [deps]",
		Short: "Download a Mu Stack",
		Long: "Get downloads a Mu Stack by name.  If run without arguments, get will attempt\n" +
			"to download dependencies referenced by the current Stack.  Otherwise, if one\n" +
			"or more specific dependencies are provided, only those will be downloaded.",
		Run: func(cmd *cobra.Command, args []string) {
			contract.FailM("Get command is not yet implemented")
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&global, "global", "g", false,
		"Install to a shared location on this machine")
	cmd.PersistentFlags().BoolVarP(
		&save, "save", "s", false,
		"Save new dependencies in the current Mu Stack")

	return cmd
}
