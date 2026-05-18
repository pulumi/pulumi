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
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

func newOrgWebhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage organization-level webhooks",
		Long:  "[EXPERIMENTAL] Manage organization-level webhooks.",
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

func newOrgWebhookDeliveryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delivery",
		Short: "[EXPERIMENTAL] Inspect organization webhook deliveries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newOrgWebhookDeliveryListCmd())
	return cmd
}
