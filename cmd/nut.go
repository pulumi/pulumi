// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newNutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nut",
		Short: "Manage Nuts (packages)",
	}

	cmd.AddCommand(newNutEvalCmd())
	cmd.AddCommand(newNutInfoCmd())
	cmd.AddCommand(newNutGetCmd())
	cmd.AddCommand(newNutVerifyCmd())

	return cmd
}
