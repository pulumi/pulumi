// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvScheduleListCmd(env *envCommand) *cobra.Command {
	var (
		utc   bool
		count int
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
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return fmt.Errorf("the list command does not accept versions")
			}
			if count < 0 {
				return fmt.Errorf("--count must be non-negative")
			}

			resp, err := env.esc.client.ListEnvironmentSchedules(ctx, ref.orgName, ref.projectName, ref.envName)
			if err != nil {
				return err
			}

			if count > 0 && resp != nil && len(resp.Schedules) > count {
				resp.Schedules = resp.Schedules[:count]
			}

			printSchedules(env.esc.stdout, resp, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")
	cmd.Flags().IntVar(&count, "count", 0, "the maximum number of schedules to return (all if unset)")

	return cmd
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
