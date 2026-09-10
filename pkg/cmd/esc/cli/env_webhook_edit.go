// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
)

func newEnvWebhookEditCmd(env *envCommand) *cobra.Command {
	var (
		url          string
		displayName  string
		format       string
		events       []string
		groups       []string
		active       bool
		secret       string
		removeSecret bool
		addEvents    []string
		removeEvents []string
		addGroups    []string
		removeGroups []string
	)

	cmd := &cobra.Command{
		Use:     "edit [<org-name>/][<project-name>/]<environment-name> <webhook-name>",
		Aliases: []string{"update", "modify"},
		Short:   "Edit an environment webhook.",
		Long: "[EXPERIMENTAL] Edit an environment webhook\n" +
			"\n" +
			"This command updates one or more fields of the named webhook. The CLI fetches the\n" +
			"current webhook, applies the supplied flag values on top of it, and submits the\n" +
			"merged state to the service.\n" +
			"\n" +
			"--event replaces the event list. Use --add-event and --remove-event to apply\n" +
			"incremental changes that merge with the existing events; mixing --event with\n" +
			"either of those is not allowed. Event names are validated by the service.\n" +
			"\n" +
			"--group, --add-group, and --remove-group behave the same way for event groups.\n" +
			"Valid groups for environment webhooks are: environments, change_requests.\n" +
			"\n" +
			"Allowed --format values are: raw, slack, ms_teams, pulumi_deployments. URL\n" +
			"requirements (validated against the format that will be in effect):\n" +
			"  raw, ms_teams:      any http(s) URL\n" +
			"  slack:              must begin with https://hooks.slack.com/\n" +
			"  pulumi_deployments: must be of the form <project>/<stack>\n" +
			"\n" +
			"--secret replaces the shared secret. Use --remove-secret to clear an existing\n" +
			"secret; passing --secret \"\" leaves it unchanged.\n",
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
				return errors.New("the edit command does not accept versions")
			}

			webhookName := args[0]
			if webhookName == "" {
				return errors.New("webhook name cannot be empty")
			}

			eventChanged := cmd.Flags().Changed("event")
			addEventChanged := cmd.Flags().Changed("add-event")
			removeEventChanged := cmd.Flags().Changed("remove-event")
			if eventChanged && (addEventChanged || removeEventChanged) {
				return errors.New("--event cannot be combined with --add-event or --remove-event")
			}

			groupChanged := cmd.Flags().Changed("group")
			addGroupChanged := cmd.Flags().Changed("add-group")
			removeGroupChanged := cmd.Flags().Changed("remove-group")
			if groupChanged && (addGroupChanged || removeGroupChanged) {
				return errors.New("--group cannot be combined with --add-group or --remove-group")
			}

			secretChanged := cmd.Flags().Changed("secret")
			if secretChanged && removeSecret {
				return errors.New("--secret cannot be combined with --remove-secret")
			}

			formatChanged := cmd.Flags().Changed("format")
			urlChanged := cmd.Flags().Changed("url")
			if formatChanged {
				if err := validateWebhookFormat(format); err != nil {
					return err
				}
			}

			// The service's PATCH handler is effectively a PUT: omitted fields are
			// not "leave unchanged" but rather "set to the zero value". The CLI
			// therefore fetches the current webhook and applies flag overrides on
			// top of it before submitting the merged state.
			existing, err := env.esc.client.GetEnvironmentWebhook(
				ctx, ref.orgName, ref.projectName, ref.envName, webhookName)
			if err != nil {
				return err
			}

			req := client.UpdateEnvironmentWebhookRequest{
				Active:      existing.Active,
				DisplayName: existing.DisplayName,
				PayloadURL:  existing.PayloadURL,
				Filters:     append([]string(nil), existing.Filters...),
				Groups:      append([]string(nil), existing.Groups...),
			}
			if cmd.Flags().Changed("active") {
				req.Active = active
			}
			if cmd.Flags().Changed("display-name") {
				req.DisplayName = displayName
			}
			if urlChanged {
				req.PayloadURL = url
			}
			if formatChanged {
				v := format
				req.Format = &v
			}
			if secretChanged {
				req.Secret = secret
			} else if removeSecret {
				req.Secret = removeSecretSentinel
			}
			if eventChanged {
				req.Filters = append([]string(nil), events...)
			} else if addEventChanged || removeEventChanged {
				req.Filters = mergeEvents(existing.Filters, addEvents, removeEvents)
			}

			if groupChanged {
				req.Groups = append([]string(nil), groups...)
			} else if addGroupChanged || removeGroupChanged {
				req.Groups = mergeEvents(existing.Groups, addGroups, removeGroups)
			}

			// Cross-check the final URL against the format that will be in effect.
			effectiveFormat := existing.Format
			if formatChanged {
				effectiveFormat = format
			}
			if err := validateWebhookURL(effectiveFormat, req.PayloadURL); err != nil {
				return err
			}

			w, err := env.esc.client.UpdateEnvironmentWebhook(
				ctx, ref.orgName, ref.projectName, ref.envName, webhookName, req)
			if err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout, "Updated webhook %s for %s/%s/%s\n",
				w.Name, ref.orgName, ref.projectName, ref.envName)
			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "", "The payload URL to deliver events to")
	cmd.Flags().StringVar(&displayName, "display-name", "", "The display name")
	cmd.Flags().StringVar(&format, "format", "", "The payload format")
	cmd.Flags().StringArrayVar(&events, "event", nil, "Replace the subscribed events (repeatable)")
	cmd.Flags().StringArrayVar(&groups, "group", nil, "Replace the subscribed event groups (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "", "Shared secret used to sign deliveries")
	cmd.Flags().BoolVar(&removeSecret, "remove-secret", false, "Clear the existing shared secret")
	cmd.Flags().StringArrayVar(&addEvents, "add-event", nil, "Subscribe to an additional event (repeatable)")
	cmd.Flags().StringArrayVar(&removeEvents, "remove-event", nil, "Unsubscribe from an event (repeatable)")
	cmd.Flags().StringArrayVar(&addGroups, "add-group", nil, "Subscribe to an additional event group (repeatable)")
	cmd.Flags().StringArrayVar(&removeGroups, "remove-group", nil, "Unsubscribe from an event group (repeatable)")

	return cmd
}

// mergeEvents returns existing minus removes, then appends adds (skipping duplicates).
func mergeEvents(existing, adds, removes []string) []string {
	removeSet := map[string]struct{}{}
	for _, r := range removes {
		removeSet[r] = struct{}{}
	}
	out := make([]string, 0, len(existing)+len(adds))
	present := map[string]struct{}{}
	for _, f := range existing {
		if _, drop := removeSet[f]; drop {
			continue
		}
		if _, seen := present[f]; seen {
			continue
		}
		present[f] = struct{}{}
		out = append(out, f)
	}
	for _, a := range adds {
		if _, drop := removeSet[a]; drop {
			continue
		}
		if _, seen := present[a]; seen {
			continue
		}
		present[a] = struct{}{}
		out = append(out, a)
	}
	return out
}
