// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newLogsCmd() *cobra.Command {
	var stack string
	var follow bool
	var since string

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Show aggregated logs for a project",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName, err := explicitOrCurrent(stack, backend)
			if err != nil {
				return err
			}

			startTime := parseRelativeDuration(since)

			// IDEA: This map will grow forever as new log entries are found.  We may need to do a more approximate
			// approach here to ensure we don't grow memory unboundedly while following logs.
			//
			// Note: Just tracking latest log date is not sufficient - as stale logs may show up which should have been
			// displayed before previously rendered log entries, but weren't available at the time, so still need to be
			// rendered now even though they are technically out of order.
			shown := map[operations.LogEntry]bool{}

			for {
				logs, err := backend.GetLogs(stackName, operations.LogQuery{
					StartTime: startTime,
				})
				if err != nil {
					return err
				}

				for _, logEntry := range logs {
					if _, shownAlready := shown[logEntry]; !shownAlready {
						eventTime := time.Unix(0, logEntry.Timestamp*1000000)
						fmt.Printf("%30.30s[%30.30s] %v\n", eventTime.Format(time.RFC3339Nano), logEntry.ID, logEntry.Message)
						shown[logEntry] = true
					}
				}

				if !follow {
					return nil
				}

				time.Sleep(time.Second)
			}
		}),
	}

	logsCmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"List configuration for a different stack than the currently selected stack")
	logsCmd.PersistentFlags().BoolVarP(
		&follow, "follow", "f", false,
		"Follow the log stream in real time (like tail -f)")
	logsCmd.PersistentFlags().StringVar(
		&since, "since", "",
		"Only return logs newer than a relative duration ('5s', '2m', '3h').  Defaults to returning all logs.")

	return logsCmd
}

var durationRegexp = regexp.MustCompile(`(\d+)([y|w|d|h|m|s])`)

// parseRelativeDuration extracts a time.Time previous to now by the a relative duration in the format '5s', '2m', '3h'.
func parseRelativeDuration(duration string) *time.Time {
	now := time.Now()
	if duration == "" {
		return nil
	}
	parts := durationRegexp.FindStringSubmatch(duration)
	if parts == nil {
		fmt.Printf("Warning: duration could not be parsed: '%v'\n", duration)
		return nil
	}
	num, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		fmt.Printf("Warning: duration could not be parsed: '%v'\n", duration)
		return nil
	}
	d := time.Duration(-num)
	switch parts[2] {
	case "y":
		d *= time.Hour * 24 * 365
	case "w":
		d *= time.Hour * 24 * 7
	case "d":
		d *= time.Hour * 24
	case "h":
		d *= time.Hour
	case "m":
		d *= time.Minute
	case "s":
		d *= time.Second
	default:
		fmt.Printf("Warning: duration could not be parsed: '%v'\n", duration)
		return nil
	}
	ret := now.Add(d)
	return &ret
}
