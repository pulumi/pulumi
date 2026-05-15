// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvScheduleHistoryCmd(env *envCommand) *cobra.Command {
	var (
		utc    bool
		count  int
		output string
	)

	cmd := &cobra.Command{
		Use:   "history [<org-name>/][<project-name>/]<environment-name> <schedule-id>",
		Short: "Show the execution history of an environment scheduled action.",
		Long: "[EXPERIMENTAL] Show the execution history of an environment scheduled action\n" +
			"\n" +
			"This command lists past executions of a scheduled action.\n",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the history command does not accept versions")
			}
			if count < 0 {
				return errors.New("--count must be non-negative")
			}

			scheduleID := args[0]
			if scheduleID == "" {
				return errors.New("schedule ID cannot be empty")
			}

			resp, err := env.esc.client.ListEnvironmentScheduleHistory(ctx, ref.orgName, ref.projectName, ref.envName, scheduleID)
			if err != nil {
				return err
			}

			if count > 0 && resp != nil && len(resp.ScheduleHistoryEvents) > count {
				resp.ScheduleHistoryEvents = resp.ScheduleHistoryEvents[:count]
			}

			if format == outputJSON {
				out := struct {
					Events []scheduleHistoryEventJSON `json:"events"`
				}{Events: []scheduleHistoryEventJSON{}}
				if resp != nil {
					out.Events = make([]scheduleHistoryEventJSON, 0, len(resp.ScheduleHistoryEvents))
					for _, e := range resp.ScheduleHistoryEvents {
						out.Events = append(out.Events, scheduleHistoryEventJSON{
							ID:       e.ID,
							Executed: formatHistoryTime(e.Executed, utcFlag(utc)),
							Version:  e.Version,
							Result:   e.Result,
						})
					}
				}
				return writeJSON(env.esc.stdout, out)
			}

			printScheduleHistory(env.esc.stdout, resp, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")
	cmd.Flags().IntVar(&count, "count", 0, "the maximum number of events to return (all if unset)")
	addOutputFlag(cmd, &output)

	return cmd
}

// scheduleHistoryEventJSON is the slim per-event projection emitted by JSON
// output. Mirrors the fields shown by printScheduleHistory; the parent
// scheduledActionID is omitted because the user provided it as a CLI arg.
type scheduleHistoryEventJSON struct {
	ID       string `json:"id"`
	Executed string `json:"executed"`
	Version  int    `json:"version"`
	Result   string `json:"result"`
}

func printScheduleHistory(stdout io.Writer, resp *client.ListScheduleHistoryResponse, utc utcFlag) {
	if resp == nil || len(resp.ScheduleHistoryEvents) == 0 {
		return
	}
	t := newTable(stdout)
	t.AppendHeader(table.Row{"ID", "EXECUTED", "VERSION", "RESULT"})
	for _, e := range resp.ScheduleHistoryEvents {
		t.AppendRow(table.Row{
			e.ID,
			formatHistoryTime(e.Executed, utc),
			e.Version,
			e.Result,
		})
	}
	t.Render()
}

// formatHistoryTime parses an event timestamp (RFC 3339 on the wire) and re-formats it honouring
// the --utc flag. Unparseable values pass through as-is.
func formatHistoryTime(s string, utc utcFlag) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		if t, err = time.ParseInLocation(scheduleTimeFormat, s, time.UTC); err != nil {
			return s
		}
	}
	return utc.time(t).Format(time.RFC3339)
}
