// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newEnvWebhookRmCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm [<org-name>/][<project-name>/]<environment-name> <webhook-name>",
		Short: "Remove an environment webhook.",
		Long: "[EXPERIMENTAL] Remove an environment webhook\n" +
			"\n" +
			"This command removes the named webhook from the environment.\n",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the rm command does not accept versions")
			}

			webhookName := args[0]
			if webhookName == "" {
				return errors.New("webhook name cannot be empty")
			}

			if err := env.esc.client.DeleteEnvironmentWebhook(
				ctx, ref.orgName, ref.projectName, ref.envName, webhookName); err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout, "Removed webhook %s from %s/%s/%s\n",
				webhookName, ref.orgName, ref.projectName, ref.envName)
			return nil
		},
	}

	return cmd
}
