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
	"strconv"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackWebhookDeliveryListClient is the interface the command needs.
type stackWebhookDeliveryListClient interface {
	GetStackWebhook(
		ctx context.Context, stackID client.StackIdentifier, webhookName string,
	) (apitype.Webhook, error)
	ListStackWebhookDeliveries(
		ctx context.Context, stackID client.StackIdentifier, webhookName string,
	) ([]apitype.WebhookDelivery, error)
}

type stackWebhookDeliveryListClientFactory func(
	ctx context.Context, stackFlag string,
) (stackWebhookDeliveryListClient, client.StackIdentifier, error)

type deliveryListRender func(
	cmd *deliveryListCmd, deliveries []apitype.WebhookDelivery,
) error

type deliveryListCmd struct {
	stack string
	count int
	w     io.Writer

	output  outputflag.OutputFlag[deliveryListRender]
	factory stackWebhookDeliveryListClientFactory
}

func newStackWebhookDeliveryListCmd() *cobra.Command {
	return newStackWebhookDeliveryListCmdWith(nil)
}

func newStackWebhookDeliveryListCmdWith(
	factory stackWebhookDeliveryListClientFactory,
) *cobra.Command {
	dlcmd := &deliveryListCmd{
		output: outputflag.OutputFlag[deliveryListRender]{
			RenderForTerminal: (*deliveryListCmd).renderTable,
			RenderJSON:        (*deliveryListCmd).renderJSON,
		},
		factory: factory,
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "[EXPERIMENTAL] List recent deliveries for a stack webhook",
		Long: "[EXPERIMENTAL] List recent deliveries for a stack webhook.\n" +
			"\n" +
			"Returns the recent delivery history for a specific webhook. Each\n" +
			"delivery includes the timestamp, event kind, HTTP response code,\n" +
			"and request duration.\n" +
			"\n" +
			"Returns an error if the webhook does not exist.",
		Example: "  # List deliveries for a webhook\n" +
			"  pulumi stack webhook delivery list 1a2b3c4d\n\n" +
			"  # List the last 5 deliveries\n" +
			"  pulumi stack webhook delivery list 1a2b3c4d --count 5\n\n" +
			"  # List deliveries as JSON\n" +
			"  pulumi stack webhook delivery list 1a2b3c4d --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dlcmd.factory == nil {
				dlcmd.factory = defaultDeliveryListClientFactory
			}
			dlcmd.w = cmd.OutOrStdout()
			return dlcmd.run(cmd.Context(), args[0])
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&dlcmd.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().IntVar(&dlcmd.count, "count", 0,
		"Number of deliveries to display (default: all)")
	outputflag.VarP(cmd.Flags(), &dlcmd.output)

	return cmd
}

func defaultDeliveryListClientFactory(
	ctx context.Context, stackFlag string,
) (stackWebhookDeliveryListClient, client.StackIdentifier, error) {
	return RequireCloudStack(
		ctx, cmdutil.Diag(), pkgWorkspace.Instance,
		cmdBackend.DefaultLoginManager, stackFlag)
}

func (c *deliveryListCmd) run(ctx context.Context, webhookName string) error {
	cl, stackID, err := c.factory(ctx, c.stack)
	if err != nil {
		return err
	}

	// Verify the webhook exists first — the deliveries endpoint returns
	// an empty list for non-existent webhooks instead of 404.
	if _, err := cl.GetStackWebhook(ctx, stackID, webhookName); err != nil {
		return fmt.Errorf("webhook %q not found: %w", webhookName, err)
	}

	deliveries, err := cl.ListStackWebhookDeliveries(ctx, stackID, webhookName)
	if err != nil {
		return fmt.Errorf("listing webhook deliveries: %w", err)
	}

	if c.count > 0 && c.count < len(deliveries) {
		deliveries = deliveries[:c.count]
	}

	return c.output.Get()(c, deliveries)
}

func (c *deliveryListCmd) renderJSON(deliveries []apitype.WebhookDelivery) error {
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

func (c *deliveryListCmd) renderTable(deliveries []apitype.WebhookDelivery) error {
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

	t.Render()

	fmt.Fprintf(c.w, "\n%d delivery(ies)\n", len(deliveries))
	return nil
}
