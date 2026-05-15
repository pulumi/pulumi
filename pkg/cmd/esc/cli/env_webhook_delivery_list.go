// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvWebhookDeliveryListCmd(env *envCommand) *cobra.Command {
	var (
		utc   bool
		count int
	)

	cmd := &cobra.Command{
		Use:     "list [<org-name>/][<project-name>/]<environment-name> <webhook-name>",
		Aliases: []string{"ls"},
		Short:   "List environment webhook deliveries.",
		Long: "[EXPERIMENTAL] List environment webhook deliveries\n" +
			"\n" +
			"This command lists the deliveries recorded for the named webhook.\n",
		Args: cobra.ExactArgs(2),
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
				return errors.New("the list command does not accept versions")
			}
			if count < 0 {
				return errors.New("--count must be non-negative")
			}

			webhookName := args[0]
			if webhookName == "" {
				return errors.New("webhook name cannot be empty")
			}

			deliveries, err := env.esc.client.ListEnvironmentWebhookDeliveries(
				ctx, ref.orgName, ref.projectName, ref.envName, webhookName)
			if err != nil {
				return err
			}

			if count > 0 && len(deliveries) > count {
				deliveries = deliveries[:count]
			}

			printWebhookDeliveries(env.esc.stdout, deliveries, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "Display times in UTC")
	cmd.Flags().IntVar(&count, "count", 0, "The maximum number of deliveries to return (all if unset)")

	return cmd
}

func printWebhookDeliveries(stdout io.Writer, ds []client.EnvironmentWebhookDelivery, utc utcFlag) {
	if len(ds) == 0 {
		return
	}
	t := newTable(stdout)
	t.AppendHeader(table.Row{"ID", "KIND", "TIMESTAMP", "RESPONSE", "DURATION (ms)"})
	for _, d := range ds {
		ts := time.Unix(d.Timestamp, 0)
		t.AppendRow(table.Row{
			d.ID,
			d.Kind,
			utc.time(ts).Format(time.RFC3339),
			d.ResponseCode,
			d.Duration,
		})
	}
	t.Render()
}
