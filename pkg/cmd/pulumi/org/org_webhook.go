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
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/22964]: Not yet implemented.
func newOrgWebhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "webhook",
		Short:  "Manage organization-level webhooks",
		Long:   "[EXPERIMENTAL] Manage organization-level webhooks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newOrgWebhookListCmd())
	cmd.AddCommand(newOrgWebhookNewCmd())
	cmd.AddCommand(newOrgWebhookEditCmd())
	cmd.AddCommand(newOrgWebhookRemoveCmd())
	cmd.AddCommand(newOrgWebhookPingCmd())
	cmd.AddCommand(newOrgWebhookDeliveryCmd())

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22967]: Not yet implemented.
func newOrgWebhookEditCmd() *cobra.Command {
	var (
		org         string
		url         string
		format      string
		filters     []string
		active      bool
		secret      string
		displayName string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit",
		Short:  "Update an organization webhook's configuration",
		Long:   "[EXPERIMENTAL] Update an organization webhook's configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the webhook")
	cmd.Flags().StringVar(&url, "url", "", "The webhook payload URL")
	cmd.Flags().StringVar(&format, "format", "",
		"The webhook format: raw, slack, or ms_teams")
	cmd.Flags().StringArrayVar(&filters, "filter", nil,
		"An event type to subscribe to (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "", "The HMAC key for signature verification")
	cmd.Flags().StringVar(&displayName, "display-name", "", "The webhook display name")

	return cmd
}

// newOrgWebhookRemoveCmd is defined in org_webhook_remove.go.

// TODO[https://github.com/pulumi/pulumi/issues/22969]: Not yet implemented.
func newOrgWebhookPingCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "ping",
		Short:  "Send a test ping to an organization webhook",
		Long:   "[EXPERIMENTAL] Send a test ping to an organization webhook.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the webhook")

	return cmd
}

func newOrgWebhookDeliveryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "delivery",
		Short:  "Inspect organization webhook deliveries",
		Long:   "[EXPERIMENTAL] Inspect organization webhook deliveries.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newOrgWebhookDeliveryListCmd())
	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22997]: Not yet implemented.
func newOrgWebhookDeliveryListCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List recent deliveries for an organization webhook",
		Long:   "[EXPERIMENTAL] List recent deliveries for an organization webhook.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "webhook"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the webhook")

	return cmd
}
