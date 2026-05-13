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
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackWebhookRedeliverClient is the interface the redeliver command needs.
type stackWebhookRedeliverClient interface {
	RedeliverStackWebhookEvent(
		ctx context.Context, stackID client.StackIdentifier,
		webhookName, eventID string,
	) (apitype.WebhookDelivery, error)
}

type stackWebhookRedeliverClientFactory func(
	ctx context.Context, stackFlag string,
) (stackWebhookRedeliverClient, client.StackIdentifier, error)

func newStackWebhookDeliveryRedeliverCmd() *cobra.Command {
	return newStackWebhookDeliveryRedeliverCmdWith(nil)
}

func newStackWebhookDeliveryRedeliverCmdWith(
	factory stackWebhookRedeliverClientFactory,
) *cobra.Command {
	var (
		stack  string
		output string
	)

	cmd := &cobra.Command{
		Use:   "redeliver",
		Short: "[EXPERIMENTAL] Redeliver a specific webhook event",
		Long: "Redeliver a specific webhook event.\n" +
			"\n" +
			"Triggers the Pulumi Service to redeliver a specific event to a\n" +
			"webhook. This is useful for resending an event that the webhook\n" +
			"endpoint failed to process on the initial delivery attempt.\n" +
			"\n" +
			"Returns the delivery result with HTTP status and response details.\n" +
			"Returns an error if the webhook or event does not exist.",
		Example: "  # Redeliver an event\n" +
			"  pulumi stack webhook delivery redeliver my-webhook evt-abc123\n\n" +
			"  # Redeliver and get the full result as JSON\n" +
			"  pulumi stack webhook delivery redeliver my-webhook evt-abc123 --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultRedeliverClientFactory
			}
			return runRedeliver(
				cmd.Context(), cmd.OutOrStdout(), factory,
				stack, args[0], args[1], output,
			)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "webhook"},
			{Name: "event-id"},
		},
		Required: 2,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"The output format: default (human-readable text) or json")

	return cmd
}

func defaultRedeliverClientFactory(
	ctx context.Context, stackFlag string,
) (stackWebhookRedeliverClient, client.StackIdentifier, error) {
	return RequireCloudStack(
		ctx, cmdutil.Diag(), pkgWorkspace.Instance,
		cmdBackend.DefaultLoginManager, stackFlag)
}

func runRedeliver(
	ctx context.Context,
	w io.Writer,
	factory stackWebhookRedeliverClientFactory,
	stackFlag string,
	webhookName, eventID string,
	output string,
) error {
	renderer, err := redeliverRenderer(output)
	if err != nil {
		return err
	}

	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	delivery, err := c.RedeliverStackWebhookEvent(ctx, stackID, webhookName, eventID)
	if err != nil {
		return fmt.Errorf("redelivering webhook event: %w", err)
	}

	return renderer(w, delivery)
}

type redeliverRenderFunc func(w io.Writer, d apitype.WebhookDelivery) error

func redeliverRenderer(output string) (redeliverRenderFunc, error) {
	switch output {
	case "", "default":
		return renderRedeliverText, nil
	case "json":
		return renderRedeliverJSON, nil
	default:
		return nil, fmt.Errorf(
			"invalid --output value %q: expected \"default\" or \"json\"", output)
	}
}

func renderRedeliverJSON(w io.Writer, d apitype.WebhookDelivery) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

func renderRedeliverText(w io.Writer, d apitype.WebhookDelivery) error {
	ts := time.Unix(d.Timestamp, 0).UTC().Format(time.RFC3339)

	fmt.Fprintf(w, "ID:                %s\n", d.ID)
	fmt.Fprintf(w, "Kind:              %s\n", d.Kind)
	fmt.Fprintf(w, "URL:               %s\n", d.RequestURL)
	fmt.Fprintf(w, "Timestamp:         %s\n", ts)
	fmt.Fprintf(w, "Duration:          %dms\n", d.Duration)
	if d.RequestHeaders != "" {
		fmt.Fprintln(w, "Request headers:")
		for _, line := range strings.Split(d.RequestHeaders, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				fmt.Fprintf(w, "  %s\n", line)
			}
		}
	}
	if d.Payload != "" {
		fmt.Fprintf(w, "Payload:           %s\n", d.Payload)
	}
	fmt.Fprintf(w, "Response code:     %d\n", d.ResponseCode)
	if d.ResponseBody != "" {
		fmt.Fprintln(w, "Response body:")
		for _, line := range strings.Split(d.ResponseBody, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				fmt.Fprintf(w, "  %s\n", line)
			}
		}
	}
	return nil
}
