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

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

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
		stack string
		count int
	)
	output := outputflag.OutputFlag[scheduleListRenderFunc]{
		RenderForTerminal: renderScheduleListTable,
		RenderJSON:        renderScheduleListJSON,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[EXPERIMENTAL] List all scheduled actions configured for a stack",
		Long:  "[EXPERIMENTAL] List all scheduled actions configured for a stack.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackScheduleListClientFactory
			}
			return runStackScheduleList(cmd.Context(), cmd.OutOrStdout(), factory, stack, count, output.Get())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().IntVar(&count, "count", 0,
		"Show only the first N schedules. 0 (the default) shows all")
	outputflag.Var(cmd.Flags(), &output)

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
	count int,
	render scheduleListRenderFunc,
) error {
	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	schedules, err := c.ListStackSchedules(ctx, stackID)
	if err != nil {
		return fmt.Errorf("listing stack schedules: %w", err)
	}

	if count > 0 && count < len(schedules) {
		schedules = schedules[:count]
	}

	return render(w, schedules)
}

type scheduleListRenderFunc func(w io.Writer, schedules []apitype.ScheduledAction) error

func renderScheduleListJSON(w io.Writer, schedules []apitype.ScheduledAction) error {
	rows := make([]scheduleSummary, 0, len(schedules))
	for _, s := range schedules {
		rows = append(rows, summarizeSchedule(s))
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Schedules []scheduleSummary `json:"schedules"`
	}{Schedules: rows})
}

// formatSchedule renders the SCHEDULE column. Cron schedules show their expression; one-shot schedules show the literal
// "Once" and the actual timestamp lives in NEXT RUN.
func formatSchedule(s apitype.ScheduledAction) string {
	if s.ScheduleCron != "" {
		return s.ScheduleCron
	}
	if s.ScheduleOnce != "" {
		return "Once"
	}
	return ""
}

func renderScheduleListTable(w io.Writer, schedules []apitype.ScheduledAction) error {
	if len(schedules) == 0 {
		fmt.Fprintln(w, "No scheduled actions configured for this stack.")
		return nil
	}

	header := table.Row{"ID", "TYPE", "SETTINGS", "SCHEDULE", "NEXT RUN", "LAST RUN", "CREATED"}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(header)

	for _, s := range schedules {
		lastRun := ""
		if s.LastExecuted != nil {
			lastRun = *s.LastExecuted
		}
		t.AppendRow(table.Row{
			s.ID,
			scheduleKindLabel(s),
			scheduleSettings(s),
			formatSchedule(s),
			s.NextExecution,
			lastRun,
			s.Created,
		})
	}

	// Let SETTINGS absorb extra width when the terminal is wide.
	cols := cmdCmd.StdoutWidth()
	borderWidth := 3*len(header) + 1
	fixedColsWidth := 100
	settingsWidth := max(cols-borderWidth-fixedColsWidth, 20)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "SETTINGS", WidthMax: settingsWidth, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()
	return nil
}
