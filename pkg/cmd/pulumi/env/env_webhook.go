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

package env

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/23030]: Not yet implemented.
func newEnvWebhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "webhook",
		Short:  "Manage environment webhooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newEnvWebhookListCmd())
	cmd.AddCommand(newEnvWebhookNewCmd())
	cmd.AddCommand(newEnvWebhookEditCmd())
	cmd.AddCommand(newEnvWebhookRemoveCmd())
	cmd.AddCommand(newEnvWebhookPingCmd())
	cmd.AddCommand(newEnvWebhookDeliveryCmd())

	return cmd
}

func envWebhookEnvArg() *constrictor.Arguments {
	return &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "project"},
			{Name: "name"},
		},
		Required: 2,
	}
}

func envWebhookEnvWithHookArg() *constrictor.Arguments {
	return &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "project"},
			{Name: "name"},
			{Name: "webhook"},
		},
		Required: 3,
	}
}

// TODO[https://github.com/pulumi/pulumi/issues/23029]: Not yet implemented.
func newEnvWebhookListCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List all webhooks configured for an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23028]: Not yet implemented.
func newEnvWebhookNewCmd() *cobra.Command {
	var (
		org     string
		url     string
		format  string
		filters []string
		active  bool
		secret  string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new",
		Short:  "Create a new environment webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVar(&url, "url", "", "The webhook payload URL")
	cmd.Flags().StringVar(&format, "format", "raw",
		"The webhook format: raw, slack, ms_teams, or pulumi_deployments")
	cmd.Flags().StringArrayVar(&filters, "filter", nil,
		"An event type to subscribe to (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "", "The HMAC key for signature verification")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23027]: Not yet implemented.
func newEnvWebhookEditCmd() *cobra.Command {
	var (
		org     string
		url     string
		format  string
		filters []string
		active  bool
		secret  string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit",
		Short:  "Update an environment webhook's configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")
	cmd.Flags().StringVar(&url, "url", "", "The webhook payload URL")
	cmd.Flags().StringVar(&format, "format", "",
		"The webhook format: raw, slack, ms_teams, or pulumi_deployments")
	cmd.Flags().StringArrayVar(&filters, "filter", nil,
		"An event type to subscribe to (repeatable)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the webhook is active")
	cmd.Flags().StringVar(&secret, "secret", "", "The HMAC key for signature verification")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23026]: Not yet implemented.
func newEnvWebhookRemoveCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "remove",
		Short:  "Delete an environment webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23025]: Not yet implemented.
func newEnvWebhookPingCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "ping",
		Short:  "Send a test ping to an environment webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23024]: Not yet implemented.
func newEnvWebhookDeliveryCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "delivery",
		Short:  "Inspect environment webhook deliveries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	cmd.AddCommand(newEnvWebhookDeliveryListCmd())
	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23023]: Not yet implemented.
func newEnvWebhookDeliveryListCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List recent deliveries for an environment webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, envWebhookEnvWithHookArg())

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the environment")

	return cmd
}
