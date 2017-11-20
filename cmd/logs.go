// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newLogsCmd() *cobra.Command {
	var stack string
	var follow bool

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Show aggregated logs for a project",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName, err := explicitOrCurrent(stack, backend)
			if err != nil {
				return err
			}

			sinceTime := time.Unix(0, 0)
			highestTimeSeen := time.Unix(0, 0)

			for {
				logs, err := backend.GetLogs(stackName, operations.LogQuery{})
				if err != nil {
					return err
				}

				for _, logEntry := range logs {
					eventTime := time.Unix(0, logEntry.Timestamp*1000000)
					if eventTime.After(sinceTime) {
						fmt.Printf("[%v] %v\n", eventTime, logEntry.Message)
					}

					if eventTime.After(highestTimeSeen) {
						highestTimeSeen = eventTime
					}
				}

				if !follow {
					return nil
				}

				sinceTime = highestTimeSeen
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

	return logsCmd
}
