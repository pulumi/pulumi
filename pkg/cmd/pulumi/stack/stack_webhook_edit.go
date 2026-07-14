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

package stack

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackWebhookEditClient is the interface the edit command needs.
type stackWebhookEditClient interface {
	GetStackWebhook(
		ctx context.Context, stackID client.StackIdentifier, webhookName string,
	) (apitype.Webhook, error)
	UpdateStackWebhook(
		ctx context.Context, stackID client.StackIdentifier,
		webhookName string, req apitype.Webhook,
	) (apitype.Webhook, error)
}

type stackWebhookEditClientFactory func(
	ctx context.Context, stackFlag string,
) (stackWebhookEditClient, client.StackIdentifier, error)

func newStackWebhookEditCmd() *cobra.Command {
	return newStackWebhookEditCmdWith(nil)
}

func newStackWebhookEditCmdWith(factory stackWebhookEditClientFactory) *cobra.Command {
	var (
		stack        string
		url          string
		format       string
		addEvents    []string
		removeEvents []string
		addGroups    []string
		removeGroups []string
		active       bool
		secret       string
		displayName  string
	)
	output := outputflag.OutputFlag[webhookGetRenderFunc]{
		RenderForTerminal: renderWebhookGetText,
		RenderJSON:        renderWebhookGetJSON,
	}

	cmd := &cobra.Command{
		Use:     "edit",
		Aliases: []string{"update", "modify"},
		Short:   "[EXPERIMENTAL] Update a stack webhook's configuration",
		Long: "[EXPERIMENTAL] Update a stack webhook's configuration.\n" +
			"\n" +
			"Modifies an existing webhook. Only the flags you pass are changed;\n" +
			"all other fields are preserved. For example, passing --active=false\n" +
			"disables the webhook without altering its URL or filters.\n" +
			"\n" +
			"To clear the webhook secret, pass --secret \"\".\n" +
			"\n" +
			"Returns an error if the webhook does not exist.",
		Example: "  # Disable a webhook\n" +
			"  pulumi stack webhook edit 1a2b3c4d --active=false\n\n" +
			"  # Change the payload URL\n" +
			"  pulumi stack webhook edit 1a2b3c4d --url https://new-url.example.com\n\n" +
			"  # Add event filters\n" +
			"  pulumi stack webhook edit 1a2b3c4d " +
			"--add-event update_succeeded --add-event update_failed\n\n" +
			"  # Remove an event and add another\n" +
			"  pulumi stack webhook edit 1a2b3c4d " +
			"--remove-event update_failed --add-event destroy_failed\n\n" +
			"  # Clear the secret\n" +
			"  pulumi stack webhook edit 1a2b3c4d --secret \"\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultEditClientFactory
			}
			return runEdit(
				cmd.Context(), cmd.OutOrStdout(), cmd, factory,
				stack, args[0], editFlags{
					url:          url,
					format:       format,
					addEvents:    addEvents,
					removeEvents: removeEvents,
					addGroups:    addGroups,
					removeGroups: removeGroups,
					active:       active,
					secret:       secret,
					displayName:  displayName,
				}, output.Get(),
			)
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&url, "url", "",
		"The webhook payload URL")
	cmd.Flags().StringVar(&format, "hook-format", "",
		"The webhook format: raw, slack, ms_teams, or pulumi_deployments")
	cmd.Flags().StringArrayVar(&addEvents, "add-event", nil,
		"An event type to add (repeatable)")
	cmd.Flags().StringArrayVar(&removeEvents, "remove-event", nil,
		"An event type to remove (repeatable)")
	cmd.Flags().StringArrayVar(&addGroups, "add-group", nil,
		"An event group to add (repeatable)")
	cmd.Flags().StringArrayVar(&removeGroups, "remove-group", nil,
		"An event group to remove (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true,
		"Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "",
		"The HMAC key for signature verification (empty string removes the secret)")
	cmd.Flags().StringVar(&displayName, "display-name", "",
		"The webhook display name")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}

type editFlags struct {
	url          string
	format       string
	addEvents    []string
	removeEvents []string
	addGroups    []string
	removeGroups []string
	active       bool
	secret       string
	displayName  string
}

func defaultEditClientFactory(
	ctx context.Context, stackFlag string,
) (stackWebhookEditClient, client.StackIdentifier, error) {
	return RequireCloudStack(
		ctx, cmdutil.Diag(), pkgWorkspace.Instance,
		cmdBackend.DefaultLoginManager, stackFlag)
}

func runEdit(
	ctx context.Context,
	w io.Writer,
	cmd *cobra.Command,
	factory stackWebhookEditClientFactory,
	stackFlag string,
	webhookName string,
	flags editFlags,
	render webhookGetRenderFunc,
) error {
	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	// Fetch the existing webhook so we only change what was explicitly set.
	existing, err := c.GetStackWebhook(ctx, stackID, webhookName)
	if err != nil {
		return fmt.Errorf("reading webhook %q: %w", webhookName, err)
	}

	req, err := applyEditFlags(cmd, existing, flags)
	if err != nil {
		return err
	}

	updated, err := c.UpdateStackWebhook(ctx, stackID, webhookName, req)
	if err != nil {
		return fmt.Errorf("updating webhook %q: %w", webhookName, err)
	}

	return render(w, updated)
}

// removeSecretSentinel is the value the Pulumi Service recognises as
// "clear the secret" in a PATCH request.
const removeSecretSentinel = "__remove-secret"

// applyEditFlags merges explicitly-set flags into the existing webhook
// and validates the result.
func applyEditFlags(
	cmd *cobra.Command, wh apitype.Webhook, f editFlags,
) (apitype.Webhook, error) {
	if cmd.Flags().Changed("display-name") {
		wh.DisplayName = f.displayName
	}
	if cmd.Flags().Changed("url") {
		wh.PayloadURL = f.url
	}
	if cmd.Flags().Changed("hook-format") {
		wh.Format = &f.format
	}
	if cmd.Flags().Changed("remove-event") {
		wh.Filters = removeFromSlice(wh.Filters, f.removeEvents)
	}
	if cmd.Flags().Changed("add-event") {
		wh.Filters = addToSlice(wh.Filters, f.addEvents)
	}
	if cmd.Flags().Changed("remove-group") {
		wh.Groups = removeFromSlice(wh.Groups, f.removeGroups)
	}
	if cmd.Flags().Changed("add-group") {
		wh.Groups = addToSlice(wh.Groups, f.addGroups)
	}
	if cmd.Flags().Changed("active") {
		wh.Active = f.active
	}
	if cmd.Flags().Changed("secret") {
		if f.secret == "" {
			wh.Secret = removeSecretSentinel
		} else {
			wh.Secret = f.secret
		}
	}

	if err := validateGroupsAndEvents(wh.Groups, wh.Filters); err != nil {
		return apitype.Webhook{}, err
	}
	return wh, nil
}

// removeFromSlice returns a new slice with elements in remove excluded.
func removeFromSlice(existing, remove []string) []string {
	toRemove := make(map[string]bool, len(remove))
	for _, r := range remove {
		toRemove[r] = true
	}
	var result []string
	for _, v := range existing {
		if !toRemove[v] {
			result = append(result, v)
		}
	}
	return result
}

// addToSlice returns a new slice with elements in add appended,
// skipping any that are already present.
func addToSlice(existing, add []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, v := range existing {
		seen[v] = true
	}
	result := make([]string, len(existing))
	copy(result, existing)
	for _, v := range add {
		if !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}
