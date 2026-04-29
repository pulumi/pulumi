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
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	autotable "go.pennock.tech/tabular/auto"

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
		Long: "List every endpoint exposed by the Pulumi Cloud OpenAPI spec. " +
			"Output is sorted by (tag asc, path asc, method precedence). The default " +
			"is a human-readable table on a terminal; when stdout is piped to another " +
			"process, list automatically switches to the JSON envelope so downstream " +
			"parsers don't trip on the table's box-drawing characters. Pass " +
			"--format=json to request JSON explicitly, or --format=table to keep the " +
			"table even when piped.\n\n" +
			"Preview endpoints are listed by default; deprecated endpoints are hidden. " +
			"Use --include-preview=false or --include-deprecated to change that.",
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

// emitLsTable writes a human-readable table to w, with TAG as the leading
// column so operations visibly group by domain (matches the (tag, path,
// method) sort). Preview/deprecated markers are only exposed in the JSON
// envelope — keeping them out of the table avoids a column that's empty
// for the vast majority of rows.
func emitLsTable(w io.Writer, idx *Index) error {
	table := autotable.New("utf8-heavy")
	table.AddHeaders("TAG", "METHOD", "PATH", "SUMMARY")
	for _, op := range idx.Operations {
		table.AddRowItems(op.Tag, op.Method, op.Path, op.Summary)
	}
	if errs := table.Errors(); errs != nil {
		return errors.Join(errs...)
	}
	if err := table.RenderTo(w); err != nil {
		return err
	}
	fmt.Fprintf(w, "\n%d operations. Pass --format=json for a stable, scriptable contract.\n",
		len(idx.Operations))
	return nil
}
