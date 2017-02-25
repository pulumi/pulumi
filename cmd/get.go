// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/util/contract"
)

func newGetCmd() *cobra.Command {
	var global bool
	var save bool
	var cmd = &cobra.Command{
		Use:   "get [deps]",
		Short: "Download a Nut",
		Long: "Get downloads a Nut by name.  If run without arguments, get will attempt\n" +
			"to download dependencies referenced by the current Nut.  Otherwise, if one\n" +
			"or more specific dependencies are provided, only those will be downloaded.",
		Run: func(cmd *cobra.Command, args []string) {
			contract.Failf("Get command is not yet implemented")
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&global, "global", "g", false,
		"Install to a shared location on this machine")
	cmd.PersistentFlags().BoolVarP(
		&save, "save", "s", false,
		"Save new dependencies in the current Nut")

	return cmd
}
