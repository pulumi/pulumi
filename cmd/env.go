// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newEnvCmd() *cobra.Command {
	var showIDs bool
	var showURNs bool
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage target environments",
		Long: "Manage target environments\n" +
			"\n" +
			"An environment is a named update target, and a single project may have many of them.\n" +
			"Each environment has a configuration and update history associated with it, stored in\n" +
			"the workspace, in addition to a full checkpoint of the last known good update.\n",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return lumiEngine.EnvInfo(showIDs, showURNs)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&showIDs, "show-ids", "i", false, "Display each resource's provider-assigned unique ID")
	cmd.PersistentFlags().BoolVarP(
		&showURNs, "show-urns", "u", false, "Display each resource's Pulumi-assigned globally unique URN")

	cmd.AddCommand(newEnvInitCmd())
	cmd.AddCommand(newEnvLsCmd())
	cmd.AddCommand(newEnvRmCmd())
	cmd.AddCommand(newEnvSelectCmd())

	return cmd
}
