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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type stackScheduleGetClient interface {
	GetStackSchedule(
		ctx context.Context, stackID client.StackIdentifier, scheduleID string,
	) (apitype.ScheduledAction, error)
}

type stackScheduleGetClientFactory func(
	ctx context.Context, stackFlag string,
) (stackScheduleGetClient, client.StackIdentifier, error)

func newStackScheduleGetCmd() *cobra.Command {
	return newStackScheduleGetCmdWith(nil)
}

func newStackScheduleGetCmdWith(factory stackScheduleGetClientFactory) *cobra.Command {
	var stack string
	output := outputflag.OutputFlag[scheduleGetRenderFunc]{
		RenderForTerminal: renderScheduleGetText,
		RenderJSON:        renderScheduleGetJSON,
	}

	cmd := &cobra.Command{
		Use:   "get <schedule-id>",
		Short: "[EXPERIMENTAL] Retrieve the configuration of a stack schedule",
		Long:  "[EXPERIMENTAL] Retrieve the configuration of a stack schedule.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackScheduleGetClientFactory
			}
			return runStackScheduleGet(cmd.Context(), cmd.OutOrStdout(), factory, stack, args[0], output.Get())
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
	outputflag.Var(cmd.Flags(), &output)

	return cmd
}

func defaultStackScheduleGetClientFactory(
	ctx context.Context, stackFlag string,
) (stackScheduleGetClient, client.StackIdentifier, error) {
	return RequireCloudStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

func runStackScheduleGet(
	ctx context.Context,
	w io.Writer,
	factory stackScheduleGetClientFactory,
	stackFlag, scheduleID string,
	render scheduleGetRenderFunc,
) error {
	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	schedule, err := c.GetStackSchedule(ctx, stackID, scheduleID)
	if err != nil {
		return fmt.Errorf("getting stack schedule: %w", err)
	}

	return render(w, schedule)
}

type scheduleGetRenderFunc func(w io.Writer, schedule apitype.ScheduledAction) error

func renderScheduleGetJSON(w io.Writer, schedule apitype.ScheduledAction) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(summarizeSchedule(schedule))
}

func renderScheduleGetText(w io.Writer, s apitype.ScheduledAction) error {
	sum := summarizeSchedule(s)
	line := func(label, value string) {
		fmt.Fprintf(w, "%-10s %s\n", label+":", value)
	}
	lastRun := sum.LastRun
	if lastRun == "" {
		lastRun = "(never)"
	}
	line("ID", sum.ID)
	line("Type", sum.Type)
	line("Settings", sum.Settings)
	line("Schedule", sum.Schedule)
	line("Next run", sum.NextRun)
	line("Last run", lastRun)
	line("Created", sum.Created)
	return nil
}
