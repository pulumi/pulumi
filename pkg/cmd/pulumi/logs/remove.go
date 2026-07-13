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

package logs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newRemoveCmd() *cobra.Command {
	var before string
	var all bool
	var yes bool

	command := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "Remove automatic log files",
		Long: "Remove automatic log files.\n" +
			"\n" +
			"If no filters are given, a list of available log files is\n" +
			"displayed and the user is prompted to choose one to remove.\n" +
			"\n" +
			"Logs can also be removed in bulk by passing --stack to limit\n" +
			"removal to a single stack, --before to remove logs older than\n" +
			"a given date or duration, or --all to remove every log file.\n" +
			"\n" +
			"By default the user is asked to confirm by typing the stack\n" +
			"name (or 'yes' when multiple stacks are involved). Pass --yes\n" +
			"to skip the prompt.",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			stackName, _ := cobraCmd.Flags().GetString("stack")
			yes = yes || env.SkipConfirmations.Value()

			logsDir, err := workspace.GetPulumiPath("logs")
			if err != nil {
				return fmt.Errorf("getting log directory: %w", err)
			}

			opts := display.Options{Color: cmdutil.GetGlobalColorization()}

			targets, err := selectLogsToRemove(logsDir, stackName, before, all)
			if err != nil {
				return err
			}
			if len(targets) == 0 {
				fmt.Fprintf(cobraCmd.OutOrStdout(), "No matching log files found in %s\n", logsDir)
				return nil
			}

			out := cobraCmd.OutOrStdout()
			// Show what will be removed before asking the user to confirm.
			if !yes && cmdutil.Interactive() {
				suffix := "s"
				if len(targets) == 1 {
					suffix = ""
				}
				fmt.Fprintln(out, opts.Color.Colorize(
					fmt.Sprintf("%sThis will permanently remove %d log file%s:%s",
						colors.SpecAttention, len(targets), suffix, colors.Reset)))
				printRemovalTable(out, targets)
			}
			if err := ui.ConfirmDeletion(
				yes, cmdutil.Interactive(), "", confirmToken(targets), out, opts,
			); err != nil {
				return err
			}

			for _, t := range targets {
				if err := os.Remove(t.path); err != nil {
					return fmt.Errorf("removing %s: %w", t.path, err)
				}
				fmt.Fprintf(out, "removed %s\n", t.path)
			}
			return nil
		},
	}

	constrictor.AttachArguments(command, constrictor.NoArgs)

	command.Flags().StringVar(&before, "before", "",
		"Remove logs created before this date or duration (e.g. '24h', '2026-01-01')")
	command.Flags().BoolVar(&all, "all", false, "Remove all log files")
	command.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")

	return command
}

func selectLogsToRemove(
	logsDir string, stackName, before string, all bool,
) ([]logEntry, error) {
	if !all && before == "" && stackName == "" {
		if !cmdutil.Interactive() {
			return nil, errors.New(
				"cannot prompt for a log file in non-interactive mode; " +
					"pass --all, --before, or --stack")
		}
		entry, err := chooseLogToRemove(logsDir)
		if err != nil {
			return nil, err
		}
		return []logEntry{entry}, nil
	}

	var beforeTime *time.Time
	if before != "" {
		var err error
		beforeTime, err = parseSince(before, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to parse --before as duration or timestamp: %w", err)
		}
	}

	entries, err := listLogs(logsDir)
	if err != nil {
		return nil, err
	}

	return filterLogs(entries, stackName, beforeTime), nil
}

func filterLogs(entries []logEntry, stackName string, beforeTime *time.Time) []logEntry {
	var matches []logEntry
	for _, e := range entries {
		if stackName != "" && e.stack != stackName {
			continue
		}
		if beforeTime != nil && !e.timestamp.Before(*beforeTime) {
			continue
		}
		matches = append(matches, e)
	}
	return matches
}

func confirmToken(targets []logEntry) string {
	var stack string
	for _, t := range targets {
		if t.stack == "" {
			return "yes"
		}
		if stack == "" {
			stack = t.stack
		} else if stack != t.stack {
			return "yes"
		}
	}
	if stack == "" {
		return "yes"
	}
	return stack
}

func chooseLogToRemove(logsDir string) (logEntry, error) {
	entries, err := listLogs(logsDir)
	if err != nil {
		return logEntry{}, err
	}
	if len(entries) == 0 {
		return logEntry{}, fmt.Errorf("no log files found in %s", logsDir)
	}

	options, optionMap := formatLogRemovalChoices(entries)

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message:  "Select a log file to remove:",
		Options:  options,
		PageSize: cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: len(options)}),
	}, &choice, ui.SurveyIcons(cmdutil.GetGlobalColorization())); err != nil {
		return logEntry{}, errors.New("no log file selected")
	}

	selected := optionMap[choice]
	for _, e := range entries {
		if e.path == selected {
			return e, nil
		}
	}
	return logEntry{}, errors.New("internal error: selected log not found")
}

func printRemovalTable(out io.Writer, entries []logEntry) {
	rows := slice.Prealloc[cmdutil.TableRow](len(entries))
	for _, e := range entries {
		stack := e.stackDisplay()
		updateID := e.updateID
		if updateID == "" {
			updateID = "—"
		}
		rows = append(rows, cmdutil.TableRow{
			Columns: []string{
				stack,
				humanize.Time(e.timestamp),
				updateID,
				humanize.Bytes(uint64(e.size)),
			},
		})
	}

	ui.FprintTable(out, cmdutil.Table{
		Headers: []string{"STACK", "CREATED", "UPDATE ID", "SIZE"},
		Rows:    rows,
	}, nil)
}

func formatLogRemovalChoices(entries []logEntry) ([]string, map[string]string) {
	type row struct {
		stack, created, updateID, path string
	}

	rows := make([]row, len(entries))
	stackWidth, createdWidth := 0, 0
	for i, e := range entries {
		stack := e.stackDisplay()
		updateID := e.updateID
		if updateID == "" {
			updateID = "—"
		}
		created := humanize.Time(e.timestamp)

		rows[i] = row{
			stack:    stack,
			created:  created,
			updateID: updateID,
			path:     e.path,
		}
		if l := len(stack); l > stackWidth {
			stackWidth = l
		}
		if l := len(created); l > createdWidth {
			createdWidth = l
		}
	}

	options := make([]string, len(rows))
	optionMap := make(map[string]string, len(rows))
	for i, r := range rows {
		// Append the index to disambiguate identical (stack, created,
		// updateID) rows that map to different files.
		line := fmt.Sprintf("%-*s  %-*s  %s", stackWidth, r.stack, createdWidth, r.created, r.updateID)
		if _, dup := optionMap[line]; dup {
			line = fmt.Sprintf("%s  [%d]", line, i+1)
		}
		options[i] = line
		optionMap[line] = r.path
	}
	return options, optionMap
}
