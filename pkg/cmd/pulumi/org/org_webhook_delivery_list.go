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
	"strconv"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
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

type orgWebhookDeliveryListRender func(
	cmd *orgWebhookDeliveryListCmd, deliveries []apitype.WebhookDelivery,
) error

type orgWebhookDeliveryListCmd struct {
	orgName string
	output  outputflag.OutputFlag[orgWebhookDeliveryListRender]
	w       io.Writer

	ws             pkgWorkspace.Context
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
		*workspace.Project, display.Options,
	) (backend.Backend, error)
}

func newOrgWebhookDeliveryListCmd() *cobra.Command {
	dlcmd := &orgWebhookDeliveryListCmd{
		output: outputflag.OutputFlag[orgWebhookDeliveryListRender]{
			RenderForTerminal: (*orgWebhookDeliveryListCmd).renderTable,
			RenderJSON:        (*orgWebhookDeliveryListCmd).renderJSON,
		},
		ws:             pkgWorkspace.Instance,
		currentBackend: cmdBackend.CurrentBackend,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[EXPERIMENTAL] List recent deliveries for an organization webhook",
		Long: "List recent deliveries for an organization webhook.\n" +
			"\n" +
			"Returns the recent delivery history for a specific webhook. Each\n" +
			"delivery includes the timestamp, event kind, HTTP response code,\n" +
			"and request duration.",
		Example: "  # List deliveries for a webhook\n" +
			"  pulumi org webhook delivery list my-webhook\n\n" +
			"  # List deliveries as JSON\n" +
			"  pulumi org webhook delivery list my-webhook --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			dlcmd.w = cmd.OutOrStdout()
			return dlcmd.run(cmd.Context(), args[0])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "webhook"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&dlcmd.orgName, "org", "",
		"The organization that owns the webhook. Defaults to the current org.")
	outputflag.VarP(cmd.Flags(), &dlcmd.output)

	return cmd
}

func (c *orgWebhookDeliveryListCmd) run(ctx context.Context, webhookName string) error {
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

	// Verify the webhook exists — the deliveries endpoint returns empty
	// for non-existent webhooks instead of 404.
	if _, err := cl.GetOrgWebhook(ctx, orgName, webhookName); err != nil {
		return fmt.Errorf("webhook %q not found: %w", webhookName, err)
	}

	deliveries, err := cl.ListOrgWebhookDeliveries(ctx, orgName, webhookName)
	if err != nil {
		return fmt.Errorf("listing webhook deliveries: %w", err)
	}

	return c.output.Get()(c, deliveries)
}

func (c *orgWebhookDeliveryListCmd) renderJSON(deliveries []apitype.WebhookDelivery) error {
	if deliveries == nil {
		deliveries = []apitype.WebhookDelivery{}
	}
	enc := json.NewEncoder(c.w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Deliveries []apitype.WebhookDelivery `json:"deliveries"`
		Count      int                       `json:"count"`
	}{
		Deliveries: deliveries,
		Count:      len(deliveries),
	})
}

func (c *orgWebhookDeliveryListCmd) renderTable(deliveries []apitype.WebhookDelivery) error {
	if len(deliveries) == 0 {
		fmt.Fprintln(c.w, "No deliveries found for this webhook.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(c.w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"ID", "KIND", "TIMESTAMP", "DURATION", "STATUS"})

	for _, d := range deliveries {
		ts := time.Unix(d.Timestamp, 0).UTC().Format(time.RFC3339)
		duration := strconv.Itoa(d.Duration) + "ms"
		t.AppendRow(table.Row{d.ID, d.Kind, ts, duration, d.ResponseCode})
	}

	t.SetAllowedRowLength(cmdCmd.StdoutWidth())
	t.Render()

	fmt.Fprintf(c.w, "\n%d delivery(ies)\n", len(deliveries))
	return nil
}
