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
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	backendlogging "github.com/pulumi/pulumi/pkg/v3/logging"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type lsRender func(c *lsCmd, entries []logEntry) error

type lsCmd struct {
	logsDir string
	w       io.Writer

	output outputflag.OutputFlag[lsRender]
}

func newListCmd() *cobra.Command {
	lc := &lsCmd{
		output: outputflag.OutputFlag[lsRender]{
			RenderForTerminal: (*lsCmd).renderTable,
			RenderJSON:        (*lsCmd).renderJSON,
		},
	}

	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List automatic log files",
		Long: "List automatic log files\n" +
			"\n" +
			"Each entry shows the stack the log belongs to, when it was\n" +
			"created, the associated update ID (or PID for CLI-level logs\n" +
			"that were never attached to a stack), the file size, and the\n" +
			"full path to the file.",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			logsDir, err := workspace.GetPulumiPath("logs")
			if err != nil {
				return fmt.Errorf("getting log directory: %w", err)
			}

			entries, err := listLogs(logsDir)
			if err != nil {
				return err
			}

			lc.logsDir = logsDir
			lc.w = cobraCmd.OutOrStdout()
			return lc.output.Get()(lc, entries)
		},
	}

	constrictor.AttachArguments(c, constrictor.NoArgs)
	outputflag.Var(c.Flags(), &lc.output)

	return c
}

func (c *lsCmd) renderTable(entries []logEntry) error {
	return printLogsTable(c.w, c.logsDir, entries)
}

func (c *lsCmd) renderJSON(entries []logEntry) error {
	return printLogsJSON(c.w, entries)
}

type logEntry struct {
	path      string    // absolute path to the file
	name      string    // basename
	stack     string    // decoded stack name; empty for CLI-level logs
	timestamp time.Time // timestamp parsed from the filename
	updateID  string    // update ID, or PID for CLI-level logs; may be empty
	cliLevel  bool      // true if the file was a CLI-level log (pulumi- prefix)
	size      int64
}

func listLogs(logsDir string) ([]logEntry, error) {
	dirEntries, err := os.ReadDir(logsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading log directory %s: %w", logsDir, err)
	}

	ownLog := backendlogging.CurrentLogFilePath()

	entries := slice.Prealloc[logEntry](len(dirEntries))
	for _, e := range dirEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		path := filepath.Join(logsDir, e.Name())
		if path == ownLog {
			continue
		}
		ts, ok := parseLogTimestamp(e.Name())
		if !ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		stack, updateID, cliLevel := parseLogName(e.Name())
		entries = append(entries, logEntry{
			path:      path,
			name:      e.Name(),
			stack:     stack,
			timestamp: ts,
			updateID:  updateID,
			cliLevel:  cliLevel,
			size:      info.Size(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].timestamp.After(entries[j].timestamp)
	})

	return entries, nil
}

func parseLogName(name string) (stack, updateID string, cliLevel bool) {
	loc := logTimestampRe.FindStringIndex(name)
	if loc == nil {
		return "", "", false
	}

	prefix := strings.TrimSuffix(name[:loc[0]], "-")
	suffix := strings.TrimSuffix(name[loc[1]:], ".log")
	suffix = strings.TrimPrefix(suffix, "-")

	if prefix == "pulumi" {
		return "", suffix, true
	}
	return strings.ReplaceAll(prefix, "+", "/"), suffix, false
}

func printLogsTable(out io.Writer, logsDir string, entries []logEntry) error {
	if len(entries) == 0 {
		fmt.Fprintf(out, "No log files found in %s\n", logsDir)
		return nil
	}

	rows := slice.Prealloc[cmdutil.TableRow](len(entries))
	for _, e := range entries {
		stack := e.stack
		if stack == "" {
			stack = "(cli)"
		}

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
				e.path,
			},
		})
	}

	ui.FprintTable(out, cmdutil.Table{
		Headers: []string{"STACK", "CREATED", "UPDATE ID", "SIZE", "FILE"},
		Rows:    rows,
	}, nil)

	return nil
}

type logEntryListJSON struct {
	File      string `json:"file"`
	Path      string `json:"path"`
	Stack     string `json:"stack,omitempty"`
	Timestamp string `json:"timestamp"`
	UpdateID  string `json:"updateId,omitempty"`
	PID       string `json:"pid,omitempty"`
	Size      int64  `json:"size"`
}

func printLogsJSON(out io.Writer, entries []logEntry) error {
	rows := slice.Prealloc[logEntryListJSON](len(entries))
	for _, e := range entries {
		row := logEntryListJSON{
			File:      e.name,
			Path:      e.path,
			Stack:     e.stack,
			Timestamp: cmd.FormatTime(e.timestamp.UTC()),
			Size:      e.size,
		}
		if e.cliLevel {
			row.PID = e.updateID
		} else {
			row.UpdateID = e.updateID
		}
		rows = append(rows, row)
	}
	return ui.FprintJSON(out, rows)
}
