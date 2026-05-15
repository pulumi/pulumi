// Copyright 2026, Pulumi Corporation.

package cli

import (
	"github.com/spf13/cobra"
)

func newEnvProviderCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage login providers within an environment",
		Long:  "[EXPERIMENTAL] Manage providers within an environment\n",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newEnvProviderAWSLoginCmd(env))
	cmd.AddCommand(newEnvProviderAzureLoginCmd(env))
	cmd.AddCommand(newEnvProviderGCPLoginCmd(env))

	return cmd
}
