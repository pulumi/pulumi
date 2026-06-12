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
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type orgWebhookPingRender func(cmd *orgWebhookPingCmd, d apitype.WebhookDelivery) error

type orgWebhookPingCmd struct {
	orgName string
	output  outputflag.OutputFlag[orgWebhookPingRender]
	w       io.Writer

	ws             pkgWorkspace.Context
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
		*workspace.Project, display.Options,
	) (backend.Backend, error)
}

func newOrgWebhookPingCmd() *cobra.Command {
	opcmd := &orgWebhookPingCmd{
		output: outputflag.OutputFlag[orgWebhookPingRender]{
			RenderForTerminal: (*orgWebhookPingCmd).renderText,
			RenderJSON:        (*orgWebhookPingCmd).renderJSON,
		},
		ws:             pkgWorkspace.Instance,
		currentBackend: cmdBackend.CurrentBackend,
	}

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "[EXPERIMENTAL] Send a test ping to an organization webhook",
		Long: "[EXPERIMENTAL] Send a test ping to an organization webhook.\n" +
			"\n" +
			"Issues a test ping event to the specified webhook to verify it is\n" +
			"properly configured and reachable. Returns the delivery result\n" +
			"including the HTTP response code and duration.",
		Example: "  # Ping a webhook\n" +
			"  pulumi org webhook ping 1a2b3c4d\n\n" +
			"  # Ping and get full delivery details as JSON\n" +
			"  pulumi org webhook ping 1a2b3c4d --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			opcmd.w = cmd.OutOrStdout()
			return opcmd.run(cmd.Context(), args[0])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "id"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&opcmd.orgName, "org", "",
		"The organization that owns the webhook. Defaults to the current org.")
	outputflag.VarP(cmd.Flags(), &opcmd.output)

	return cmd
}

func (c *orgWebhookPingCmd) run(ctx context.Context, webhookName string) error {
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

	delivery, err := cloudBackend.Client().PingOrgWebhook(ctx, orgName, webhookName)
	if err != nil {
		return fmt.Errorf("pinging organization webhook: %w", err)
	}

	return c.output.Get()(c, delivery)
}

func (c *orgWebhookPingCmd) renderJSON(d apitype.WebhookDelivery) error {
	enc := json.NewEncoder(c.w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

func (c *orgWebhookPingCmd) renderText(d apitype.WebhookDelivery) error {
	ts := time.Unix(d.Timestamp, 0).UTC().Format(time.RFC3339)

	fmt.Fprintf(c.w, "ID:                %s\n", d.ID)
	fmt.Fprintf(c.w, "Kind:              %s\n", d.Kind)
	fmt.Fprintf(c.w, "URL:               %s\n", d.RequestURL)
	fmt.Fprintf(c.w, "Timestamp:         %s\n", ts)
	fmt.Fprintf(c.w, "Duration:          %dms\n", d.Duration)
	if d.RequestHeaders != "" {
		fmt.Fprintln(c.w, "Request headers:")
		for _, line := range strings.Split(d.RequestHeaders, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				fmt.Fprintf(c.w, "  %s\n", line)
			}
		}
	}
	if d.Payload != "" {
		fmt.Fprintf(c.w, "Payload:           %s\n", d.Payload)
	}
	fmt.Fprintf(c.w, "Response code:     %d\n", d.ResponseCode)
	if d.ResponseBody != "" {
		fmt.Fprintln(c.w, "Response body:")
		for _, line := range strings.Split(d.ResponseBody, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				fmt.Fprintf(c.w, "  %s\n", line)
			}
		}
	}
	return nil
}
