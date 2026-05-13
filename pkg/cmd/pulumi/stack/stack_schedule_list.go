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
	"strconv"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackScheduleListClient is the interface the list command needs from the API client.
type stackScheduleListClient interface {
	ListStackSchedules(ctx context.Context, stackID client.StackIdentifier) ([]apitype.ScheduledAction, error)
}

type stackScheduleListClientFactory func(
	ctx context.Context, stackFlag string,
) (stackScheduleListClient, client.StackIdentifier, error)

func newStackScheduleListCmd() *cobra.Command {
	return newStackScheduleListCmdWith(nil)
}

func newStackScheduleListCmdWith(factory stackScheduleListClientFactory) *cobra.Command {
	var (
		stack  string
		output string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all scheduled actions configured for a stack",
		Long:  "[EXPERIMENTAL] List all scheduled actions configured for a stack.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackScheduleListClientFactory
			}
			return runStackScheduleList(cmd.Context(), cmd.OutOrStdout(), factory, stack, output)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"The output format: default (human-readable table) or json")

	return cmd
}

func defaultStackScheduleListClientFactory(
	ctx context.Context, stackFlag string,
) (stackScheduleListClient, client.StackIdentifier, error) {
	return RequireCloudStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

func runStackScheduleList(
	ctx context.Context,
	w io.Writer,
	factory stackScheduleListClientFactory,
	stackFlag string,
	output string,
) error {
	renderer, err := ui.Renderer(output, ui.OutputRenderers[scheduleListRenderFunc]{
		Default: renderScheduleListTable,
		JSON:    renderScheduleListJSON,
	})
	if err != nil {
		return err
	}

	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	schedules, err := c.ListStackSchedules(ctx, stackID)
	if err != nil {
		return fmt.Errorf("listing stack schedules: %w", err)
	}

	return renderer(w, schedules)
}

type scheduleListRenderFunc func(w io.Writer, schedules []apitype.ScheduledAction) error

// deploymentOperation extracts the operation name from a scheduled deployment's definition.
// Returns the empty string for non-deployment kinds or when the definition can't be parsed.
func deploymentOperation(s apitype.ScheduledAction) string {
	if s.Kind != apitype.ScheduledActionKindDeployment || len(s.Definition) == 0 {
		return ""
	}
	var def apitype.ScheduledDeploymentDefinition
	if err := json.Unmarshal(s.Definition, &def); err != nil || def.Request == nil {
		return ""
	}
	return string(def.Request.Op)
}

func renderScheduleListJSON(w io.Writer, schedules []apitype.ScheduledAction) error {
	if schedules == nil {
		schedules = []apitype.ScheduledAction{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(apitype.ListScheduledActionsResponse{Schedules: schedules})
}

func formatSchedule(s apitype.ScheduledAction) string {
	switch {
	case s.ScheduleCron != "":
		return s.ScheduleCron
	case s.ScheduleOnce != "":
		return s.ScheduleOnce
	default:
		return ""
	}
}

func renderScheduleListTable(w io.Writer, schedules []apitype.ScheduledAction) error {
	if len(schedules) == 0 {
		fmt.Fprintln(w, "No scheduled actions configured for this stack.")
		return nil
	}

	header := table.Row{"ID", "KIND", "SCHEDULE", "OPERATION", "NEXT EXECUTION", "PAUSED"}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(header)

	for _, s := range schedules {
		t.AppendRow(table.Row{
			s.ID,
			string(s.Kind),
			formatSchedule(s),
			deploymentOperation(s),
			s.NextExecution,
			strconv.FormatBool(s.Paused),
		})
	}

	// Let SCHEDULE absorb remaining width when needed.
	cols := cmdCmd.StdoutWidth()
	borderWidth := 3*len(header) + 1
	fixedColsWidth := 60
	scheduleWidth := max(cols-borderWidth-fixedColsWidth, 20)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "SCHEDULE", WidthMax: scheduleWidth, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()
	return nil
}
