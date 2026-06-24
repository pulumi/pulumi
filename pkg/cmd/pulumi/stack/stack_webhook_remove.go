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
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackWebhookRemoveClient is the interface the remove command needs.
type stackWebhookRemoveClient interface {
	DeleteStackWebhook(
		ctx context.Context, stackID client.StackIdentifier, webhookName string,
	) error
}

// stackWebhookRemoveClientFactory builds a stackWebhookRemoveClient.
type stackWebhookRemoveClientFactory func(
	ctx context.Context, stackFlag string,
) (stackWebhookRemoveClient, client.StackIdentifier, error)

func newStackWebhookRemoveCmd() *cobra.Command {
	return newStackWebhookRemoveCmdWith(nil)
}

func newStackWebhookRemoveCmdWith(factory stackWebhookRemoveClientFactory) *cobra.Command {
	var (
		stack string
		yes   bool
	)

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "[EXPERIMENTAL] Delete a stack webhook",
		Long: "[EXPERIMENTAL] Delete a stack webhook.\n" +
			"\n" +
			"Permanently removes the specified webhook from the stack. This cannot\n" +
			"be undone. You will be prompted to confirm unless --yes is passed.\n" +
			"\n" +
			"Returns an error if the webhook does not exist.",
		Example: "  # Remove a webhook (will prompt for confirmation)\n" +
			"  pulumi stack webhook remove 1a2b3c4d\n\n" +
			"  # Remove without confirmation\n" +
			"  pulumi stack webhook remove 1a2b3c4d --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			yes = yes || env.SkipConfirmations.Value()
			if factory == nil {
				factory = defaultStackWebhookRemoveClientFactory
			}
			return runStackWebhookRemove(
				cmd.Context(), cmd.OutOrStdout(), factory,
				stack, args[0], yes,
			)
		},
	}

	constrictor.AttachArguments(cmd, stackWebhookHookArg())

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"Skip confirmation prompts")

	return cmd
}

func defaultStackWebhookRemoveClientFactory(
	ctx context.Context, stackFlag string,
) (stackWebhookRemoveClient, client.StackIdentifier, error) {
	return RequireCloudStack(
		ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

func runStackWebhookRemove(
	ctx context.Context,
	w io.Writer,
	factory stackWebhookRemoveClientFactory,
	stackFlag string,
	webhookName string,
	yes bool,
) error {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	prompt := fmt.Sprintf("This will permanently remove the webhook '%s'!", webhookName)
	if err := ui.ConfirmDeletion(yes, cmdutil.Interactive(), prompt, webhookName, w, opts); err != nil {
		return err
	}

	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	if err := c.DeleteStackWebhook(ctx, stackID, webhookName); err != nil {
		return fmt.Errorf("removing stack webhook: %w", err)
	}

	fmt.Fprintf(w, "Webhook '%s' has been removed.\n", webhookName)
	return nil
}
