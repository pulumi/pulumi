// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/spf13/cobra"
)

func newPackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "Manage packages",
	}

	cmd.AddCommand(newPackEvalCmd())
	cmd.AddCommand(newPackInfoCmd())
	cmd.AddCommand(newPackGetCmd())
	cmd.AddCommand(newPackVerifyCmd())

	return cmd
}
