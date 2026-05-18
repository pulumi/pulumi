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

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

type stackScheduleRemoveClient interface {
	DeleteStackSchedule(ctx context.Context, stackID client.StackIdentifier, scheduleID string) error
}

type stackScheduleRemoveClientFactory func(
	ctx context.Context, stackFlag string,
) (stackScheduleRemoveClient, client.StackIdentifier, error)

func newStackScheduleRemoveCmd() *cobra.Command {
	return newStackScheduleRemoveCmdWith(nil)
}

func newStackScheduleRemoveCmdWith(factory stackScheduleRemoveClientFactory) *cobra.Command {
	var (
		stack string
		yes   bool
	)

	cmd := &cobra.Command{
		Use:   "remove <schedule-id>",
		Short: "[EXPERIMENTAL] Delete a scheduled deployment action",
		Long: "[EXPERIMENTAL] Delete a scheduled deployment action.\n\n" +
			"You will be prompted to confirm by typing `remove` unless --yes is passed.",
		Example: "  # Remove a schedule (prompts for confirmation)\n" +
			"  pulumi stack schedule remove bb61b60a-a313-46cb-b4ab-9d42dce46de8\n" +
			"\n" +
			"  # Remove without confirmation\n" +
			"  pulumi stack schedule remove bb61b60a-a313-46cb-b4ab-9d42dce46de8 --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackScheduleRemoveClientFactory
			}
			return runStackScheduleRemove(
				cmd.Context(), cmd.OutOrStdout(), factory, stack, args[0], yes,
			)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "schedule-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"Skip confirmation prompts")

	return cmd
}

func defaultStackScheduleRemoveClientFactory(
	ctx context.Context, stackFlag string,
) (stackScheduleRemoveClient, client.StackIdentifier, error) {
	return RequireCloudStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

func runStackScheduleRemove(
	ctx context.Context,
	w io.Writer,
	factory stackScheduleRemoveClientFactory,
	stackFlag string,
	scheduleID string,
	yes bool,
) error {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	if !yes {
		if !cmdutil.Interactive() {
			return backenderr.NonInteractiveRequiresYesError{}
		}
		prompt := fmt.Sprintf("This will remove the schedule '%s'!", scheduleID)
		if !ui.ConfirmPrompt(prompt, "remove", opts) {
			return result.FprintBailf(w, "confirmation declined")
		}
	}

	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	if err := c.DeleteStackSchedule(ctx, stackID, scheduleID); err != nil {
		return fmt.Errorf("removing stack schedule: %w", err)
	}

	fmt.Fprintf(w, "Schedule '%s' has been removed.\n", scheduleID)
	return nil
}
