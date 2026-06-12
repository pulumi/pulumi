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
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

func newStackWebhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "[EXPERIMENTAL] Manage stack webhooks",
		Long:  "[EXPERIMENTAL] Manage stack webhooks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newStackWebhookListCmd())
	cmd.AddCommand(newStackWebhookGetCmd())
	cmd.AddCommand(newStackWebhookNewCmd())
	cmd.AddCommand(newStackWebhookEditCmd())
	cmd.AddCommand(newStackWebhookRemoveCmd())
	cmd.AddCommand(newStackWebhookPingCmd())
	cmd.AddCommand(newStackWebhookDeliveryCmd())
	return cmd
}

func stackWebhookHookArg() *constrictor.Arguments {
	return &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "id"},
		},
		Required: 1,
	}
}

func newStackWebhookDeliveryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delivery",
		Short: "[EXPERIMENTAL] Inspect stack webhook deliveries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newStackWebhookDeliveryListCmd())
	cmd.AddCommand(newStackWebhookDeliveryRedeliverCmd())
	return cmd
}
