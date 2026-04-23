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

package api

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	autotable "go.pennock.tech/tabular/auto"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// newLsCmd builds `pulumi cloud api ls` — a stable, agent-first listing of
// every operation exposed by the embedded OpenAPI spec.
func newLsCmd() *cobra.Command {
	var output, jq string
	includePreview := true
	includeDeprecated := false

	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List every Pulumi Cloud API endpoint",
		Long: "List every endpoint exposed by the Pulumi Cloud OpenAPI spec.\n\n" +
			"Output is sorted by (tag asc, path asc, method precedence). The default is\n" +
			"a human-readable table on a terminal; when stdout is piped to another\n" +
			"process, ls automatically switches to the JSON envelope so downstream\n" +
			"parsers don't trip on the table's box-drawing characters. Pass\n" +
			"--output=json to request JSON explicitly, or --output=table to keep the\n" +
			"table even when piped. Passing --jq implies --output=json.\n\n" +
			"Preview endpoints are listed by default; deprecated endpoints are hidden.\n" +
			"Use --include-preview=false or --include-deprecated to change that.",
		Example: "  # Print the table of stable endpoints.\n" +
			"  pulumi cloud api ls\n\n" +
			"  # Grab every operation as JSON (the default when piped).\n" +
			"  pulumi cloud api ls --output=json\n\n" +
			"  # Count endpoints per tag with jq.\n" +
			"  pulumi cloud api ls --jq '[.operations[] | .tag] | group_by(.) |\n" +
			"    map({tag: .[0], count: length})'\n\n" +
			"  # Find all stack-related GETs.\n" +
			"  pulumi cloud api ls --jq '.operations[] |\n" +
			"    select(.method==\"GET\" and (.path | contains(\"/stacks\"))) | .operationId'\n\n" +
			"  # Full-text search descriptions for deployment-related endpoints.\n" +
			"  pulumi cloud api ls --jq '.operations[] |\n" +
			"    select(.description | test(\"deployment\"; \"i\")) |\n" +
			"    {operationId, path, description}'\n\n" +
			"  # Include deprecated endpoints (hidden by default).\n" +
			"  pulumi cloud api ls --include-deprecated\n\n" +
			"  # Hide preview endpoints.\n" +
			"  pulumi cloud api ls --include-preview=false",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.Flags().StringVar(&output, "output", "",
		"Output format: `table` (human-readable, default on a TTY), `json` "+
			"(stable agent envelope, default when piped). Use --output=table "+
			"to keep the table when redirecting.")
	cmd.Flags().StringVar(&jq, "jq", "",
		"Filter the JSON envelope with a jq expression (implies --output=json)")
	cmd.Flags().BoolVar(&includePreview, "include-preview", true,
		"Include endpoints marked as preview")
	cmd.Flags().BoolVar(&includeDeprecated, "include-deprecated", false,
		"Include endpoints marked as deprecated")

	cmd.RunE = runWithEnvelope(func(cmd *cobra.Command, args []string) error {
		return runLs(cmd.Context(), output, jq,
			cmd.Flags().Changed("output"), includePreview, includeDeprecated, refreshSpecFlag(cmd))
	})
	return cmd
}

func runLs(
	ctx context.Context,
	output, jq string,
	outputSet, includePreview, includeDeprecated, refresh bool,
) error {
	mode, err := ResolveOutput(output)
	if err != nil {
		return err
	}
	// ls only renders table or JSON; ResolveOutput accepts raw/markdown for
	// `describe` and the raw dispatcher, so reject them here explicitly.
	if mode == OutputRaw || mode == OutputMarkdown {
		return NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
			"--output="+output+" is not supported for ls").
			WithField("output").
			WithSuggestions("--output=json", "--output=table")
	}
	mode, err = reconcileJQOutput(mode, jq, output, outputSet)
	if err != nil {
		return err
	}

	// Auto-switch to JSON when stdout is piped: the pretty table uses
	// box-drawing characters that break downstream parsers. Users who want
	// the table when piped can pass --output=table explicitly.
	if !outputSet && mode == OutputDefault && !stdoutIsTTY() {
		mode = OutputJSON
	}

	idx, err := LoadIndex(ctx, refresh)
	if err != nil {
		return err
	}

	view := filterListedOps(idx, includePreview, includeDeprecated)

	if mode == OutputJSON {
		return emitLsJSON(view, jq)
	}
	return emitLsTable(view)
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

func emitLsJSON(idx *Index, jq string) error {
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
	return writeJSONEnvelope(lsEnvelope{
		SchemaVersion: SchemaVersion,
		OrderedBy:     orderedByDesc,
		SpecVersion:   idx.SpecVersion,
		Count:         len(ops),
		Operations:    ops,
	}, jq, "ls")
}

// emitLsTable writes a human-readable table to stdout, with TAG as the
// leading column so operations visibly group by domain (matches the
// (tag, path, method) sort). Preview/deprecated markers are only exposed
// in the JSON envelope — keeping them out of the table avoids a column
// that's empty for the vast majority of rows.
func emitLsTable(idx *Index) error {
	table := autotable.New("utf8-heavy")
	table.AddHeaders("TAG", "METHOD", "PATH", "SUMMARY")
	for _, op := range idx.Operations {
		table.AddRowItems(op.Tag, op.Method, op.Path, truncateForTable(op.Summary, 60))
	}
	if errs := table.Errors(); errs != nil {
		return errors.Join(errs...)
	}
	if err := table.RenderTo(os.Stdout); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "\n%d operations. Pass --jq '<expr>' (or --output=json) for a stable, scriptable contract.\n",
		len(idx.Operations))
	return nil
}

// truncateForTable trims long strings so the table doesn't wrap catastrophically
// on narrow terminals. JSON output is untouched. Cuts on a rune boundary so
// multi-byte characters don't split into invalid UTF-8.
func truncateForTable(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "\u2026"
}
