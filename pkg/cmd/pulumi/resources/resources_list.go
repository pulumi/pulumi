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

package resources

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// resourceRow is the canonical, machine-readable shape for a resource in the list output.
// Fields are stable: callers (including agents) may rely on their names. New fields may be
// added but existing fields will not be renamed or removed without a deprecation cycle.
type resourceRow struct {
	URN       string     `json:"urn" yaml:"urn"`
	Type      string     `json:"type" yaml:"type"`
	Name      string     `json:"name" yaml:"name"`
	Project   string     `json:"project" yaml:"project"`
	Stack     string     `json:"stack" yaml:"stack"`
	Status    string     `json:"status" yaml:"status"`
	Provider  string     `json:"provider,omitempty" yaml:"provider,omitempty"`
	Parent    string     `json:"parent,omitempty" yaml:"parent,omitempty"`
	ID        string     `json:"id,omitempty" yaml:"id,omitempty"`
	Created   *time.Time `json:"created,omitempty" yaml:"created,omitempty"`
	Modified  *time.Time `json:"modified,omitempty" yaml:"modified,omitempty"`
	Protected bool       `json:"protected" yaml:"protected"`
	Custom    bool       `json:"custom" yaml:"custom"`
	Tainted   bool       `json:"tainted" yaml:"tainted"`
	External  bool       `json:"external" yaml:"external"`
}

// listOutput wraps a page of resourceRow values with query metadata, mirroring the
// structure called out in the Infralake CLI Parity UX specification.
type listOutput struct {
	Resources  []resourceRow `json:"resources" yaml:"resources"`
	TotalCount int           `json:"total_count" yaml:"total_count"`
	Page       listPage      `json:"page" yaml:"page"`
	Query      listQuery     `json:"query" yaml:"query"`
}

type listPage struct {
	Offset int `json:"offset" yaml:"offset"`
	Limit  int `json:"limit" yaml:"limit"`
}

type listQuery struct {
	Filters listFilters `json:"filters" yaml:"filters"`
	Sort    listSort    `json:"sort" yaml:"sort"`
}

type listFilters struct {
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
	Project string `json:"project,omitempty" yaml:"project,omitempty"`
	Stack   string `json:"stack,omitempty" yaml:"stack,omitempty"`
	Status  string `json:"status,omitempty" yaml:"status,omitempty"`
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
}

type listSort struct {
	Field string `json:"field" yaml:"field"`
	Order string `json:"order" yaml:"order"`
}

// All columns available for table output, in their canonical order.
var allColumns = []string{
	"name", "type", "project", "stack", "status",
	"provider", "parent", "id", "urn", "created", "modified",
	"protected", "custom", "tainted", "external",
}

// Default columns shown when `--columns` is not specified.
var defaultColumns = []string{"name", "type", "status", "modified"}

type listArgs struct {
	stackName string
	output    string
	typeGlob  string
	nameGlob  string
	project   string
	stack     string
	status    string
	sortBy    string
	sortOrder string
	limit     int
	offset    int
	groupBy   string
	columns   string
	noColor   bool
	stdout    io.Writer
}

func newResourcesListCmd() *cobra.Command {
	args := listArgs{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List resources tracked in a stack's state",
		Long: "List resources tracked in a stack's state.\n" +
			"\n" +
			"By default, the command lists all resources in the currently selected stack. Use\n" +
			"`--stack` to target a different stack. Output is a formatted table when stdout is\n" +
			"a TTY and machine-readable JSON otherwise; either can be forced with `--output`.\n" +
			"\n" +
			"Filters mirror the AG Grid column filters in the Pulumi Console: glob patterns on\n" +
			"`--type` and `--name`, exact matches on `--project`, `--stack-filter` and\n" +
			"`--status`. Sorting, pagination, column selection and grouping are all supported\n" +
			"so that agents can reliably page through large stacks.\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi resources list\n" +
			"  pulumi resources list --output json\n" +
			"  pulumi resources list --type 'aws:s3/bucket:*' --sort-by modified --sort-order desc\n" +
			"  pulumi resources list --columns name,type,status --limit 50 --offset 100\n" +
			"  pulumi resources list --group-by type --output jsonl",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runResourcesList(cmd, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&args.stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVar(&args.output, "output", "",
		"Output format: table, json, jsonl, csv, yaml. Defaults to table when stdout is a TTY, json otherwise")
	cmd.Flags().StringVar(&args.typeGlob, "type", "",
		"Filter by resource type (supports glob patterns, e.g. 'aws:s3/bucket:*')")
	cmd.Flags().StringVar(&args.nameGlob, "name", "",
		"Filter by resource name (supports glob patterns)")
	cmd.Flags().StringVar(&args.project, "project", "",
		"Filter by project name (exact match)")
	cmd.Flags().StringVar(&args.stack, "stack-filter", "",
		"Filter by stack name (exact match, applies within the loaded snapshot)")
	cmd.Flags().StringVar(&args.status, "status", "",
		"Filter by status: active, pending-delete, pending-replacement, external")
	cmd.Flags().StringVar(&args.sortBy, "sort-by", "name",
		"Field to sort by: name, type, project, stack, status, created, modified")
	cmd.Flags().StringVar(&args.sortOrder, "sort-order", "asc",
		"Sort order: asc or desc")
	cmd.Flags().IntVar(&args.limit, "limit", 0,
		"Maximum number of resources to return (0 = no limit)")
	cmd.Flags().IntVar(&args.offset, "offset", 0,
		"Number of resources to skip (for pagination)")
	cmd.Flags().StringVar(&args.groupBy, "group-by", "",
		"Group results by a field (comma-separated, e.g. 'project,stack'). Adds ordering; does not nest output")
	cmd.Flags().StringVar(&args.columns, "columns", "",
		"Comma-separated list of columns to show in table/csv output. Available: "+strings.Join(allColumns, ","))
	cmd.Flags().BoolVar(&args.noColor, "no-color", false,
		"Disable ANSI color codes in table output")

	return cmd
}

func runResourcesList(cobraCmd *cobra.Command, args listArgs) error {
	ctx := cobraCmd.Context()
	if args.stdout == nil {
		args.stdout = os.Stdout
	}

	format, err := resolveOutputFormat(args.output, args.stdout)
	if err != nil {
		return err
	}

	columns, err := resolveColumns(args.columns)
	if err != nil {
		return err
	}

	sink := cmdutil.Diag()
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := cmdStack.RequireStack(
		ctx, sink, ws, cmdBackend.DefaultLoginManager,
		args.stackName, cmdStack.LoadOnly, opts,
	)
	if err != nil {
		return err
	}

	snap, err := s.Snapshot(ctx, secrets.DefaultProvider)
	if err != nil {
		return err
	}

	var rows []resourceRow
	if snap != nil {
		rows = buildRows(snap.Resources)
	}

	rows, err = filterRows(rows, args)
	if err != nil {
		return err
	}

	totalCount := len(rows)

	if err := sortRows(rows, args.sortBy, args.sortOrder, args.groupBy); err != nil {
		return err
	}

	rows = paginateRows(rows, args.offset, args.limit)

	output := listOutput{
		Resources:  rows,
		TotalCount: totalCount,
		Page:       listPage{Offset: args.offset, Limit: args.limit},
		Query: listQuery{
			Filters: listFilters{
				Type:    args.typeGlob,
				Project: args.project,
				Stack:   args.stack,
				Status:  args.status,
				Name:    args.nameGlob,
			},
			Sort: listSort{Field: args.sortBy, Order: args.sortOrder},
		},
	}

	return renderOutput(args.stdout, format, columns, output, args.noColor)
}

// buildRows converts deploy.Snapshot resources into the flat, presentation-ready row shape.
func buildRows(states []*resource.State) []resourceRow {
	rows := slice.Prealloc[resourceRow](len(states))
	for _, res := range states {
		if res == nil {
			continue
		}
		row := resourceRow{
			URN:       string(res.URN),
			Type:      string(res.Type),
			Name:      res.URN.Name(),
			Project:   string(res.URN.Project()),
			Stack:     string(res.URN.Stack()),
			Status:    statusFor(res),
			Provider:  res.Provider,
			Parent:    string(res.Parent),
			ID:        string(res.ID),
			Created:   res.Created,
			Modified:  res.Modified,
			Protected: res.Protect,
			Custom:    res.Custom,
			Tainted:   res.Taint,
			External:  res.External,
		}
		rows = append(rows, row)
	}
	return rows
}

func statusFor(res *resource.State) string {
	switch {
	case res.PendingReplacement:
		return "pending-replacement"
	case res.Delete:
		return "pending-delete"
	case res.External:
		return "external"
	default:
		return "active"
	}
}

func filterRows(rows []resourceRow, args listArgs) ([]resourceRow, error) {
	if args.typeGlob == "" && args.nameGlob == "" && args.project == "" &&
		args.stack == "" && args.status == "" {
		return rows, nil
	}

	// Validate globs up front so typos surface immediately.
	for _, g := range []string{args.typeGlob, args.nameGlob} {
		if g == "" {
			continue
		}
		if _, err := filepath.Match(g, ""); err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", g, err)
		}
	}

	out := rows[:0]
	for _, row := range rows {
		if args.typeGlob != "" && !globMatch(args.typeGlob, row.Type) {
			continue
		}
		if args.nameGlob != "" && !globMatch(args.nameGlob, row.Name) {
			continue
		}
		if args.project != "" && row.Project != args.project {
			continue
		}
		if args.stack != "" && row.Stack != args.stack {
			continue
		}
		if args.status != "" && row.Status != args.status {
			continue
		}
		out = append(out, row)
	}
	// Return a fresh slice so the underlying array doesn't pin the originals.
	return append([]resourceRow(nil), out...), nil
}

// globMatch runs filepath.Match but falls back to substring matching for patterns without
// glob metacharacters, so users can pass bare prefixes like "aws:ec2" without needing "*".
func globMatch(pattern, value string) bool {
	if !strings.ContainsAny(pattern, "*?[") {
		return strings.Contains(value, pattern)
	}
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		return false
	}
	return matched
}

func sortRows(rows []resourceRow, field, order, groupBy string) error {
	if field == "" {
		field = "name"
	}
	descending := false
	switch strings.ToLower(order) {
	case "", "asc", "ascending":
		descending = false
	case "desc", "descending":
		descending = true
	default:
		return fmt.Errorf("invalid --sort-order %q: must be asc or desc", order)
	}

	// Build the group key list (if any).
	var groupKeys []string
	if groupBy != "" {
		for _, k := range strings.Split(groupBy, ",") {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			if !isSortableField(k) {
				return fmt.Errorf("invalid --group-by field %q", k)
			}
			groupKeys = append(groupKeys, k)
		}
	}

	if !isSortableField(field) {
		return fmt.Errorf("invalid --sort-by field %q", field)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		for _, k := range groupKeys {
			a, b := fieldValue(rows[i], k), fieldValue(rows[j], k)
			if a != b {
				return a < b
			}
		}
		a, b := fieldValue(rows[i], field), fieldValue(rows[j], field)
		if descending {
			return a > b
		}
		return a < b
	})
	return nil
}

func isSortableField(field string) bool {
	switch field {
	case "name", "type", "project", "stack", "status",
		"urn", "provider", "parent", "id", "created", "modified":
		return true
	}
	return false
}

func fieldValue(row resourceRow, field string) string {
	switch field {
	case "name":
		return row.Name
	case "type":
		return row.Type
	case "project":
		return row.Project
	case "stack":
		return row.Stack
	case "status":
		return row.Status
	case "urn":
		return row.URN
	case "provider":
		return row.Provider
	case "parent":
		return row.Parent
	case "id":
		return row.ID
	case "created":
		if row.Created == nil {
			return ""
		}
		return row.Created.UTC().Format(time.RFC3339Nano)
	case "modified":
		if row.Modified == nil {
			return ""
		}
		return row.Modified.UTC().Format(time.RFC3339Nano)
	}
	return ""
}

func paginateRows(rows []resourceRow, offset, limit int) []resourceRow {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		return nil
	}
	rows = rows[offset:]
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	return rows
}

// resolveOutputFormat picks the effective output format. If `--output` is empty, it falls
// back to `table` when stdout is a TTY, and `json` otherwise. This is the core "agent-first"
// dual-mode behavior called out in the spec.
func resolveOutputFormat(flag string, stdout io.Writer) (string, error) {
	if flag != "" {
		switch strings.ToLower(flag) {
		case "table", "json", "jsonl", "csv", "yaml", "yml":
			f := strings.ToLower(flag)
			if f == "yml" {
				f = "yaml"
			}
			return f, nil
		default:
			return "", fmt.Errorf("invalid --output %q: must be one of table, json, jsonl, csv, yaml", flag)
		}
	}
	if stdout == os.Stdout && cmdutil.InteractiveTerminal() {
		return "table", nil
	}
	return "json", nil
}

func resolveColumns(flag string) ([]string, error) {
	if flag == "" {
		return defaultColumns, nil
	}
	parts := strings.Split(flag, ",")
	cols := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !isKnownColumn(p) {
			return nil, fmt.Errorf("unknown column %q: available columns are %s",
				p, strings.Join(allColumns, ", "))
		}
		cols = append(cols, p)
	}
	if len(cols) == 0 {
		return nil, errors.New("--columns must list at least one column")
	}
	return cols, nil
}

func isKnownColumn(c string) bool {
	for _, a := range allColumns {
		if a == c {
			return true
		}
	}
	return false
}

func renderOutput(
	stdout io.Writer, format string, columns []string, output listOutput, noColor bool,
) error {
	switch format {
	case "json":
		return ui.FprintJSON(stdout, output)
	case "jsonl":
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		for _, row := range output.Resources {
			if err := enc.Encode(row); err != nil {
				return err
			}
		}
		return nil
	case "yaml":
		enc := yaml.NewEncoder(stdout)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(output)
	case "csv":
		return renderCSV(stdout, columns, output.Resources)
	case "table":
		return renderTable(stdout, columns, output, noColor)
	}
	return fmt.Errorf("unsupported output format %q", format)
}

func renderCSV(stdout io.Writer, columns []string, rows []resourceRow) error {
	w := csv.NewWriter(stdout)
	if err := w.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, len(columns))
		for i, col := range columns {
			record[i] = cellValue(row, col)
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func renderTable(stdout io.Writer, columns []string, output listOutput, noColor bool) error {
	headers := make([]string, len(columns))
	for i, c := range columns {
		headers[i] = strings.ToUpper(c)
	}

	tableRows := slice.Prealloc[cmdutil.TableRow](len(output.Resources))
	for _, row := range output.Resources {
		cols := make([]string, len(columns))
		for i, c := range columns {
			cols[i] = cellValue(row, c)
		}
		tableRows = append(tableRows, cmdutil.TableRow{Columns: cols})
	}

	table := cmdutil.Table{Headers: headers, Rows: tableRows}

	if stdout == os.Stdout && !noColor {
		ui.PrintTable(table, nil)
	} else {
		fmt.Fprint(stdout, table.String())
	}

	// Footer summary helps humans understand pagination at a glance. Agents should use
	// structured output formats instead, so this does not appear in JSON/CSV/YAML.
	shown := len(output.Resources)
	total := output.TotalCount
	if shown != total || output.Page.Offset > 0 || output.Page.Limit > 0 {
		offset := output.Page.Offset
		if shown == 0 {
			fmt.Fprintf(stdout, "\nShowing 0 of %d resources\n", total)
		} else {
			fmt.Fprintf(stdout, "\nShowing %d-%d of %d resources\n",
				offset+1, offset+shown, total)
		}
	}
	return nil
}

func cellValue(row resourceRow, col string) string {
	switch col {
	case "name":
		return row.Name
	case "type":
		return row.Type
	case "project":
		return row.Project
	case "stack":
		return row.Stack
	case "status":
		return row.Status
	case "provider":
		return row.Provider
	case "parent":
		return row.Parent
	case "id":
		return row.ID
	case "urn":
		return row.URN
	case "created":
		if row.Created == nil {
			return ""
		}
		return row.Created.UTC().Format(time.RFC3339)
	case "modified":
		if row.Modified == nil {
			return ""
		}
		return row.Modified.UTC().Format(time.RFC3339)
	case "protected":
		return strconv.FormatBool(row.Protected)
	case "custom":
		return strconv.FormatBool(row.Custom)
	case "tainted":
		return strconv.FormatBool(row.Tainted)
	case "external":
		return strconv.FormatBool(row.External)
	}
	return ""
}
