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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackWebhookListClient is the interface the list command needs from the API client.
type stackWebhookListClient interface {
	ListStackWebhooks(ctx context.Context, stackID client.StackIdentifier) ([]apitype.Webhook, error)
}

// stackWebhookListClientFactory builds a stackWebhookListClient from the environment.
// It returns the client, the resolved StackIdentifier, and any error.
type stackWebhookListClientFactory func(
	ctx context.Context, stackFlag string,
) (stackWebhookListClient, client.StackIdentifier, error)

func newStackWebhookListCmd() *cobra.Command {
	return newStackWebhookListCmdWith(nil)
}

func newStackWebhookListCmdWith(factory stackWebhookListClientFactory) *cobra.Command {
	var (
		stack  string
		output string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all webhooks configured for a stack",
		Long: "List all webhooks configured for a stack.\n" +
			"\n" +
			"Displays the ID, name, payload URL, format, event groups, events, and active\n" +
			"status of each webhook. By default the output is a human-readable table;\n" +
			"pass --output=json for a stable, machine-readable JSON envelope.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackWebhookListClientFactory
			}
			return runStackWebhookList(cmd.Context(), cmd.OutOrStdout(), factory, stack, output)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"The output format: default (human-readable table) or json")

	return cmd
}

// defaultStackWebhookListClientFactory resolves the current Pulumi Cloud context and
// returns a client and stack identifier.
func defaultStackWebhookListClientFactory(
	ctx context.Context, stackFlag string,
) (stackWebhookListClient, client.StackIdentifier, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, LoadOnly, opts)
	if err != nil {
		return nil, client.StackIdentifier{}, fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{}, fmt.Errorf("stack webhooks require the Pulumi Cloud backend; run `pulumi login`")
	}

	ref := cloudStack.Ref()
	project := ""
	if p, ok := ref.Project(); ok {
		project = string(p)
	}
	stackID := client.StackIdentifier{
		Owner:   cloudStack.OrgName(),
		Project: project,
		Stack:   ref.Name(),
	}

	be := cloudStack.Backend().(httpstate.Backend)
	return be.Client(), stackID, nil
}

func runStackWebhookList(
	ctx context.Context,
	w io.Writer,
	factory stackWebhookListClientFactory,
	stackFlag string,
	output string,
) error {
	renderer, err := webhookListRenderer(output)
	if err != nil {
		return err
	}

	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	webhooks, err := c.ListStackWebhooks(ctx, stackID)
	if err != nil {
		return fmt.Errorf("listing stack webhooks: %w", err)
	}

	return renderer(w, webhooks)
}

type webhookListRenderFunc func(w io.Writer, webhooks []apitype.Webhook) error

func webhookListRenderer(output string) (webhookListRenderFunc, error) {
	switch output {
	case "", "default", "table":
		return renderWebhookListTable, nil
	case "json":
		return renderWebhookListJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q: expected \"default\", \"table\", or \"json\"", output)
	}
}

// webhookListEnvelope is the JSON shape emitted by `pulumi stack webhook list --output=json`.
type webhookListEnvelope struct {
	Webhooks []webhookJSON `json:"webhooks"`
	Count    int           `json:"count"`
}

type webhookJSON struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Format   string   `json:"format"`
	Active   bool     `json:"active"`
	Groups   []string `json:"eventGroups"`
	Filters  []string `json:"events"`
}

func toWebhookJSON(wh apitype.Webhook) webhookJSON {
	format := ""
	if wh.Format != nil {
		format = *wh.Format
	}
	groups := wh.Groups
	if groups == nil {
		groups = []string{}
	}
	filters := wh.Filters
	if filters == nil {
		filters = []string{}
	}
	return webhookJSON{
		ID:       wh.Name,
		Name:     wh.DisplayName,
		URL:      wh.PayloadURL,
		Format:   format,
		Active:   wh.Active,
		Groups:   groups,
		Filters:  filters,
	}
}

func renderWebhookListJSON(w io.Writer, webhooks []apitype.Webhook) error {
	items := make([]webhookJSON, 0, len(webhooks))
	for _, wh := range webhooks {
		items = append(items, toWebhookJSON(wh))
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(webhookListEnvelope{
		Webhooks: items,
		Count:    len(items),
	})
}

const webhookTableFallbackCols = 120

// webhookTableRow holds the formatted string values for a single table row.
type webhookTableRow struct {
	id, name, url, format, groups, events, active string
}

func buildTableRows(webhooks []apitype.Webhook) []webhookTableRow {
	rows := make([]webhookTableRow, len(webhooks))
	for i, wh := range webhooks {
		format := ""
		if wh.Format != nil {
			format = *wh.Format
		}
		groups := ""
		if len(wh.Groups) > 0 {
			groups = strings.Join(wh.Groups, ", ")
		}
		events := ""
		if len(wh.Filters) > 0 {
			events = strings.Join(wh.Filters, ", ")
		}
		active := "yes"
		if !wh.Active {
			active = "no"
		}
		rows[i] = webhookTableRow{
			id:     wh.Name,
			name:   wh.DisplayName,
			url:    wh.PayloadURL,
			format: format,
			groups: groups,
			events: events,
			active: active,
		}
	}
	return rows
}

func renderWebhookListTable(w io.Writer, webhooks []apitype.Webhook) error {
	if len(webhooks) == 0 {
		fmt.Fprintln(w, "No webhooks configured for this stack.")
		return nil
	}

	rows := buildTableRows(webhooks)

	// Detect which optional columns have at least one non-empty value.
	hasName, hasFormat, hasGroups, hasEvents := false, false, false, false
	for _, r := range rows {
		hasName = hasName || r.name != ""
		hasFormat = hasFormat || r.format != ""
		hasGroups = hasGroups || r.groups != ""
		hasEvents = hasEvents || r.events != ""
	}

	// Build header and rows dynamically, skipping empty columns.
	header := table.Row{"ID"}
	if hasName {
		header = append(header, "NAME")
	}
	header = append(header, "URL")
	if hasFormat {
		header = append(header, "FORMAT")
	}
	if hasGroups {
		header = append(header, "EVENT GROUPS")
	}
	if hasEvents {
		header = append(header, "EVENTS")
	}
	header = append(header, "ACTIVE")

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(header)

	for _, r := range rows {
		row := table.Row{r.id}
		if hasName {
			row = append(row, r.name)
		}
		row = append(row, r.url)
		if hasFormat {
			row = append(row, r.format)
		}
		if hasGroups {
			row = append(row, r.groups)
		}
		if hasEvents {
			row = append(row, r.events)
		}
		row = append(row, r.active)
		t.AppendRow(row)
	}

	// Let URL absorb remaining width.
	cols := termWidth(webhookTableFallbackCols)
	// 3 chars per column separator + 1 outer border each side = 3*ncols + 1.
	borderWidth := 3*len(header) + 1
	fixedColsWidth := 40 // rough room for the non-URL columns
	urlWidth := cols - borderWidth - fixedColsWidth
	if urlWidth < 20 {
		urlWidth = 20
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "URL", WidthMax: urlWidth, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()

	fmt.Fprintf(w, "\n%d webhook(s)\n", len(webhooks))
	return nil
}

func termWidth(fallback int) int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return fallback
	}
	return w
}
