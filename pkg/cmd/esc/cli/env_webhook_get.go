// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvWebhookGetCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [<org-name>/][<project-name>/]<environment-name> <webhook-name>",
		Short: "Get an environment webhook.",
		Long: "[EXPERIMENTAL] Get an environment webhook\n" +
			"\n" +
			"This command prints the named webhook attached to the given environment.\n",
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
				return errors.New("the get command does not accept versions")
			}

			webhookName := args[0]
			if webhookName == "" {
				return errors.New("webhook name cannot be empty")
			}

			w, err := env.esc.client.GetEnvironmentWebhook(ctx, ref.orgName, ref.projectName, ref.envName, webhookName)
			if err != nil {
				return err
			}

			printWebhook(env.esc.stdout, *w)
			return nil
		},
	}

	return cmd
}

// printWebhook renders a single webhook as a key/value block.
func printWebhook(stdout io.Writer, w client.EnvironmentWebhook) {
	fmt.Fprintf(stdout, "Name: %s\n", w.Name)
	fmt.Fprintf(stdout, "Display name: %s\n", w.DisplayName)
	fmt.Fprintf(stdout, "URL: %s\n", w.PayloadURL)
	fmt.Fprintf(stdout, "Active: %t\n", w.Active)
	format := w.Format
	if format == "" {
		format = "-"
	}
	fmt.Fprintf(stdout, "Format: %s\n", format)
	events := "-"
	if len(w.Filters) > 0 {
		events = strings.Join(w.Filters, ", ")
	}
	fmt.Fprintf(stdout, "Events: %s\n", events)
	groups := "-"
	if len(w.Groups) > 0 {
		groups = strings.Join(w.Groups, ", ")
	}
	fmt.Fprintf(stdout, "Event groups: %s\n", groups)
	fmt.Fprintf(stdout, "Has secret: %t\n", w.HasSecret)
}
