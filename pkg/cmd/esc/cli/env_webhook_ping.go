// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newEnvWebhookPingCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ping [<org-name>/][<project-name>/]<environment-name> <webhook-name>",
		Short: "Send a test delivery to an environment webhook.",
		Long: "[EXPERIMENTAL] Send a test delivery to an environment webhook\n" +
			"\n" +
			"This command triggers a synthetic delivery against the named webhook and prints\n" +
			"the resulting delivery record.\n",
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
				return errors.New("the ping command does not accept versions")
			}

			webhookName := args[0]
			if webhookName == "" {
				return errors.New("webhook name cannot be empty")
			}

			d, err := env.esc.client.PingEnvironmentWebhook(
				ctx, ref.orgName, ref.projectName, ref.envName, webhookName)
			if err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout, "ID: %s\n", d.ID)
			fmt.Fprintf(env.esc.stdout, "Kind: %s\n", d.Kind)
			fmt.Fprintf(env.esc.stdout, "Response code: %d\n", d.ResponseCode)
			fmt.Fprintf(env.esc.stdout, "Duration (ms): %d\n", d.Duration)
			return nil
		},
	}

	return cmd
}
