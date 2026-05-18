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

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// orgWebhookListClient is the interface the list command needs.
type orgWebhookListClient interface {
	ListOrgWebhooks(ctx context.Context, org string) ([]apitype.Webhook, error)
}

type orgWebhookListRender func(
	cmd *orgWebhookListCmd, webhooks []apitype.Webhook,
) error

type orgWebhookListCmd struct {
	orgName string
	count   int
	all     bool
	output  outputflag.OutputFlag[orgWebhookListRender]
	w       io.Writer

	ws             pkgWorkspace.Context
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
		*workspace.Project, display.Options,
	) (backend.Backend, error)
}

func newOrgWebhookListCmd() *cobra.Command {
	return newOrgWebhookListCmdWith(pkgWorkspace.Instance, cmdBackend.CurrentBackend)
}

func newOrgWebhookListCmdWith(
	ws pkgWorkspace.Context,
	overrideBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
		*workspace.Project, display.Options,
	) (backend.Backend, error),
) *cobra.Command {
	olcmd := &orgWebhookListCmd{
		output: outputflag.OutputFlag[orgWebhookListRender]{
			RenderForTerminal: (*orgWebhookListCmd).renderTable,
			RenderJSON:        (*orgWebhookListCmd).renderJSON,
		},
		ws:             ws,
		currentBackend: overrideBackend,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[EXPERIMENTAL] List all webhooks configured for an organization",
		Long: "[EXPERIMENTAL] List all webhooks configured for an organization.\n" +
			"\n" +
			"Returns all webhooks configured at the organization level. Each\n" +
			"webhook includes its ID, name, payload URL, format, event groups,\n" +
			"events, and active status.\n" +
			"\n" +
			"Organization-level webhooks can fire on stack lifecycle events,\n" +
			"deployment events, drift detection events, and policy violation events.",
		Example: "  # List webhooks for the default organization\n" +
			"  pulumi org webhook list\n\n" +
			"  # List webhooks for a specific organization\n" +
			"  pulumi org webhook list --org my-org\n\n" +
			"  # List webhooks as JSON\n" +
			"  pulumi org webhook list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if olcmd.all && olcmd.count > 0 {
				return errors.New("--all and --count are mutually exclusive")
			}
			olcmd.w = cmd.OutOrStdout()
			return olcmd.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&olcmd.orgName, "org", "",
		"The organization to list webhooks for. Defaults to the current org.")
	cmd.Flags().IntVar(&olcmd.count, "count", 0,
		"Number of results to display")
	cmd.Flags().BoolVar(&olcmd.all, "all", false,
		"Show all results (mutually exclusive with --count)")
	outputflag.Var(cmd.Flags(), &olcmd.output)

	return cmd
}

func (c *orgWebhookListCmd) run(ctx context.Context) error {
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

	webhooks, err := cloudBackend.Client().ListOrgWebhooks(ctx, orgName)
	if err != nil {
		return fmt.Errorf("listing organization webhooks: %w", err)
	}

	if c.count > 0 && c.count < len(webhooks) {
		webhooks = webhooks[:c.count]
	}

	return c.output.Get()(c, webhooks)
}

// webhookJSON is the per-item shape for JSON output.
type orgWebhookJSON struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Format  string   `json:"format"`
	Active  bool     `json:"active"`
	Groups  []string `json:"eventGroups"`
	Filters []string `json:"events"`
}

func toOrgWebhookJSON(wh apitype.Webhook) orgWebhookJSON {
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
	return orgWebhookJSON{
		ID:      wh.Name,
		Name:    wh.DisplayName,
		URL:     wh.PayloadURL,
		Format:  format,
		Active:  wh.Active,
		Groups:  groups,
		Filters: filters,
	}
}

func (c *orgWebhookListCmd) renderJSON(webhooks []apitype.Webhook) error {
	items := make([]orgWebhookJSON, 0, len(webhooks))
	for _, wh := range webhooks {
		items = append(items, toOrgWebhookJSON(wh))
	}
	enc := json.NewEncoder(c.w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Webhooks []orgWebhookJSON `json:"webhooks"`
		Count    int              `json:"count"`
	}{
		Webhooks: items,
		Count:    len(items),
	})
}

type orgWebhookTableRow struct {
	id, name, url, format, groups, events, active string
}

func (c *orgWebhookListCmd) renderTable(webhooks []apitype.Webhook) error {
	if len(webhooks) == 0 {
		fmt.Fprintln(c.w, "No webhooks configured for this organization.")
		return nil
	}

	rows := make([]orgWebhookTableRow, len(webhooks))
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
		rows[i] = orgWebhookTableRow{
			id:     wh.Name,
			name:   wh.DisplayName,
			url:    wh.PayloadURL,
			format: format,
			groups: groups,
			events: events,
			active: active,
		}
	}

	// Detect which optional columns have data.
	hasName, hasFormat, hasGroups, hasEvents := false, false, false, false
	for _, r := range rows {
		hasName = hasName || r.name != ""
		hasFormat = hasFormat || r.format != ""
		hasGroups = hasGroups || r.groups != ""
		hasEvents = hasEvents || r.events != ""
	}

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
	t.SetOutputMirror(c.w)
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

	// Wrap flexible columns to fit the terminal.
	cols := cmdCmd.StdoutWidth()
	borderWidth := 3*len(header) + 1
	fixedWidth := borderWidth + 12 + 6 // ID + ACTIVE
	if hasName {
		fixedWidth += 12
	}
	if hasFormat {
		fixedWidth += 8
	}
	flexCols := 1 // URL always present
	if hasGroups {
		flexCols++
	}
	if hasEvents {
		flexCols++
	}
	remaining := cols - fixedWidth
	if remaining < 20*flexCols {
		remaining = 20 * flexCols
	}
	flexWidth := remaining / flexCols
	colConfigs := []table.ColumnConfig{
		{Name: "URL", WidthMax: flexWidth, WidthMaxEnforcer: text.WrapText},
	}
	if hasGroups {
		colConfigs = append(colConfigs,
			table.ColumnConfig{Name: "EVENT GROUPS", WidthMax: flexWidth, WidthMaxEnforcer: text.WrapText})
	}
	if hasEvents {
		colConfigs = append(colConfigs,
			table.ColumnConfig{Name: "EVENTS", WidthMax: flexWidth, WidthMaxEnforcer: text.WrapText})
	}
	t.SetColumnConfigs(colConfigs)
	t.Render()

	fmt.Fprintf(c.w, "\n%d webhook(s)\n", len(webhooks))
	return nil
}
