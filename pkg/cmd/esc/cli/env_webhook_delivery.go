// Copyright 2026, Pulumi Corporation.

package cli

import (
	"github.com/spf13/cobra"
)

func newEnvWebhookDeliveryCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delivery",
		Short: "Manage environment webhook deliveries",
		Long: "[EXPERIMENTAL] Manage environment webhook deliveries\n" +
			"\n" +
			"A delivery is a single attempt to deliver a webhook payload.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
	}

	cmd.AddCommand(newEnvWebhookDeliveryListCmd(env))

	return cmd
}
