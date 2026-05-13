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

package env

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// envWebhookClient is the subset of cloud-API operations the env webhook
// commands need. Defined inside this package so unit tests can stub it without
// touching the full HTTP client surface.
type envWebhookClient interface {
	ListEnvironmentWebhooks(
		ctx context.Context, org, project, env string,
	) ([]apitype.EnvironmentWebhook, error)
	GetEnvironmentWebhook(
		ctx context.Context, org, project, env, name string,
	) (apitype.EnvironmentWebhook, error)
	CreateEnvironmentWebhook(
		ctx context.Context, org, project, env string, req apitype.CreateEnvironmentWebhookRequest,
	) (apitype.EnvironmentWebhook, error)
	UpdateEnvironmentWebhook(
		ctx context.Context, org, project, env, name string, req apitype.UpdateEnvironmentWebhookRequest,
	) (apitype.EnvironmentWebhook, error)
	DeleteEnvironmentWebhook(
		ctx context.Context, org, project, env, name string,
	) error
	PingEnvironmentWebhook(
		ctx context.Context, org, project, env, name string,
	) (apitype.EnvironmentWebhookDelivery, error)
	ListEnvironmentWebhookDeliveries(
		ctx context.Context, org, project, env, name string,
	) ([]apitype.EnvironmentWebhookDelivery, error)
}

// envWebhookFactory resolves a cloud client and the effective org for the
// command. orgOverride wins when non-empty; otherwise the default org from the
// cloud context is used.
type envWebhookFactory func(ctx context.Context, orgOverride string) (envWebhookClient, string, error)

// defaultEnvWebhookFactory is the production wiring for envWebhookFactory. It
// resolves the cloud context via cloud.ResolveContext and surfaces the
// *client.Client directly — *client.Client already satisfies envWebhookClient.
func defaultEnvWebhookFactory(
	ctx context.Context, orgOverride string,
) (envWebhookClient, string, error) {
	resolved, err := cloud.ResolveContext(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("resolving cloud context: %w", err)
	}
	if !resolved.LoggedIn {
		return nil, "", errors.New("not logged in to Pulumi Cloud; run `pulumi login` first")
	}
	org := orgOverride
	if org == "" {
		org = resolved.OrgName
	}
	if org == "" {
		return nil, "", errors.New(
			"no organization available; pass --org or set a default with `pulumi org set-default`")
	}
	return resolved.Client, org, nil
}

func newEnvWebhookCmd() *cobra.Command {
	return newEnvWebhookCmdWith(nil)
}

func newEnvWebhookCmdWith(factory envWebhookFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage environment webhooks",
		Long:  "[EXPERIMENTAL] Manage Pulumi ESC environment webhooks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newEnvWebhookListCmd(factory))
	cmd.AddCommand(newEnvWebhookNewCmd(factory))
	cmd.AddCommand(newEnvWebhookEditCmd(factory))
	cmd.AddCommand(newEnvWebhookRemoveCmd(factory))
	cmd.AddCommand(newEnvWebhookPingCmd(factory))
	cmd.AddCommand(newEnvWebhookDeliveryCmd(factory))

	return cmd
}

func envWebhookEnvArg() *constrictor.Arguments {
	return &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "project"},
			{Name: "name"},
		},
		Required: 2,
	}
}

func envWebhookEnvWithHookArg() *constrictor.Arguments {
	return &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "project"},
			{Name: "name"},
			{Name: "webhook"},
		},
		Required: 3,
	}
}

// --- list --------------------------------------------------------------------

func newEnvWebhookListCmd(factory envWebhookFactory) *cobra.Command {
	var (
		org    string
		output string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all webhooks configured for an environment",
		Long:  "[EXPERIMENTAL] List all webhooks configured for an environment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := factory
			if f == nil {
				f = defaultEnvWebhookFactory
			}
			render, err := envWebhookListRenderer(output)
			if err != nil {
				return err
			}
			c, resolvedOrg, err := f(cmd.Context(), org)
			if err != nil {
				return err
			}
			hooks, err := c.ListEnvironmentWebhooks(cmd.Context(), resolvedOrg, args[0], args[1])
			if err != nil {
				return fmt.Errorf("listing environment webhooks: %w", err)
			}
			return render(cmd.OutOrStdout(), hooks)
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"Output format. One of: default, json")

	return cmd
}

// --- new ---------------------------------------------------------------------

func newEnvWebhookNewCmd(factory envWebhookFactory) *cobra.Command {
	var (
		org         string
		payloadURL  string
		displayName string
		format      string
		filters     []string
		active      bool
		secret      string
		output      string
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new environment webhook",
		Long:  "[EXPERIMENTAL] Create a new environment webhook.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := factory
			if f == nil {
				f = defaultEnvWebhookFactory
			}
			renderOne, err := envWebhookRenderer(output)
			if err != nil {
				return err
			}

			project, envName, hookName := args[0], args[1], args[2]
			displayed := displayName
			if displayed == "" {
				displayed = hookName
			}

			req := apitype.CreateEnvironmentWebhookRequest{
				Name:        hookName,
				DisplayName: displayed,
				PayloadURL:  payloadURL,
				Active:      active,
				Format:      format,
				Filters:     filters,
				Secret:      secret,
			}

			c, resolvedOrg, err := f(cmd.Context(), org)
			if err != nil {
				return err
			}
			hook, err := c.CreateEnvironmentWebhook(cmd.Context(), resolvedOrg, project, envName, req)
			if err != nil {
				return fmt.Errorf("creating environment webhook: %w", err)
			}
			return renderOne(cmd.OutOrStdout(), hook)
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVar(&payloadURL, "url", "", "The webhook payload URL")
	cmd.Flags().StringVar(&displayName, "display-name", "",
		"The human-readable display name for the webhook (defaults to the webhook name)")
	cmd.Flags().StringVar(&format, "format", "raw",
		"The webhook format: raw, slack, ms_teams, or pulumi_deployments")
	cmd.Flags().StringArrayVar(&filters, "filter", nil,
		"An event type to subscribe to (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "", "The HMAC key for signature verification")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"Output format. One of: default, json")
	_ = cmd.MarkFlagRequired("url")

	return cmd
}

// --- edit --------------------------------------------------------------------

func newEnvWebhookEditCmd(factory envWebhookFactory) *cobra.Command {
	var (
		org         string
		payloadURL  string
		displayName string
		format      string
		filters     []string
		addFilters  []string
		rmFilters   []string
		active      bool
		secret      string
		output      string
	)

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Update an environment webhook's configuration",
		Long: "[EXPERIMENTAL] Update an environment webhook's configuration.\n" +
			"\n" +
			"Only flags explicitly provided on the command line are sent in the PATCH.\n" +
			"Use --filter to replace the filter list outright, or --add-filter /\n" +
			"--remove-filter to merge against the current webhook configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := factory
			if f == nil {
				f = defaultEnvWebhookFactory
			}
			renderOne, err := envWebhookRenderer(output)
			if err != nil {
				return err
			}

			project, envName, hookName := args[0], args[1], args[2]

			if cmd.Flags().Changed("filter") &&
				(cmd.Flags().Changed("add-filter") || cmd.Flags().Changed("remove-filter")) {
				return errors.New(
					"--filter cannot be combined with --add-filter or --remove-filter")
			}

			req := buildEnvWebhookPatch(cmd, payloadURL, displayName, format, secret, active, filters)

			c, resolvedOrg, err := f(cmd.Context(), org)
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("add-filter") || cmd.Flags().Changed("remove-filter") {
				current, gerr := c.GetEnvironmentWebhook(cmd.Context(), resolvedOrg, project, envName, hookName)
				if gerr != nil {
					return fmt.Errorf("reading current webhook: %w", gerr)
				}
				merged := mergeFilters(current.Filters, addFilters, rmFilters)
				req.Filters = &merged
			}

			hook, err := c.UpdateEnvironmentWebhook(cmd.Context(), resolvedOrg, project, envName, hookName, req)
			if err != nil {
				return fmt.Errorf("updating environment webhook: %w", err)
			}
			return renderOne(cmd.OutOrStdout(), hook)
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVar(&payloadURL, "url", "", "The webhook payload URL")
	cmd.Flags().StringVar(&displayName, "display-name", "",
		"The human-readable display name for the webhook")
	cmd.Flags().StringVar(&format, "format", "",
		"The webhook format: raw, slack, ms_teams, or pulumi_deployments")
	cmd.Flags().StringArrayVar(&filters, "filter", nil,
		"Replace the filter list with these event types (repeatable)")
	cmd.Flags().StringArrayVar(&addFilters, "add-filter", nil,
		"Add an event type to the existing filter list (repeatable)")
	cmd.Flags().StringArrayVar(&rmFilters, "remove-filter", nil,
		"Remove an event type from the existing filter list (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "", "The HMAC key for signature verification")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"Output format. One of: default, json")

	return cmd
}

// buildEnvWebhookPatch constructs the PATCH body from the parsed flags. Only
// flags explicitly set on the command line are included — this implements the
// parent epic's "ternary flags" rule, where the absence of a flag means "leave
// unchanged" rather than "send the default".
func buildEnvWebhookPatch(
	cmd *cobra.Command,
	payloadURL, displayName, format, secret string,
	active bool,
	filters []string,
) apitype.UpdateEnvironmentWebhookRequest {
	var req apitype.UpdateEnvironmentWebhookRequest
	if cmd.Flags().Changed("url") {
		u := payloadURL
		req.PayloadURL = &u
	}
	if cmd.Flags().Changed("display-name") {
		d := displayName
		req.DisplayName = &d
	}
	if cmd.Flags().Changed("format") {
		fv := format
		req.Format = &fv
	}
	if cmd.Flags().Changed("active") {
		a := active
		req.Active = &a
	}
	if cmd.Flags().Changed("secret") {
		s := secret
		req.Secret = &s
	}
	if cmd.Flags().Changed("filter") {
		f := append([]string{}, filters...)
		req.Filters = &f
	}
	return req
}

// mergeFilters returns a new list with adds appended (deduped, preserving order)
// and any entries in removes filtered out. The original ordering of existing
// is preserved; new entries from adds are appended in the order given.
func mergeFilters(existing, adds, removes []string) []string {
	rm := make(map[string]struct{}, len(removes))
	for _, r := range removes {
		rm[r] = struct{}{}
	}
	out := make([]string, 0, len(existing)+len(adds))
	seen := make(map[string]struct{}, len(existing)+len(adds))
	for _, e := range existing {
		if _, drop := rm[e]; drop {
			continue
		}
		if _, dup := seen[e]; dup {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	for _, a := range adds {
		if _, drop := rm[a]; drop {
			continue
		}
		if _, dup := seen[a]; dup {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}

// --- remove ------------------------------------------------------------------

func newEnvWebhookRemoveCmd(factory envWebhookFactory) *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Delete an environment webhook",
		Long:  "[EXPERIMENTAL] Delete an environment webhook.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := factory
			if f == nil {
				f = defaultEnvWebhookFactory
			}
			project, envName, hookName := args[0], args[1], args[2]
			c, resolvedOrg, err := f(cmd.Context(), org)
			if err != nil {
				return err
			}
			if err := c.DeleteEnvironmentWebhook(cmd.Context(), resolvedOrg, project, envName, hookName); err != nil {
				return fmt.Errorf("deleting environment webhook: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed webhook %q from environment %s/%s\n",
				hookName, project, envName)
			return nil
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	return cmd
}

// --- ping --------------------------------------------------------------------

func newEnvWebhookPingCmd(factory envWebhookFactory) *cobra.Command {
	var (
		org    string
		output string
	)

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Send a test ping to an environment webhook",
		Long:  "[EXPERIMENTAL] Send a test ping to an environment webhook.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := factory
			if f == nil {
				f = defaultEnvWebhookFactory
			}
			render, err := envWebhookDeliveryRenderer(output)
			if err != nil {
				return err
			}
			project, envName, hookName := args[0], args[1], args[2]
			c, resolvedOrg, err := f(cmd.Context(), org)
			if err != nil {
				return err
			}
			delivery, err := c.PingEnvironmentWebhook(cmd.Context(), resolvedOrg, project, envName, hookName)
			if err != nil {
				return fmt.Errorf("pinging environment webhook: %w", err)
			}
			return render(cmd.OutOrStdout(), delivery)
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"Output format. One of: default, json")

	return cmd
}

// --- delivery / delivery list ------------------------------------------------

func newEnvWebhookDeliveryCmd(factory envWebhookFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delivery",
		Short: "Inspect environment webhook deliveries",
		Long:  "[EXPERIMENTAL] Inspect environment webhook deliveries.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	cmd.AddCommand(newEnvWebhookDeliveryListCmd(factory))
	return cmd
}

func newEnvWebhookDeliveryListCmd(factory envWebhookFactory) *cobra.Command {
	var (
		org    string
		output string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent deliveries for an environment webhook",
		Long:  "[EXPERIMENTAL] List recent deliveries for an environment webhook.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := factory
			if f == nil {
				f = defaultEnvWebhookFactory
			}
			render, err := envWebhookDeliveryListRenderer(output)
			if err != nil {
				return err
			}
			project, envName, hookName := args[0], args[1], args[2]
			c, resolvedOrg, err := f(cmd.Context(), org)
			if err != nil {
				return err
			}
			deliveries, err := c.ListEnvironmentWebhookDeliveries(cmd.Context(), resolvedOrg, project, envName, hookName)
			if err != nil {
				return fmt.Errorf("listing environment webhook deliveries: %w", err)
			}
			return render(cmd.OutOrStdout(), deliveries)
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"Output format. One of: default, json")

	return cmd
}

// --- rendering ---------------------------------------------------------------

func envWebhookRenderer(format string) (func(io.Writer, apitype.EnvironmentWebhook) error, error) {
	switch format {
	case "", "default":
		return renderEnvWebhookText, nil
	case "json":
		return renderEnvWebhookJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

func renderEnvWebhookText(w io.Writer, h apitype.EnvironmentWebhook) error {
	fmt.Fprintf(w, "Name:         %s\n", h.Name)
	if h.DisplayName != "" && h.DisplayName != h.Name {
		fmt.Fprintf(w, "Display name: %s\n", h.DisplayName)
	}
	fmt.Fprintf(w, "URL:          %s\n", h.PayloadURL)
	if h.Format != "" {
		fmt.Fprintf(w, "Format:       %s\n", h.Format)
	}
	fmt.Fprintf(w, "Active:       %s\n", yesNo(h.Active))
	if len(h.Filters) > 0 {
		fmt.Fprintf(w, "Filters:      %s\n", strings.Join(h.Filters, ", "))
	}
	if len(h.Groups) > 0 {
		fmt.Fprintf(w, "Groups:       %s\n", strings.Join(h.Groups, ", "))
	}
	if h.HasSecret {
		fmt.Fprintln(w, "Secret:       (set)")
	}
	return nil
}

func renderEnvWebhookJSON(w io.Writer, h apitype.EnvironmentWebhook) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(h)
}

func envWebhookListRenderer(format string) (func(io.Writer, []apitype.EnvironmentWebhook) error, error) {
	switch format {
	case "", "default", "table":
		return renderEnvWebhookListTable, nil
	case "json":
		return renderEnvWebhookListJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

func renderEnvWebhookListJSON(w io.Writer, hooks []apitype.EnvironmentWebhook) error {
	if hooks == nil {
		hooks = []apitype.EnvironmentWebhook{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Webhooks []apitype.EnvironmentWebhook `json:"webhooks"`
		Count    int                          `json:"count"`
	}{
		Webhooks: hooks,
		Count:    len(hooks),
	})
}

func renderEnvWebhookListTable(w io.Writer, hooks []apitype.EnvironmentWebhook) error {
	if len(hooks) == 0 {
		fmt.Fprintln(w, "No webhooks configured for this environment.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"NAME", "DISPLAY NAME", "URL", "FORMAT", "FILTERS", "ACTIVE"})

	for _, h := range hooks {
		filters := ""
		if len(h.Filters) > 0 {
			filters = strings.Join(h.Filters, ", ")
		}
		t.AppendRow(table.Row{
			h.Name,
			h.DisplayName,
			h.PayloadURL,
			h.Format,
			filters,
			yesNo(h.Active),
		})
	}

	cols := cmdCmd.StdoutWidth()
	borderWidth := 3*6 + 1
	fixedColsWidth := 50
	urlWidth := cols - borderWidth - fixedColsWidth
	if urlWidth < 20 {
		urlWidth = 20
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "URL", WidthMax: urlWidth, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()

	fmt.Fprintf(w, "\n%d webhook(s)\n", len(hooks))
	return nil
}

func envWebhookDeliveryRenderer(format string) (func(io.Writer, apitype.EnvironmentWebhookDelivery) error, error) {
	switch format {
	case "", "default":
		return renderEnvWebhookDeliveryText, nil
	case "json":
		return renderEnvWebhookDeliveryJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

func renderEnvWebhookDeliveryText(w io.Writer, d apitype.EnvironmentWebhookDelivery) error {
	fmt.Fprintf(w, "ID:            %s\n", d.ID)
	fmt.Fprintf(w, "Kind:          %s\n", d.Kind)
	fmt.Fprintf(w, "Response code: %d\n", d.ResponseCode)
	fmt.Fprintf(w, "Duration:      %dms\n", d.Duration)
	return nil
}

func renderEnvWebhookDeliveryJSON(w io.Writer, d apitype.EnvironmentWebhookDelivery) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

func envWebhookDeliveryListRenderer(
	format string,
) (func(io.Writer, []apitype.EnvironmentWebhookDelivery) error, error) {
	switch format {
	case "", "default", "table":
		return renderEnvWebhookDeliveryListTable, nil
	case "json":
		return renderEnvWebhookDeliveryListJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

func renderEnvWebhookDeliveryListJSON(w io.Writer, deliveries []apitype.EnvironmentWebhookDelivery) error {
	if deliveries == nil {
		deliveries = []apitype.EnvironmentWebhookDelivery{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Deliveries []apitype.EnvironmentWebhookDelivery `json:"deliveries"`
		Count      int                                  `json:"count"`
	}{
		Deliveries: deliveries,
		Count:      len(deliveries),
	})
}

func renderEnvWebhookDeliveryListTable(w io.Writer, deliveries []apitype.EnvironmentWebhookDelivery) error {
	if len(deliveries) == 0 {
		fmt.Fprintln(w, "No deliveries recorded for this webhook.")
		return nil
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"ID", "KIND", "TIMESTAMP", "RESPONSE", "DURATION (MS)"})
	for _, d := range deliveries {
		t.AppendRow(table.Row{d.ID, d.Kind, d.Timestamp, d.ResponseCode, d.Duration})
	}
	t.Render()
	fmt.Fprintf(w, "\n%d delivery(ies)\n", len(deliveries))
	return nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
