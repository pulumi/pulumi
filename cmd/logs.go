// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"time"

	mobytime "github.com/moby/moby/api/types/time"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newLogsCmd() *cobra.Command {
	var stack string
	var follow bool
	var since string
	var resource string

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Show aggregated logs for a stack",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			s, err := requireStack(stack, false)
			if err != nil {
				return err
			}

			startTime, err := parseSince(since, time.Now())
			if err != nil {
				return errors.Wrapf(err, "failed to parse argument to '--since' as duration or timestamp")
			}
			var resourceFilter *operations.ResourceFilter
			if resource != "" {
				var rf = operations.ResourceFilter(resource)
				resourceFilter = &rf
			}

			return showLogs(s, startTime, resourceFilter, follow)
		}),
	}

	logsCmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"List configuration for a different stack than the currently selected stack")
	logsCmd.PersistentFlags().BoolVarP(
		&follow, "follow", "f", false,
		"Follow the log stream in real time (like tail -f)")
	logsCmd.PersistentFlags().StringVar(
		&since, "since", "1h",
		"Only return logs newer than a relative duration ('5s', '2m', '3h') or absolute timestamp.  "+
			"Defaults to returning the last 1 hour of logs.")
	logsCmd.PersistentFlags().StringVarP(
		&resource, "resource", "r", "",
		"Only return logs for the requested resource ('name', 'type::name' or full URN).  Defaults to returning all logs.")

	return logsCmd
}

func parseSince(since string, reference time.Time) (*time.Time, error) {
	startTimestamp, err := mobytime.GetTimestamp(since, reference)
	if err != nil {
		return nil, err
	}
	startTimeSec, startTimeNs, err := mobytime.ParseTimestamps(startTimestamp, 0)
	if err != nil {
		return nil, err
	}
	if startTimeSec == 0 && startTimeNs == 0 {
		return nil, nil
	}
	startTime := time.Unix(startTimeSec, startTimeNs)
	return &startTime, nil
}
