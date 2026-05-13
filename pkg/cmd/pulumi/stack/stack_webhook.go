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
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/23063]: Not yet implemented.
func newStackWebhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "webhook",
		Short:  "Manage stack webhooks",
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
			{Name: "name"},
		},
		Required: 1,
	}
}

// TODO[https://github.com/pulumi/pulumi/issues/23062]: Not yet implemented.
func newStackWebhookListCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List all webhooks configured for a stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23061]: Not yet implemented.
func newStackWebhookGetCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "Get the details of a stack webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23060]: Not yet implemented.
func newStackWebhookNewCmd() *cobra.Command {
	var (
		stack       string
		url         string
		format      string
		filters     []string
		active      bool
		secret      string
		displayName string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new",
		Short:  "Create a new stack webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&url, "url", "", "The webhook payload URL")
	cmd.Flags().StringVar(&format, "format", "raw",
		"The webhook format: raw, slack, ms_teams, or pulumi_deployments")
	cmd.Flags().StringArrayVar(&filters, "filter", nil,
		"An event type to subscribe to (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "", "The HMAC key for signature verification")
	cmd.Flags().StringVar(&displayName, "display-name", "", "The webhook display name")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23059]: Not yet implemented.
func newStackWebhookEditCmd() *cobra.Command {
	var (
		stack       string
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
		Short:  "Update a stack webhook's configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&url, "url", "", "The webhook payload URL")
	cmd.Flags().StringVar(&format, "format", "",
		"The webhook format: raw, slack, ms_teams, or pulumi_deployments")
	cmd.Flags().StringArrayVar(&filters, "filter", nil,
		"An event type to subscribe to (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "", "The HMAC key for signature verification")
	cmd.Flags().StringVar(&displayName, "display-name", "", "The webhook display name")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23058]: Not yet implemented.
func newStackWebhookRemoveCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "remove",
		Short:  "Delete a stack webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23057]: Not yet implemented.
func newStackWebhookPingCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "ping",
		Short:  "Send a test ping to a stack webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23056]: Not yet implemented.
func newStackWebhookDeliveryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "delivery",
		Short:  "Inspect stack webhook deliveries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newStackWebhookDeliveryListCmd())
	cmd.AddCommand(newStackWebhookDeliveryGetCmd())
	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23055]: Not yet implemented.
func newStackWebhookDeliveryListCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List recent deliveries for a stack webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23054]: Not yet implemented.
func newStackWebhookDeliveryGetCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "Redeliver a specific webhook event",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
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

	return cmd
}
