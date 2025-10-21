// Copyright 2025, Pulumi Corporation.

package cli

import (
	"github.com/spf13/cobra"
)

func newEnvSettingsCmd(env *envCommand) *cobra.Command {
	registry := NewEnvSettingsRegistry()

	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Manage environment settings",
		Long: "Manage environment settings\n" +
			"\n" +
			"This command manages environment settings such as deletion protection.\n" +
			"\n" +
			"Subcommands exist for reading and updating settings.",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newEnvSettingsGetCmd(env, registry))
	cmd.AddCommand(newEnvSettingsSetCmd(env, registry))

	return cmd
}
