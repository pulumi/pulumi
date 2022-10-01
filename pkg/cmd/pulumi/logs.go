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

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	mobytime "github.com/moby/moby/api/types/time"
	"github.com/spf13/cobra"
	logs "go.opentelemetry.io/proto/otlp/logs/v1"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// We use RFC 5424 timestamps with millisecond precision for displaying time stamps on log entries. Go does not
// pre-define a format string for this format, though it is similar to time.RFC3339Nano.
//
// See https://tools.ietf.org/html/rfc5424#section-6.2.3.
const timeFormat = "2006-01-02T15:04:05.000Z07:00"

type logsInfo struct {
	root    string
	project *workspace.Project
	target  *deploy.Target
}

func (i *logsInfo) GetRoot() string {
	return i.root
}

func (i *logsInfo) GetProject() *workspace.Project {
	return i.project
}

func (i *logsInfo) GetTarget() *deploy.Target {
	return i.target
}

func newLogsCmd() *cobra.Command {
	var stackID string
	var follow bool
	var since string
	var resource string
	var jsonOut bool

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "[PREVIEW] Show aggregated resource logs for a stack",
		Long: "[PREVIEW] Show aggregated resource logs for a stack\n" +
			"\n" +
			"This command aggregates log entries associated with the resources in a stack from the corresponding\n" +
			"provider. For example, for AWS resources, the `pulumi logs` command will query\n" +
			"CloudWatch Logs for log data relevant to resources in a stack.\n",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stackID, false, opts, false /*setCurrent*/)
			if err != nil {
				return err
			}

			sm, err := getStackSecretsManager(s)
			if err != nil {
				return fmt.Errorf("getting secrets manager: %w", err)
			}

			cfg, err := getStackConfiguration(s, sm)
			if err != nil {
				return fmt.Errorf("getting stack configuration: %w", err)
			}

			proj, root, err := readProject()
			if err != nil {
				return fmt.Errorf("reading project: %w", err)
			}

			untypedDeployment, err := s.ExportDeployment(context.Background())
			if err != nil {
				return fmt.Errorf("exporting deployment: %w", err)
			}

			snapshot, err := stack.DeserializeUntypedDeployment(untypedDeployment, stack.DefaultSecretsProvider)
			if err != nil {
				return fmt.Errorf("deserializing deployment: %w", err)
			}

			info := &logsInfo{
				root:    root,
				project: proj,
				target: &deploy.Target{
					Name:      s.Ref().Name(),
					Config:    cfg.Config,
					Decrypter: cfg.Decrypter,
					Snapshot:  snapshot,
				},
			}
			sink := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{Color: opts.Color})
			providers, err := engine.LoadProviders(info, engine.ProvidersOptions{
				Diag:       sink,
				StatusDiag: sink,
			})
			if err != nil {
				return err
			}
			tree := operations.NewResourceTree(snapshot.Resources)
			ops := tree.OperationsProvider(providers)

			startTime, err := parseSince(since, time.Now())
			if err != nil {
				return fmt.Errorf("failed to parse argument to '--since' as duration or timestamp: %w", err)
			}

			resourceFilter := resource

			if !jsonOut {
				fmt.Printf(
					opts.Color.Colorize(colors.BrightMagenta+"Collecting logs for stack %s since %s.\n\n"+colors.Reset),
					s.Ref().String(),
					startTime.Format(timeFormat),
				)
			}

			// IDEA: This map will grow forever as new log entries are found.  We may need to do a more approximate
			// approach here to ensure we don't grow memory unboundedly while following logs.
			//
			// Note: Just tracking latest log date is not sufficient - as stale logs may show up which should have been
			// displayed before previously rendered log entries, but weren't available at the time, so still need to be
			// rendered now even though they are technically out of order.
			shown := map[operations.LogEntry]bool{}
			for {
				var entries []*logs.ResourceLogs
				var token interface{}
				for {
					batch, nextToken, err := ops.GetLogs(operations.LogQuery{
						StartTime:         startTime,
						ResourceFilter:    resourceFilter,
						ContinuationToken: token,
					})
					if err != nil {
						return fmt.Errorf("failed to get logs: %w", err)
					}
					entries = append(entries, batch...)
					if nextToken == nil {
						break
					}
					token = nextToken
				}
				logs := operations.PivotLogs(entries)

				// When we are emitting a fixed number of log entries, and outputing JSON, wrap them in an array.
				if !follow && jsonOut {
					entries := make([]logEntryJSON, 0, len(logs))

					for _, logEntry := range logs {
						if _, shownAlready := shown[logEntry]; !shownAlready {
							eventTime := time.Unix(0, logEntry.Timestamp*1000000)

							entries = append(entries, logEntryJSON{
								ID:        string(logEntry.ID),
								Timestamp: eventTime.UTC().Format(timeFormat),
								Message:   logEntry.Message,
							})

							shown[logEntry] = true
						}
					}

					return printJSON(entries)
				}

				for _, logEntry := range logs {
					if _, shownAlready := shown[logEntry]; !shownAlready {
						eventTime := time.Unix(0, logEntry.Timestamp*1000000)

						if !jsonOut {
							fmt.Printf(
								"%30.30s[%30.30s] %v\n",
								eventTime.Format(timeFormat),
								logEntry.ID,
								strings.TrimRight(logEntry.Message, "\n"),
							)
						} else {
							err = printJSON(logEntryJSON{
								ID:        string(logEntry.ID),
								Timestamp: eventTime.UTC().Format(timeFormat),
								Message:   logEntry.Message,
							})
							if err != nil {
								return err
							}
						}

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
		&stackID, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	logsCmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	logsCmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")
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

// logEntryJSON is the shape of the --json output of this command. When --json is passed, if we are not following the
// log stream, we print an array of logEntry objects. If we are following the log stream, we instead print each object
// at top level.
type logEntryJSON struct {
	ID        string
	Timestamp string
	Message   string
}
