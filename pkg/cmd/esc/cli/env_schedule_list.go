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

package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
)

func newEnvScheduleListCmd(env *envCommand) *cobra.Command {
	var (
		utc    bool
		count  int
		output string
	)

	cmd := &cobra.Command{
		Use:     "list [<org-name>/][<project-name>/]<environment-name>",
		Aliases: []string{"ls"},
		Short:   "List environment scheduled actions.",
		Long: "[EXPERIMENTAL] List environment scheduled actions\n" +
			"\n" +
			"This command lists the scheduled actions configured for the given environment.\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the list command does not accept versions")
			}
			if count < 0 {
				return errors.New("--count must be non-negative")
			}

			resp, err := env.esc.client.ListEnvironmentSchedules(ctx, ref.orgName, ref.projectName, ref.envName)
			if err != nil {
				return err
			}

			if count > 0 && resp != nil && len(resp.Schedules) > count {
				resp.Schedules = resp.Schedules[:count]
			}

			if format == outputJSON {
				out := struct {
					Schedules []scheduleJSON `json:"schedules"`
				}{Schedules: []scheduleJSON{}}
				if resp != nil {
					out.Schedules = make([]scheduleJSON, 0, len(resp.Schedules))
					for _, s := range resp.Schedules {
						out.Schedules = append(out.Schedules, newScheduleJSON(s, utcFlag(utc)))
					}
				}
				return writeJSON(env.esc.stdout, out)
			}

			printSchedules(env.esc.stdout, resp, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")
	cmd.Flags().IntVar(&count, "count", 0, "the maximum number of schedules to return (all if unset)")
	addOutputFlag(cmd, &output)

	return cmd
}

// scheduleJSON is the slim per-schedule projection emitted by JSON output.
// Mirrors the fields shown by printSchedule; internal timestamps (created,
// modified) and the org/definition blob are omitted.
type scheduleJSON struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Schedule      string `json:"schedule"`
	NextExecution string `json:"nextExecution"`
	LastExecuted  string `json:"lastExecuted"`
}

func newScheduleJSON(s client.ScheduledAction, utc utcFlag) scheduleJSON {
	return scheduleJSON{
		ID:            s.ID,
		Kind:          s.Kind,
		Schedule:      scheduleExpr(s, utc),
		NextExecution: formatScheduleTime(s.NextExecution, utc),
		LastExecuted:  formatScheduleTime(s.LastExecuted, utc),
	}
}

// printSchedules renders the schedules as a table.
func printSchedules(stdout io.Writer, resp *client.ListScheduledActionsResponse, utc utcFlag) {
	if resp == nil || len(resp.Schedules) == 0 {
		return
	}
	t := newTable(stdout)
	t.AppendHeader(table.Row{"ID", "KIND", "SCHEDULE", "NEXT", "LAST"})
	for _, s := range resp.Schedules {
		t.AppendRow(table.Row{
			s.ID,
			s.Kind,
			scheduleExpr(s, utc),
			formatScheduleTime(s.NextExecution, utc),
			formatScheduleTime(s.LastExecuted, utc),
		})
	}
	t.Render()
}

// printSchedule renders a single schedule as a key/value block.
func printSchedule(stdout io.Writer, s client.ScheduledAction, utc utcFlag) {
	fmt.Fprintf(stdout, "ID: %s\n", s.ID)
	fmt.Fprintf(stdout, "Kind: %s\n", s.Kind)
	fmt.Fprintf(stdout, "Schedule: %s\n", scheduleExpr(s, utc))
	fmt.Fprintf(stdout, "Next execution: %s\n", formatScheduleTime(s.NextExecution, utc))
	fmt.Fprintf(stdout, "Last executed: %s\n", formatScheduleTime(s.LastExecuted, utc))
}

func scheduleExpr(s client.ScheduledAction, utc utcFlag) string {
	switch {
	case s.ScheduleCron != "":
		return s.ScheduleCron
	case s.ScheduleOnce != "":
		return formatScheduleTime(s.ScheduleOnce, utc)
	default:
		return "<unknown>"
	}
}

// The backend serializes schedule timestamps without a timezone but always in UTC.
const scheduleTimeFormat = "2006-01-02 15:04:05.000"

// formatScheduleTime parses a schedule timestamp and re-formats it honouring the --utc flag.
// Empty values render as "never"; unparseable values pass through as-is so the user still sees
// the backend's raw response.
func formatScheduleTime(s string, utc utcFlag) string {
	if s == "" {
		return "never"
	}
	t, err := time.ParseInLocation(scheduleTimeFormat, s, time.UTC)
	if err != nil {
		if t, err = time.Parse(time.RFC3339, s); err != nil {
			return s
		}
	}
	return utc.time(t).Format(time.RFC3339)
}
