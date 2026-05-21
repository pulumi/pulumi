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

package org

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type orgWebhookEditRender func(cmd *orgWebhookEditCmd, wh apitype.Webhook) error

const orgRemoveSecretSentinel = "__remove-secret"

type orgWebhookEditCmd struct {
	orgName string
	output  outputflag.OutputFlag[orgWebhookEditRender]
	w       io.Writer

	// Flags.
	url          string
	format       string
	addEvents    []string
	removeEvents []string
	addGroups    []string
	removeGroups []string
	active       bool
	secret       string
	displayName  string

	ws             pkgWorkspace.Context
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
		*workspace.Project, display.Options,
	) (backend.Backend, error)
}

func newOrgWebhookEditCmd() *cobra.Command {
	oecmd := &orgWebhookEditCmd{
		output: outputflag.OutputFlag[orgWebhookEditRender]{
			RenderForTerminal: (*orgWebhookEditCmd).renderText,
			RenderJSON:        (*orgWebhookEditCmd).renderJSON,
		},
		ws:             pkgWorkspace.Instance,
		currentBackend: cmdBackend.CurrentBackend,
	}

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "[EXPERIMENTAL] Update an organization webhook's configuration",
		Long: "[EXPERIMENTAL] Update an organization webhook's configuration.\n" +
			"\n" +
			"Modifies an existing webhook. Only the flags you pass are changed;\n" +
			"all other fields are preserved.\n" +
			"\n" +
			"Use --add-event/--remove-event and --add-group/--remove-group to\n" +
			"modify event subscriptions incrementally. To clear the secret,\n" +
			"pass --secret \"\".",
		Example: "  # Disable a webhook\n" +
			"  pulumi org webhook edit 1a2b3c4d --active=false\n\n" +
			"  # Change the payload URL\n" +
			"  pulumi org webhook edit 1a2b3c4d --url https://new-url.example.com\n\n" +
			"  # Add an event and remove another\n" +
			"  pulumi org webhook edit 1a2b3c4d \\\n" +
			"    --add-event deployment_failed --remove-event deployment_started\n\n" +
			"  # Add a group\n" +
			"  pulumi org webhook edit 1a2b3c4d --add-group environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			oecmd.w = cmd.OutOrStdout()
			return oecmd.run(cmd.Context(), cmd, args[0])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "id"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&oecmd.orgName, "org", "",
		"The organization that owns the webhook. Defaults to the current org.")
	cmd.Flags().StringVar(&oecmd.url, "url", "",
		"The webhook payload URL")
	cmd.Flags().StringVar(&oecmd.format, "hook-format", "",
		"The webhook format: raw, slack, or ms_teams")
	cmd.Flags().StringArrayVar(&oecmd.addEvents, "add-event", nil,
		"An event type to add (repeatable)")
	cmd.Flags().StringArrayVar(&oecmd.removeEvents, "remove-event", nil,
		"An event type to remove (repeatable)")
	cmd.Flags().StringArrayVar(&oecmd.addGroups, "add-group", nil,
		"An event group to add (repeatable)")
	cmd.Flags().StringArrayVar(&oecmd.removeGroups, "remove-group", nil,
		"An event group to remove (repeatable)")
	cmd.Flags().BoolVar(&oecmd.active, "active", true,
		"Whether the webhook is active")
	cmd.Flags().StringVar(&oecmd.secret, "secret", "",
		"The HMAC key for signature verification (empty string removes the secret)")
	cmd.Flags().StringVar(&oecmd.displayName, "display-name", "",
		"The webhook display name")
	outputflag.VarP(cmd.Flags(), &oecmd.output)

	return cmd
}

func (c *orgWebhookEditCmd) run(ctx context.Context, cobraCmd *cobra.Command, webhookName string) error {
	project, _, err := c.ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
	be, err := c.currentBackend(ctx, c.ws, cmdBackend.DefaultLoginManager, project, displayOpts)
	if err != nil {
		return err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return errors.New("this command requires the Pulumi Cloud backend; run `pulumi login`")
	}

	orgName, err := resolveOrgName(ctx, c.orgName, cloudBackend)
	if err != nil {
		return err
	}

	cl := cloudBackend.Client()

	existing, err := cl.GetOrgWebhook(ctx, orgName, webhookName)
	if err != nil {
		return fmt.Errorf("reading webhook %q: %w", webhookName, err)
	}

	// Apply only explicitly-set flags.
	if cobraCmd.Flags().Changed("display-name") {
		existing.DisplayName = c.displayName
	}
	if cobraCmd.Flags().Changed("url") {
		existing.PayloadURL = c.url
	}
	if cobraCmd.Flags().Changed("hook-format") {
		existing.Format = &c.format
	}
	if cobraCmd.Flags().Changed("active") {
		existing.Active = c.active
	}
	if cobraCmd.Flags().Changed("secret") {
		if c.secret == "" {
			existing.Secret = orgRemoveSecretSentinel
		} else {
			existing.Secret = c.secret
		}
	}

	// Remove first, then add (order matters for idempotency).
	if cobraCmd.Flags().Changed("remove-event") {
		existing.Filters = removeFromSlice(existing.Filters, c.removeEvents)
	}
	if cobraCmd.Flags().Changed("add-event") {
		existing.Filters = addToSlice(existing.Filters, c.addEvents)
	}
	if cobraCmd.Flags().Changed("remove-group") {
		existing.Groups = removeFromSlice(existing.Groups, c.removeGroups)
	}
	if cobraCmd.Flags().Changed("add-group") {
		existing.Groups = addToSlice(existing.Groups, c.addGroups)
	}

	if err := validateOrgGroupsAndEvents(existing.Groups, existing.Filters); err != nil {
		return err
	}

	updated, err := cl.UpdateOrgWebhook(ctx, orgName, webhookName, existing)
	if err != nil {
		return fmt.Errorf("updating webhook %q: %w", webhookName, err)
	}

	return c.output.Get()(c, updated)
}

// removeFromSlice returns existing with all elements in remove filtered out.
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

// addToSlice returns existing with add appended, deduplicating.
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

func (c *orgWebhookEditCmd) renderText(wh apitype.Webhook) error {
	fmt.Fprintf(c.w, "Updated webhook %q\n", wh.Name)
	return nil
}

func (c *orgWebhookEditCmd) renderJSON(wh apitype.Webhook) error {
	enc := json.NewEncoder(c.w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toOrgWebhookJSON(wh))
}
