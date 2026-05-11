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

package cloud

import (
	"context"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// newLsCmd builds `pulumi cloud api list` — a stable, agent-first listing of
// every operation exposed by the embedded OpenAPI spec. api carries the
// parent command's persistent flags (--refresh-spec).
func newLsCmd(api *apiCommand) *cobra.Command {
	var format string
	includePreview := true
	includeDeprecated := false

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List every Pulumi Cloud API endpoint",
		Long: "List every endpoint exposed by the Pulumi Cloud OpenAPI spec.\n" +
			"\n" +
			"Output is sorted by (tag asc, path asc, method precedence). The default is a\n" +
			"human-readable table when interactive; when non-interactive, list switches to\n" +
			"the JSON envelope so downstream parsers don't have to deal with the table's\n" +
			"box-drawing characters. Pass --format=json to request JSON explicitly, or\n" +
			"--format=table to keep the table when redirecting.\n" +
			"\n" +
			"Preview endpoints are listed by default; deprecated endpoints are hidden. Use\n" +
			"--include-preview=false or --include-deprecated to change that.",
		Example: "  # Print the table of stable endpoints.\n" +
			"  pulumi cloud api list\n\n" +
			"  # Grab every operation as JSON (the default when piped).\n" +
			"  pulumi cloud api list --format=json\n\n" +
			"  # Count endpoints per tag with jq.\n" +
			"  pulumi cloud api list --format=json | jq '[.operations[] | .tag] | group_by(.) |\n" +
			"    map({tag: .[0], count: length})'\n\n" +
			"  # Find all stack-related GETs.\n" +
			"  pulumi cloud api list --format=json | jq '.operations[] |\n" +
			"    select(.method==\"GET\" and (.path | contains(\"/stacks\"))) | .operationId'\n\n" +
			"  # Full-text search descriptions for deployment-related endpoints.\n" +
			"  pulumi cloud api list --format=json | jq '.operations[] |\n" +
			"    select(.description | test(\"deployment\"; \"i\")) |\n" +
			"    {operationId, path, description}'\n\n" +
			"  # Include deprecated endpoints (hidden by default).\n" +
			"  pulumi cloud api list --include-deprecated\n\n" +
			"  # Hide preview endpoints.\n" +
			"  pulumi cloud api list --include-preview=false",
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	cmd.Flags().StringVar(&format, "format", "",
		"Output format: `table` (human-readable, default when interactive), `json` "+
			"(stable agent envelope, default when non-interactive). Use --format=table "+
			"to keep the table when redirecting.")
	cmd.Flags().BoolVar(&includePreview, "include-preview", true,
		"Include endpoints marked as preview")
	cmd.Flags().BoolVar(&includeDeprecated, "include-deprecated", false,
		"Include endpoints marked as deprecated")

	cmd.RunE = runWithEnvelope(func(cmd *cobra.Command, args []string) error {
		return runLs(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), format,
			includePreview, includeDeprecated, api.refreshSpec)
	})
	return cmd
}

func runLs(
	ctx context.Context,
	w, warnW io.Writer,
	format string,
	includePreview, includeDeprecated, refresh bool,
) error {
	mode, err := resolveOutput(format)
	if err != nil {
		return err
	}
	// list only renders table or JSON; resolveOutput accepts raw/markdown for
	// `describe` and the raw dispatcher, so reject them here explicitly.
	if mode == outputRaw || mode == outputMarkdown {
		return NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
			"--format="+format+" is not supported for list").
			WithField("format").
			WithSuggestions("--format=json", "--format=table")
	}

	if mode == outputDefault && !cmdutil.Interactive() {
		mode = outputJSON
	}

	idx, err := LoadIndex(ctx, warnW, refresh)
	if err != nil {
		return err
	}

	view := filterListedOps(idx, includePreview, includeDeprecated)

	if mode == outputJSON {
		return emitLsJSON(w, view)
	}
	return emitLsTable(w, view)
}

// filterListedOps returns a shallow Index view whose Operations slice has
// preview/deprecated rows excluded per the flags. Metadata (ByKey, SpecVersion)
// is reused from the source index — describe/dispatch still look up by key.
func filterListedOps(idx *Index, includePreview, includeDeprecated bool) *Index {
	filtered := make([]*Operation, 0, len(idx.Operations))
	for _, op := range idx.Operations {
		if op.IsDeprecated && !includeDeprecated {
			continue
		}
		if op.IsPreview && !includePreview {
			continue
		}
		filtered = append(filtered, op)
	}
	return &Index{
		Operations:  filtered,
		ByKey:       idx.ByKey,
		SpecVersion: idx.SpecVersion,
	}
}

func emitLsJSON(w io.Writer, idx *Index) error {
	ops := make([]lsOperation, 0, len(idx.Operations))
	for _, op := range idx.Operations {
		ops = append(ops, lsOperation{
			Method:      op.Method,
			Path:        op.Path,
			OperationID: op.OperationID,
			Summary:     op.Summary,
			Description: op.Description,
			Tag:         op.Tag,
			Preview:     op.IsPreview,
			Deprecated:  op.IsDeprecated,
		})
	}
	return WriteJSON(w, lsEnvelope{
		SchemaVersion: SchemaVersion,
		OrderedBy:     orderedByDesc,
		SpecVersion:   idx.SpecVersion,
		Count:         len(ops),
		Operations:    ops,
	}, cmdutil.Interactive())
}

// Column-sizing knobs for the list table.
const (
	lsMethodWidth  = 6  // every HTTP method we emit fits in 6 chars (DELETE).
	lsBorderWidth  = 13 // StyleLight: 1 outer border + 3 chars (sep + 2 padding) per column × 4 cols.
	lsMinPathWidth = 30 // floor for PATH so it stays legible in narrow terminals.
	// ceiling for SUMMARY
	lsSummaryHardMax = 60
	lsFallbackCols   = 120 // used when stdout isn't a terminal (e.g. piped to a file) but the user asked for a table.
)

// emitLsTable writes a human-readable table to w, with TAG as the leading
// column so operations visibly group by domain (matches the (tag, path,
// method) sort). Preview/deprecated markers are only exposed in the JSON
// envelope — keeping them out of the table avoids a column that's empty
// for the vast majority of rows.
//
// Column widths are computed from the actual data and the terminal width:
// TAG and SUMMARY get exactly what their longest value needs (no wasted
// padding), PATH absorbs the leftover so it wraps only when there isn't
// room to render it whole.
func emitLsTable(w io.Writer, idx *Index) error {
	maxTag, maxSummary, maxPath := 0, 0, 0
	for _, op := range idx.Operations {
		if n := utf8.RuneCountInString(op.Tag); n > maxTag {
			maxTag = n
		}
		if n := utf8.RuneCountInString(op.Summary); n > maxSummary {
			maxSummary = n
		}
		if n := utf8.RuneCountInString(op.Path); n > maxPath {
			maxPath = n
		}
	}
	summaryWidth := min(maxSummary, lsSummaryHardMax)
	cols := stdoutWidth(lsFallbackCols)
	pathWidth := max(lsMinPathWidth, min(maxPath, cols-maxTag-lsMethodWidth-summaryWidth-lsBorderWidth))

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"TAG", "METHOD", "PATH", "SUMMARY"})
	for _, op := range idx.Operations {
		t.AppendRow(table.Row{op.Tag, op.Method, op.Path, op.Summary})
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "METHOD", WidthMax: lsMethodWidth},
		{Name: "PATH", WidthMax: pathWidth, WidthMaxEnforcer: text.WrapText},
		{Name: "SUMMARY", WidthMax: summaryWidth, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()
	fmt.Fprintf(w, "\n%d operations. Pass --format=json for a stable, scriptable contract.\n",
		len(idx.Operations))
	return nil
}

// stdoutWidth reports the column count of stdout, falling back to fallback
// when stdout isn't a terminal or the size can't be determined (e.g. when
// the user piped --format=table to a file but still wants the table).
func stdoutWidth(fallback int) int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return fallback
	}
	return w
}
