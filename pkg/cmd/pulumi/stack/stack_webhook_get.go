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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackWebhookGetClient is the interface the get command needs from the API client.
type stackWebhookGetClient interface {
	GetStackWebhook(ctx context.Context, stackID client.StackIdentifier, webhookName string) (apitype.Webhook, error)
}

// stackWebhookGetCmd holds the resolved dependencies for `pulumi stack webhook get`.
type stackWebhookGetCmd struct {
	client  stackWebhookGetClient
	stackID client.StackIdentifier
	output  string
}

func newStackWebhookGetCmd() *cobra.Command {
	var (
		stack  string
		output string
	)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "[EXPERIMENTAL] Get the details of a stack webhook",
		Long: "[EXPERIMENTAL] Get the details of a stack webhook.\n" +
			"\n" +
			"Displays the configuration of a single webhook including its ID, name,\n" +
			"URL, format, event groups, events, and active status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, stackID, err := RequireCloudStack(
				ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stack)
			if err != nil {
				return err
			}
			get := &stackWebhookGetCmd{
				client:  c,
				stackID: stackID,
				output:  output,
			}
			return get.run(ctx, cmd.OutOrStdout(), args[0])
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&output, "output", "default",
		"The output format: default (human-readable text) or json")

	return cmd
}

func (c *stackWebhookGetCmd) run(ctx context.Context, w io.Writer, webhookName string) error {
	renderer, err := webhookGetRenderer(c.output)
	if err != nil {
		return err
	}

	webhook, err := c.client.GetStackWebhook(ctx, c.stackID, webhookName)
	if err != nil {
		return fmt.Errorf("reading stack webhook: %w", err)
	}

	return renderer(w, webhook)
}

type webhookGetRenderFunc func(w io.Writer, webhook apitype.Webhook) error

func webhookGetRenderer(output string) (webhookGetRenderFunc, error) {
	switch output {
	case "", "default":
		return renderWebhookGetText, nil
	case "json":
		return renderWebhookGetJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q: expected \"default\" or \"json\"", output)
	}
}

// webhookGetJSON is the full JSON shape for `pulumi stack webhook get --output=json`.
// It includes all fields returned by the API, unlike the list command's summary.
type webhookGetJSON struct {
	OrganizationName string   `json:"organizationName"`
	ProjectName      string   `json:"projectName"`
	StackName        string   `json:"stackName"`
	EnvName          string   `json:"envName"`
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	URL              string   `json:"url"`
	Format           string   `json:"format"`
	Active           bool     `json:"active"`
	Groups           []string `json:"eventGroups"`
	Filters          []string `json:"events"`
	HasSecret        bool     `json:"hasSecret"`
	SecretCiphertext string   `json:"secretCiphertext"`
}

func toWebhookGetJSON(wh apitype.Webhook) webhookGetJSON {
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
	projectName := ""
	if wh.ProjectName != nil {
		projectName = *wh.ProjectName
	}
	stackName := ""
	if wh.StackName != nil {
		stackName = *wh.StackName
	}
	envName := ""
	if wh.EnvName != nil {
		envName = *wh.EnvName
	}
	return webhookGetJSON{
		OrganizationName: wh.OrganizationName,
		ProjectName:      projectName,
		StackName:        stackName,
		EnvName:          envName,
		ID:               wh.Name,
		Name:             wh.DisplayName,
		URL:              wh.PayloadURL,
		Format:           format,
		Active:           wh.Active,
		Groups:           groups,
		Filters:          filters,
		HasSecret:        wh.HasSecret,
		SecretCiphertext: wh.SecretCiphertext,
	}
}

func renderWebhookGetJSON(w io.Writer, webhook apitype.Webhook) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toWebhookGetJSON(webhook))
}

func renderWebhookGetText(w io.Writer, wh apitype.Webhook) error {
	format := ""
	if wh.Format != nil {
		format = *wh.Format
	}
	active := "yes"
	if !wh.Active {
		active = "no"
	}
	hasSecret := "no"
	if wh.HasSecret {
		hasSecret = "yes"
	}

	fmt.Fprintf(w, "ID:                %s\n", wh.Name)
	if wh.DisplayName != "" {
		fmt.Fprintf(w, "Name:              %s\n", wh.DisplayName)
	}
	fmt.Fprintf(w, "Organization:      %s\n", wh.OrganizationName)
	if wh.ProjectName != nil && *wh.ProjectName != "" {
		fmt.Fprintf(w, "Project:           %s\n", *wh.ProjectName)
	}
	if wh.StackName != nil && *wh.StackName != "" {
		fmt.Fprintf(w, "Stack:             %s\n", *wh.StackName)
	}
	if wh.EnvName != nil && *wh.EnvName != "" {
		fmt.Fprintf(w, "Environment:       %s\n", *wh.EnvName)
	}
	fmt.Fprintf(w, "URL:               %s\n", wh.PayloadURL)
	if format != "" {
		fmt.Fprintf(w, "Format:            %s\n", format)
	}
	if len(wh.Groups) > 0 {
		fmt.Fprintf(w, "Event groups:      %s\n", strings.Join(wh.Groups, ", "))
	}
	if len(wh.Filters) > 0 {
		fmt.Fprintf(w, "Events:            %s\n", strings.Join(wh.Filters, ", "))
	}
	fmt.Fprintf(w, "Active:            %s\n", active)
	fmt.Fprintf(w, "Has secret:        %s\n", hasSecret)
	if wh.SecretCiphertext != "" {
		fmt.Fprintf(w, "Secret ciphertext: %s\n", wh.SecretCiphertext)
	}
	return nil
}
