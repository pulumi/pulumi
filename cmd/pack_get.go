// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/util/contract"
)

func newPackGetCmd() *cobra.Command {
	var global bool
	var save bool
	var cmd = &cobra.Command{
		Use:   "get [deps]",
		Short: "Download a package",
		Long: "Get downloads a package by name.  If run without arguments, get will attempt\n" +
			"to download dependencies referenced by the current package.  Otherwise, if one\n" +
			"or more specific dependencies are provided, only those will be downloaded.",
		Run: runFunc(func(cmd *cobra.Command, args []string) error {
			contract.Failf("Get command is not yet implemented")
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&global, "global", "g", false,
		"Install to a shared location on this machine")
	cmd.PersistentFlags().BoolVarP(
		&save, "save", "s", false,
		"Save new dependencies in the current package")

	return cmd
}
