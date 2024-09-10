// Copyright 2024, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newEnvTagRmCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm [<org-name>/][<project-name>/]<environment-name> <tag-name>",
		Short: "Remove an environment tag.",
		Long: "Remove an environment tag\n" +
			"\n" +
			"This command removes an environment tag using the tag name.\n",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return fmt.Errorf("the ls command does not accept versions")
			}

			tagIdentifier := args[1]

			if err := env.esc.client.DeleteEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, tagIdentifier); err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout, "Successfully deleted environment tag: %v\n", tagIdentifier)
			return nil
		},
	}

	return cmd
}
