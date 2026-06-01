// Copyright 2026, Pulumi Corporation.

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvWebhookNewCmd(env *envCommand) *cobra.Command {
	var (
		url    string
		format string
		events []string
		groups []string
		active bool
		secret string
	)

	cmd := &cobra.Command{
		Use:   "new [<org-name>/][<project-name>/]<environment-name> <webhook-display-name>",
		Short: "Create a new environment webhook.",
		Long: "[EXPERIMENTAL] Create a new environment webhook\n" +
			"\n" +
			"This command attaches a new webhook to the given environment. The positional\n" +
			"argument is the human-readable display name; the service generates the webhook's\n" +
			"unique name, which is printed on success and is the identifier used by the other\n" +
			"`esc env webhook` subcommands (edit, get, rm, ping, delivery list).\n" +
			"\n" +
			"The webhook will be delivered to --url whenever the environment changes. Use\n" +
			"--event to limit the set of events that trigger a delivery, or --group to\n" +
			"subscribe to every event in a named group (valid groups for environment\n" +
			"webhooks: environments, change_requests). Both flags are repeatable. Event\n" +
			"and group names are validated by the service.\n" +
			"\n" +
			"Allowed --format values are: raw (default), slack, ms_teams, pulumi_deployments.\n" +
			"\n" +
			"URL requirements depend on --format:\n" +
			"  raw, ms_teams:      any http(s) URL\n" +
			"  slack:              must begin with https://hooks.slack.com/\n" +
			"  pulumi_deployments: must be of the form <project>/<stack>\n",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the new command does not accept versions")
			}

			displayName := args[0]
			if displayName == "" {
				return errors.New("webhook display name cannot be empty")
			}
			if url == "" {
				return errors.New("--url is required")
			}
			if err := validateWebhookFormat(format); err != nil {
				return err
			}
			if err := validateWebhookURL(format, url); err != nil {
				return err
			}

			req := client.CreateEnvironmentWebhookRequest{
				Active:      active,
				DisplayName: displayName,
				// Name is intentionally left empty; the service generates the unique webhook
				// name (the identifier used by edit/get/rm/ping/delivery list) on create.
				Name:             "",
				OrganizationName: ref.orgName,
				ProjectName:      ref.projectName,
				EnvName:          ref.envName,
				PayloadURL:       url,
				Filters:          events,
				Groups:           groups,
				Format:           format,
				Secret:           secret,
			}

			w, err := env.esc.client.CreateEnvironmentWebhook(ctx, ref.orgName, ref.projectName, ref.envName, req)
			if err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout,
				"Created webhook %q for %s/%s/%s; use this name to reference the webhook in other commands.\n",
				w.Name, ref.orgName, ref.projectName, ref.envName)
			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "", "The payload URL to deliver events to (required)")
	cmd.Flags().StringVar(&format, "format", "raw", "The payload format")
	cmd.Flags().StringArrayVar(&events, "event", nil, "Event types to subscribe to (repeatable)")
	cmd.Flags().StringArrayVar(&groups, "group", nil, "Event groups to subscribe to (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "", "Shared secret used to sign deliveries")

	return cmd
}
