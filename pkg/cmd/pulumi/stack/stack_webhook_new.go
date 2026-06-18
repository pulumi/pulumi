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
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackWebhookNewClient is the interface the new command needs from the API client.
type stackWebhookNewClient interface {
	CreateStackWebhook(
		ctx context.Context, stackID client.StackIdentifier, req apitype.Webhook,
	) (apitype.Webhook, error)
}

// stackWebhookNewClientFactory builds a stackWebhookNewClient from the environment.
type stackWebhookNewClientFactory func(
	ctx context.Context, stackFlag string,
) (stackWebhookNewClient, client.StackIdentifier, error)

// stackWebhookNewArgs holds the resolved arguments for creating a webhook.
type stackWebhookNewArgs struct {
	Name    string
	URL     string
	Format  string
	Filters []string
	Groups  []string
	Active  bool
	Secret  string
}

func newStackWebhookNewCmd() *cobra.Command {
	return newStackWebhookNewCmdWith(nil)
}

func newStackWebhookNewCmdWith(factory stackWebhookNewClientFactory) *cobra.Command {
	var (
		stack   string
		name    string
		url     string
		format  string
		filters []string
		groups  []string
		active  bool
		secret  string
		yes     bool
	)
	output := outputflag.OutputFlag[webhookNewRenderFunc]{
		RenderForTerminal: renderWebhookGetText,
		RenderJSON:        renderWebhookGetJSON,
	}

	cmd := &cobra.Command{
		Use:   "new",
		Short: "[EXPERIMENTAL] Create a new stack webhook",
		Long: "[EXPERIMENTAL] Create a new stack webhook.\n" +
			"\n" +
			"Creates a webhook that delivers events for the specified stack to a\n" +
			"given URL.\n" +
			"\n" +
			"When run interactively, prompts for required values that aren't\n" +
			"provided via flags. Pass --yes to accept defaults without prompting.\n" +
			"\n" +
			"Supported formats: raw (default), slack, ms_teams, pulumi_deployments.\n" +
			"\n" +
			"Returns 409 if a webhook with the same name already exists.",
		Example: "  # Create a webhook interactively\n" +
			"  pulumi stack webhook new\n\n" +
			"  # Create a webhook non-interactively\n" +
			"  pulumi stack webhook new --name \"My Hook\" --url https://example.com/hook --yes\n\n" +
			"  # Create a Slack webhook with specific event filters\n" +
			"  pulumi stack webhook new --name slack-alerts \\\n" +
			"    --url https://hooks.slack.com/services/T00/B00/xxx \\\n" +
			"    --format slack \\\n" +
			"    --event update_succeeded --event update_failed --yes\n\n" +
			"  # Create a webhook and get the result as JSON\n" +
			"  pulumi stack webhook new --name \"My Hook\" --url https://example.com --yes --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackWebhookNewClientFactory
			}

			// Skip prompts when --yes is set or when running non-interactively.
			skipPrompts := yes || !cmdutil.Interactive()
			opts := display.Options{Color: cmdutil.GetGlobalColorization()}

			webhookArgs, err := resolveNewArgs(
				cmd.OutOrStdout(), skipPrompts, name, url, format,
				filters, groups, opts,
			)
			if err != nil {
				return err
			}
			webhookArgs.Active = active
			webhookArgs.Secret = secret

			return runStackWebhookNew(
				cmd.Context(), cmd.OutOrStdout(), factory,
				stack, webhookArgs, output.Get(),
			)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&name, "name", "",
		"The webhook name (required)")
	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&url, "url", "",
		"The webhook payload URL, including protocol (e.g. https://example.com/hook)")
	cmd.Flags().StringVar(&format, "hook-format", "raw",
		"The webhook format: raw, slack, ms_teams, or pulumi_deployments")
	cmd.Flags().StringArrayVar(&filters, "event", nil,
		"An event type to subscribe to (repeatable)")
	cmd.Flags().StringArrayVar(&groups, "group", nil,
		"An event group to subscribe to (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true,
		"Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "",
		"The HMAC key for signature verification")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"Skip prompts and proceed with default values")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}

// defaultStackWebhookNewClientFactory resolves the current Pulumi Cloud context.
func defaultStackWebhookNewClientFactory(
	ctx context.Context, stackFlag string,
) (stackWebhookNewClient, client.StackIdentifier, error) {
	return RequireCloudStack(
		ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

// Valid webhook formats.
var webhookFormats = []string{"raw", "slack", "ms_teams", "pulumi_deployments"}

// Valid event groups for stack webhooks.
var stackWebhookGroups = []string{"stacks", "deployments", "policies"}

// groupFilters maps each event group to its contained event filters.
var groupFilters = map[string][]string{
	"stacks": {
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
}

// validGroupSet is the set of valid group names for fast lookup.
var validGroupSet = func() map[string]bool {
	m := make(map[string]bool, len(stackWebhookGroups))
	for _, g := range stackWebhookGroups {
		m[g] = true
	}
	return m
}()

// validEventSet is the set of all valid event names for fast lookup.
var validEventSet = func() map[string]bool {
	m := make(map[string]bool)
	for _, events := range groupFilters {
		for _, e := range events {
			m[e] = true
		}
	}
	return m
}()

// validateGroupsAndEvents checks that all group and event names are valid,
// and that no event is already covered by a selected group.
func validateGroupsAndEvents(groups, events []string) error {
	for _, g := range groups {
		if !validGroupSet[g] {
			return fmt.Errorf("invalid group %q; valid groups: %s",
				g, joinQuoted(stackWebhookGroups))
		}
	}
	for _, e := range events {
		if !validEventSet[e] {
			return fmt.Errorf("invalid event %q", e)
		}
	}

	// Build the set of events already covered by the selected groups.
	covered := make(map[string]string) // event -> group
	for _, g := range groups {
		for _, e := range groupFilters[g] {
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

func joinQuoted(ss []string) string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}

// filtersNotCoveredByGroups returns event filters that are not already
// covered by the selected groups.
func filtersNotCoveredByGroups(selectedGroups []string) []string {
	covered := make(map[string]bool)
	for _, g := range selectedGroups {
		for _, f := range groupFilters[g] {
			covered[f] = true
		}
	}
	// Collect all known filters that aren't covered.
	var remaining []string
	for _, group := range stackWebhookGroups {
		for _, f := range groupFilters[group] {
			if !covered[f] {
				remaining = append(remaining, f)
			}
		}
	}
	return remaining
}

// resolveNewArgs prompts for any required values not provided via flags.
func resolveNewArgs(
	stdout io.Writer,
	skipPrompts bool,
	name, url, format string,
	filters, groups []string,
	opts display.Options,
) (stackWebhookNewArgs, error) {
	var err error

	// Name is required.
	if name == "" {
		nameErrMsg := "a webhook name is required (use --name)"
		if !skipPrompts {
			nameErrMsg = "a webhook name is required"
		}
		name, err = ui.PromptForValue(
			skipPrompts, "Webhook name", "", false,
			func(v string) error {
				if v == "" {
					return errors.New(nameErrMsg) //nolint:goerr113 // dynamic but intentional
				}
				return nil
			}, opts)
		if err != nil {
			return stackWebhookNewArgs{}, err
		}
	}

	// URL is required.
	if url == "" {
		if !skipPrompts {
			fmt.Fprintln(stdout, "Enter the URL that will receive webhook payloads "+
				"(e.g. https://example.com/webhook).")
		}
		urlErrMsg := "a payload URL is required (use --url)"
		if !skipPrompts {
			urlErrMsg = "a payload URL is required"
		}
		url, err = ui.PromptForValue(
			skipPrompts, "Webhook payload URL", "", false,
			func(v string) error {
				if v == "" {
					return errors.New(urlErrMsg) //nolint:goerr113 // dynamic but intentional
				}
				return nil
			}, opts)
		if err != nil {
			return stackWebhookNewArgs{}, err
		}
	}

	// Format: prompt with a picker when interactive.
	if !skipPrompts {
		format = ui.PromptUserSkippable(
			false, "Webhook format", webhookFormats, format,
			opts.Color)
	}

	// Event groups: prompt with a multi-select when interactive.
	if len(groups) == 0 && !skipPrompts {
		groups = ui.PromptUserMultiSkippable(
			false, "Event groups to subscribe to",
			stackWebhookGroups, stackWebhookGroups,
			opts.Color)
	}

	// Event filters: only show events not already covered by selected groups.
	if len(filters) == 0 && !skipPrompts {
		remaining := filtersNotCoveredByGroups(groups)
		if len(remaining) > 0 {
			filters = ui.PromptUserMultiSkippable(
				false,
				"Additional events to subscribe to",
				remaining, nil,
				opts.Color)
		}
	}

	if err := validateGroupsAndEvents(groups, filters); err != nil {
		return stackWebhookNewArgs{}, err
	}

	return stackWebhookNewArgs{
		Name:    name,
		URL:     url,
		Format:  format,
		Filters: filters,
		Groups:  groups,
	}, nil
}

func runStackWebhookNew(
	ctx context.Context,
	w io.Writer,
	factory stackWebhookNewClientFactory,
	stackFlag string,
	args stackWebhookNewArgs,
	render webhookNewRenderFunc,
) error {
	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	formatPtr := &args.Format
	if args.Format == "" {
		formatPtr = nil
	}

	project := stackID.Project
	stack := stackID.Stack.String()
	req := apitype.Webhook{
		OrganizationName: stackID.Owner,
		ProjectName:      &project,
		StackName:        &stack,
		DisplayName:      args.Name,
		PayloadURL:       args.URL,
		Active:           args.Active,
		Format:           formatPtr,
		Filters:          args.Filters,
		Groups:           args.Groups,
	}
	if args.Secret != "" {
		req.Secret = args.Secret
	}

	created, err := c.CreateStackWebhook(ctx, stackID, req)
	if err != nil {
		return fmt.Errorf("creating stack webhook: %w", err)
	}

	return render(w, created)
}

type webhookNewRenderFunc func(w io.Writer, wh apitype.Webhook) error
