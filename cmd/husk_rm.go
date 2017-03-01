// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newHuskRmCmd() *cobra.Command {
	var yes bool
	var force bool
	var cmd = &cobra.Command{
		Use:   "rm <husk>",
		Short: "Remove a husk and its configuration",
		Long: "Remove a husk and its configuration\n" +
			"\n" +
			"This command removes a husk and its configuration state.  Please refer to the\n" +
			"`destroy` command for removing a husk's resources, as it is a distinct operation.\n" +
			"\n" +
			"After this command completes, the husk will no longer be available for deployments.",
		Run: func(cmd *cobra.Command, args []string) {
			info := initHuskCmd(cmd, args)
			if !force && info.Old != nil && len(info.Old.Resources()) > 0 {
				exitError(
					"'%v' still has resources; removal rejected; pass --force to override", info.Husk.Name)
			}
			if yes ||
				confirmPrompt("This will permanently remove the '%v' husk!", info.Husk.Name) {
				remove(info.Husk)
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"By default, removal of a husk with resources will be rejected; this forces it")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with removal anyway")

	return cmd
}
