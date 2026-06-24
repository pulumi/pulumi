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
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackWebhookPingClient is the interface the ping command needs from the API client.
type stackWebhookPingClient interface {
	PingStackWebhook(
		ctx context.Context, stackID client.StackIdentifier, webhookName string,
	) (apitype.WebhookDelivery, error)
}

// stackWebhookPingClientFactory builds a stackWebhookPingClient from the environment.
type stackWebhookPingClientFactory func(
	ctx context.Context, stackFlag string,
) (stackWebhookPingClient, client.StackIdentifier, error)

func newStackWebhookPingCmd() *cobra.Command {
	return newStackWebhookPingCmdWith(nil)
}

func newStackWebhookPingCmdWith(factory stackWebhookPingClientFactory) *cobra.Command {
	var stack string
	output := outputflag.OutputFlag[webhookPingRenderFunc]{
		RenderForTerminal: renderWebhookPingText,
		RenderJSON:        renderWebhookPingJSON,
	}

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "[EXPERIMENTAL] Send a test ping to a stack webhook",
		Long: "[EXPERIMENTAL] Send a test ping to a stack webhook.\n" +
			"\n" +
			"Issues a test ping event to the specified webhook to verify it is\n" +
			"properly configured and reachable. Unlike normal webhook deliveries,\n" +
			"this bypasses the message queue and sends the request directly to the\n" +
			"webhook endpoint.\n" +
			"\n" +
			"The response includes the full delivery result: the HTTP request URL,\n" +
			"response status code, response body, and request duration.\n" +
			"\n" +
			"Returns an error if the webhook does not exist.",
		Example: "  # Ping a webhook to verify it works\n" +
			"  pulumi stack webhook ping 1a2b3c4d\n\n" +
			"  # Ping a webhook and get the full delivery details as JSON\n" +
			"  pulumi stack webhook ping 1a2b3c4d --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackWebhookPingClientFactory
			}
			return runStackWebhookPing(cmd.Context(), cmd.OutOrStdout(), factory, stack, args[0], output.Get())
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}

// defaultStackWebhookPingClientFactory resolves the current Pulumi Cloud context and
// returns a client and stack identifier.
func defaultStackWebhookPingClientFactory(
	ctx context.Context, stackFlag string,
) (stackWebhookPingClient, client.StackIdentifier, error) {
	return RequireCloudStack(
		ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

func runStackWebhookPing(
	ctx context.Context,
	w io.Writer,
	factory stackWebhookPingClientFactory,
	stackFlag string,
	webhookName string,
	render webhookPingRenderFunc,
) error {
	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	delivery, err := c.PingStackWebhook(ctx, stackID, webhookName)
	if err != nil {
		return fmt.Errorf("pinging stack webhook: %w", err)
	}

	return render(w, delivery)
}

type webhookPingRenderFunc func(w io.Writer, d apitype.WebhookDelivery) error

func renderWebhookPingJSON(w io.Writer, d apitype.WebhookDelivery) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

func renderWebhookPingText(w io.Writer, d apitype.WebhookDelivery) error {
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
