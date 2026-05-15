// Copyright 2026, Pulumi Corporation.

package cli

import (
	"github.com/spf13/cobra"
)

func newEnvWebhookCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage environment webhooks",
		Long: "[EXPERIMENTAL] Manage environment webhooks\n" +
			"\n" +
			"A webhook delivers JSON event payloads to a URL whenever the environment changes.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
	}

	cmd.AddCommand(newEnvWebhookListCmd(env))
	cmd.AddCommand(newEnvWebhookGetCmd(env))
	cmd.AddCommand(newEnvWebhookNewCmd(env))
	cmd.AddCommand(newEnvWebhookEditCmd(env))
	cmd.AddCommand(newEnvWebhookRmCmd(env))
	cmd.AddCommand(newEnvWebhookPingCmd(env))
	cmd.AddCommand(newEnvWebhookDeliveryCmd(env))

	return cmd
}
