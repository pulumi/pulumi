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
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
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

func newStackWebhookDeliveryListCmd() *cobra.Command {
	return newStackWebhookDeliveryListCmdWith(nil)
}

func newStackWebhookDeliveryListCmdWith(
	factory stackWebhookDeliveryListClientFactory,
) *cobra.Command {
	var (
		stack  string
		output string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[EXPERIMENTAL] List recent deliveries for a stack webhook",
		Long: "List recent deliveries for a stack webhook.\n" +
			"\n" +
			"Returns the recent delivery history for a specific webhook. Each\n" +
			"delivery includes the timestamp, event kind, HTTP response code,\n" +
			"and request duration.\n" +
			"\n" +
			"Returns an error if the webhook does not exist.",
		Example: "  # List deliveries for a webhook\n" +
			"  pulumi stack webhook delivery list my-webhook\n\n" +
			"  # List deliveries as JSON\n" +
			"  pulumi stack webhook delivery list my-webhook --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultDeliveryListClientFactory
			}
			return runDeliveryList(
				cmd.Context(), cmd.OutOrStdout(), factory,
				stack, args[0], output,
			)
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"The output format: default (human-readable table) or json")

	return cmd
}

func defaultDeliveryListClientFactory(
	ctx context.Context, stackFlag string,
) (stackWebhookDeliveryListClient, client.StackIdentifier, error) {
	return RequireCloudStack(
		ctx, cmdutil.Diag(), pkgWorkspace.Instance,
		cmdBackend.DefaultLoginManager, stackFlag)
}

func runDeliveryList(
	ctx context.Context,
	w io.Writer,
	factory stackWebhookDeliveryListClientFactory,
	stackFlag string,
	webhookName string,
	output string,
) error {
	renderer, err := deliveryListRenderer(output)
	if err != nil {
		return err
	}

	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	// Verify the webhook exists first — the deliveries endpoint returns
	// an empty list for non-existent webhooks instead of 404.
	if _, err := c.GetStackWebhook(ctx, stackID, webhookName); err != nil {
		return fmt.Errorf("webhook %q not found: %w", webhookName, err)
	}

	deliveries, err := c.ListStackWebhookDeliveries(ctx, stackID, webhookName)
	if err != nil {
		return fmt.Errorf("listing webhook deliveries: %w", err)
	}

	return renderer(w, deliveries)
}

type deliveryListRenderFunc func(w io.Writer, deliveries []apitype.WebhookDelivery) error

func deliveryListRenderer(output string) (deliveryListRenderFunc, error) {
	switch output {
	case "", "default", "table":
		return renderDeliveryListTable, nil
	case "json":
		return renderDeliveryListJSON, nil
	default:
		return nil, fmt.Errorf(
			"invalid --output value %q: expected \"default\", \"table\", or \"json\"", output)
	}
}

// deliveryListEnvelope is the JSON shape for delivery list output.
type deliveryListEnvelope struct {
	Deliveries []apitype.WebhookDelivery `json:"deliveries"`
	Count      int                       `json:"count"`
}

func renderDeliveryListJSON(w io.Writer, deliveries []apitype.WebhookDelivery) error {
	if deliveries == nil {
		deliveries = []apitype.WebhookDelivery{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(deliveryListEnvelope{
		Deliveries: deliveries,
		Count:      len(deliveries),
	})
}

func renderDeliveryListTable(w io.Writer, deliveries []apitype.WebhookDelivery) error {
	if len(deliveries) == 0 {
		fmt.Fprintln(w, "No deliveries found for this webhook.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"ID", "KIND", "TIMESTAMP", "DURATION", "STATUS"})

	for _, d := range deliveries {
		ts := time.Unix(d.Timestamp, 0).UTC().Format(time.RFC3339)
		duration := strconv.Itoa(d.Duration) + "ms"
		t.AppendRow(table.Row{d.ID, d.Kind, ts, duration, d.ResponseCode})
	}

	cols := cmdCmd.StdoutWidth()
	idWidth := cols - 80 // leave room for other columns
	if idWidth < 10 {
		idWidth = 10
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "ID", WidthMax: idWidth},
	})
	t.Render()

	fmt.Fprintf(w, "\n%d delivery(ies)\n", len(deliveries))
	return nil
}
