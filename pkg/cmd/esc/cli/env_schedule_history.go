// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvScheduleHistoryCmd(env *envCommand) *cobra.Command {
	var (
		utc   bool
		count int
	)

	cmd := &cobra.Command{
		Use:   "history [<org-name>/][<project-name>/]<environment-name> <schedule-id>",
		Short: "Show the execution history of an environment scheduled action.",
		Long: "[EXPERIMENTAL] Show the execution history of an environment scheduled action\n" +
			"\n" +
			"This command lists past executions of a scheduled action.\n",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

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

			printScheduleHistory(env.esc.stdout, resp, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")
	cmd.Flags().IntVar(&count, "count", 0, "the maximum number of events to return (all if unset)")

	return cmd
}

func printScheduleHistory(stdout io.Writer, resp *client.ListScheduleHistoryResponse, utc utcFlag) {
	if resp == nil {
		return
	}
	for i, e := range resp.ScheduleHistoryEvents {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "ID: %s\n", e.ID)
		fmt.Fprintf(stdout, "Executed: %s\n", formatHistoryTime(e.Executed, utc))
		fmt.Fprintf(stdout, "Version: %d\n", e.Version)
		fmt.Fprintf(stdout, "Result: %s\n", e.Result)
	}
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
