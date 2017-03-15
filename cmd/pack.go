// Copyright 2017 Pulumi, Inc. All rights reserved.

package cmd

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
