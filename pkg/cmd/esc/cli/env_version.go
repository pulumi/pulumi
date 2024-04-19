// Copyright 2023, Pulumi Corporation.

package cli

import (
	"github.com/spf13/cobra"
)

func newEnvVersionCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Manage version tags",
		Long: "Manage version tags\n" +
			"\n" +
			"This command creates, inspects, updates, deletes, and lists version tags.\n" +
			"A version tag is a name that refers to a specific revision of an environment.\n" +
			"Once created, version tags can be updated to refer to new reversions\n" +
			"of an environment. Version tags can be used to refer to a particular logical\n" +
			"version of an environment rather than a specific revision.\n",
		SilenceUsage: true,
	}

	cmd.AddCommand(newEnvVersionTagCmd(env))
	cmd.AddCommand(newEnvVersionRmCmd(env))
	cmd.AddCommand(newEnvVersionLsCmd(env))

	return cmd
}
