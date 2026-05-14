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
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type orgWebhookNewRender func(cmd *orgWebhookNewCmd, wh apitype.Webhook) error

// Valid event groups for org webhooks (superset of stack groups).
var orgWebhookGroups = []string{
	"stacks", "deployments", "policies",
	"environments", "change_requests",
}

// orgGroupFilters maps each event group to its contained filters for org webhooks.
var orgGroupFilters = map[string][]string{
	"stacks": {
		"stack_created", "stack_deleted",
		"update_succeeded", "update_failed",
		"preview_succeeded", "preview_failed",
		"destroy_succeeded", "destroy_failed",
		"refresh_succeeded", "refresh_failed",
		"import_succeeded", "import_failed",
	},
	"deployments": {
		"deployment_queued", "deployment_started",
		"deployment_succeeded", "deployment_failed",
		"drift_detected",
		"drift_detection_succeeded", "drift_detection_failed",
		"drift_remediation_succeeded", "drift_remediation_failed",
		"deployment_file_ingestion_failed",
	},
	"policies": {
		"policy_violation_mandatory", "policy_violation_advisory",
	},
	"environments": {
		"environment_created", "environment_deleted",
		"environment_revision_created", "environment_revision_retracted",
		"environment_revision_tag_created", "environment_revision_tag_deleted",
		"environment_revision_tag_updated",
		"environment_tag_created", "environment_tag_deleted",
		"environment_tag_updated",
		"imported_environment_changed",
	},
	"change_requests": {
		"change_request_created", "change_request_updated",
		"change_request_approved", "change_request_unapproved",
		"change_request_closed", "change_request_applied",
		"change_request_submitted",
	},
}

type orgWebhookNewCmd struct {
	orgName string
	output  outputflag.OutputFlag[orgWebhookNewRender]
	w       io.Writer

	// Flags.
	name   string
	url    string
	format string
	events []string
	groups []string
	active bool
	secret string
	yes    bool

	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
		*workspace.Project, display.Options,
	) (backend.Backend, error)
}

func newOrgWebhookNewCmd() *cobra.Command {
	return newOrgWebhookNewCmdWith(nil)
}

func newOrgWebhookNewCmdWith(
	overrideBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
		*workspace.Project, display.Options,
	) (backend.Backend, error),
) *cobra.Command {
	oncmd := &orgWebhookNewCmd{
		output: outputflag.OutputFlag[orgWebhookNewRender]{
			RenderForTerminal: (*orgWebhookNewCmd).renderText,
			RenderJSON:        (*orgWebhookNewCmd).renderJSON,
		},
		currentBackend: overrideBackend,
	}

	cmd := &cobra.Command{
		Use:   "new",
		Short: "[EXPERIMENTAL] Create a new organization webhook",
		Long: "Create a new organization webhook.\n" +
			"\n" +
			"Creates a webhook that delivers events for the specified organization\n" +
			"to a given URL. Organization webhooks can fire on stack lifecycle,\n" +
			"deployment, drift detection, environment, and policy violation events.\n" +
			"\n" +
			"When run interactively, prompts for required values that aren't\n" +
			"provided via flags. Pass --yes to accept defaults without prompting.",
		Example: "  # Create a webhook interactively\n" +
			"  pulumi org webhook new --name my-hook\n\n" +
			"  # Create non-interactively\n" +
			"  pulumi org webhook new --name my-hook --url https://example.com/hook --yes\n\n" +
			"  # Create with specific event groups\n" +
			"  pulumi org webhook new --name my-hook \\\n" +
			"    --url https://example.com --group stacks --group deployments --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			oncmd.w = cmd.OutOrStdout()
			return oncmd.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&oncmd.name, "name", "",
		"The webhook name (1-32 chars, alphanumeric/hyphens/underscores/dots)")
	cmd.Flags().StringVar(&oncmd.orgName, "org", "",
		"The organization to create the webhook in. Defaults to the current org.")
	cmd.Flags().StringVar(&oncmd.url, "url", "",
		"The webhook payload URL, including protocol (e.g. https://example.com/hook)")
	cmd.Flags().StringVar(&oncmd.format, "hook-format", "raw",
		"The webhook format: raw, slack, or ms_teams")
	cmd.Flags().StringArrayVar(&oncmd.events, "event", nil,
		"An event type to subscribe to (repeatable)")
	cmd.Flags().StringArrayVar(&oncmd.groups, "group", nil,
		"An event group to subscribe to (repeatable)")
	cmd.Flags().BoolVar(&oncmd.active, "active", true,
		"Whether the webhook is active")
	cmd.Flags().StringVar(&oncmd.secret, "secret", "",
		"The HMAC key for signature verification")
	cmd.Flags().BoolVarP(&oncmd.yes, "yes", "y", false,
		"Skip prompts and proceed with default values")
	outputflag.VarP(cmd.Flags(), &oncmd.output)

	return cmd
}

func (c *orgWebhookNewCmd) run(ctx context.Context) error {
	currentBackend := c.currentBackend
	if currentBackend == nil {
		currentBackend = cmdBackend.CurrentBackend
	}

	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
	be, err := currentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, displayOpts)
	if err != nil {
		return err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return errors.New("this command requires the Pulumi Cloud backend; run `pulumi login`")
	}

	orgName := c.orgName
	if orgName == "" {
		orgName, err = cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return fmt.Errorf("resolving default org: %w", err)
		}
		if orgName == "" {
			userName, _, _, err := cloudBackend.CurrentUser()
			if err != nil {
				return err
			}
			orgName = userName
		}
	}

	// Resolve interactive values.
	skipPrompts := c.yes || !cmdutil.Interactive()

	name := c.name
	if name == "" {
		nameErrMsg := "a webhook name is required (use --name)"
		if !skipPrompts {
			nameErrMsg = "a webhook name is required"
		}
		name, err = ui.PromptForValue(
			skipPrompts, "Webhook name", "", false,
			func(v string) error {
				if v == "" {
					return errors.New(nameErrMsg) //nolint:goerr113 // intentional
				}
				return nil
			}, displayOpts)
		if err != nil {
			return err
		}
	}

	webhookURL := c.url
	if webhookURL == "" {
		if !skipPrompts {
			fmt.Fprintln(c.w,
				"Enter the URL that will receive webhook payloads "+
					"(e.g. https://example.com/webhook).")
		}
		urlErrMsg := "a payload URL is required (use --url)"
		if !skipPrompts {
			urlErrMsg = "a payload URL is required"
		}
		webhookURL, err = ui.PromptForValue(
			skipPrompts, "Webhook payload URL", "", false,
			func(v string) error {
				if v == "" {
					return errors.New(urlErrMsg) //nolint:goerr113 // intentional
				}
				return nil
			}, displayOpts)
		if err != nil {
			return err
		}
	}

	format := c.format
	if !skipPrompts {
		formats := []string{"raw", "slack", "ms_teams"}
		format = ui.PromptUserSkippable(
			false, "Webhook format", formats, format, displayOpts.Color)
	}

	groups := c.groups
	if len(groups) == 0 && !skipPrompts {
		groups = ui.PromptUserMultiSkippable(
			false, "Event groups to subscribe to",
			orgWebhookGroups, orgWebhookGroups,
			displayOpts.Color)
	}

	events := c.events
	if len(events) == 0 && !skipPrompts {
		remaining := orgFiltersNotCoveredByGroups(groups)
		if len(remaining) > 0 {
			events = ui.PromptUserMultiSkippable(
				false, "Additional events to subscribe to",
				remaining, nil, displayOpts.Color)
		}
	}

	if err := validateOrgGroupsAndEvents(groups, events); err != nil {
		return err
	}

	formatPtr := &format
	if format == "" {
		formatPtr = nil
	}

	req := apitype.Webhook{
		OrganizationName: orgName,
		DisplayName:      name,
		PayloadURL:       webhookURL,
		Active:           c.active,
		Format:           formatPtr,
		Groups:           groups,
		Filters:          events,
	}
	if c.secret != "" {
		req.Secret = c.secret
	}

	created, err := cloudBackend.Client().CreateOrgWebhook(ctx, orgName, req)
	if err != nil {
		return fmt.Errorf("creating organization webhook: %w", err)
	}

	return c.output.Get()(c, created)
}

func (c *orgWebhookNewCmd) renderText(wh apitype.Webhook) error {
	fmt.Fprintf(c.w, "Created webhook %q\n", wh.Name)
	return nil
}

func (c *orgWebhookNewCmd) renderJSON(wh apitype.Webhook) error {
	enc := json.NewEncoder(c.w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toOrgWebhookJSON(wh))
}

// orgFiltersNotCoveredByGroups returns event filters not covered by selected groups.
func orgFiltersNotCoveredByGroups(selectedGroups []string) []string {
	covered := make(map[string]bool)
	for _, g := range selectedGroups {
		for _, f := range orgGroupFilters[g] {
			covered[f] = true
		}
	}
	var remaining []string
	for _, group := range orgWebhookGroups {
		for _, f := range orgGroupFilters[group] {
			if !covered[f] {
				remaining = append(remaining, f)
			}
		}
	}
	return remaining
}

var validOrgGroupSet = func() map[string]bool {
	m := make(map[string]bool, len(orgWebhookGroups))
	for _, g := range orgWebhookGroups {
		m[g] = true
	}
	return m
}()

var validOrgEventSet = func() map[string]bool {
	m := make(map[string]bool)
	for _, events := range orgGroupFilters {
		for _, e := range events {
			m[e] = true
		}
	}
	return m
}()

func validateOrgGroupsAndEvents(groups, events []string) error {
	for _, g := range groups {
		if !validOrgGroupSet[g] {
			return fmt.Errorf("invalid group %q; valid groups: %s",
				g, joinOrgQuoted(orgWebhookGroups))
		}
	}
	for _, e := range events {
		if !validOrgEventSet[e] {
			return fmt.Errorf("invalid event %q", e)
		}
	}
	covered := make(map[string]string)
	for _, g := range groups {
		for _, e := range orgGroupFilters[g] {
			covered[e] = g
		}
	}
	for _, e := range events {
		if g, ok := covered[e]; ok {
			return fmt.Errorf(
				"event %q is already included by group %q; "+
					"remove the event or the group", e, g)
		}
	}
	return nil
}

func joinOrgQuoted(ss []string) string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}
