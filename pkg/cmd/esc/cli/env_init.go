// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newEnvInitCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [<org-name>/]<environment-name>",
		Args:  cobra.MaximumNArgs(1),
		Short: "Create an empty environment with the given name.",
		Long: "Create an empty environment with the given name, ready for editing\n" +
			"\n" +
			"This command creates an empty environment with the given name. It has no definition,\n" +
			"but afterwards it can be edited using the `edit` command.\n" +
			"\n" +
			"To create an environment in an organization when logged in to the Pulumi Cloud,\n" +
			"prefix the stack name with the organization name and a slash (e.g. 'acmecorp/dev').\n",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			orgName, envName, args, err := env.getEnvName(args)
			if err != nil {
				return err
			}
			_ = args

			return env.esc.client.CreateEnvironment(ctx, orgName, envName)
		},
	}
	return cmd
}
