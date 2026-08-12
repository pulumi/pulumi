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

package insights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// validSortFields enumerates the sort-field whitelist documented for the
// resource search v2 endpoint. Validating up front turns a typo into a clean
// CLI error instead of a wasted round-trip and a less-helpful server error.
var validSortFields = map[string]struct{}{
	"category":     {},
	"created":      {},
	"custom":       {},
	"delete":       {},
	"dependencies": {},
	"id":           {},
	"managed":      {},
	"modified":     {},
	"module":       {},
	"name":         {},
	"package":      {},
	"parentUrn":    {},
	"project":      {},
	"protected":    {},
	"providerUrn":  {},
	"stack":        {},
	"type":         {},
	"urn":          {},
}

// insightsResourceSearchClient is the subset of cloud-API operations the
// search command needs. Defined inside this package so unit tests can stub it
// without touching the full HTTP client surface.
type insightsResourceSearchClient interface {
	SearchInsightsResources(
		ctx context.Context, org string, params apitype.InsightsResourceSearchParams,
	) (apitype.InsightsResourceSearchResponse, error)
}

// searchClientFactory resolves the cloud client and the effective org for the
// call. orgOverride wins when non-empty; otherwise the default org from the
// cloud context is used.
type searchClientFactory func(
	ctx context.Context, orgOverride string,
) (insightsResourceSearchClient, string, error)

type insightsResourceSearchRenderFunc func(w io.Writer, r apitype.InsightsResourceSearchResponse) error

type insightsResourceSearchArgs struct {
	org          string
	query        string
	sort         []string
	asc          bool
	page         int
	size         int
	cursor       string
	properties   bool
	collapse     bool
	renderOutput insightsResourceSearchRenderFunc
}

type insightsResourceSearchCmd struct {
	clientFactory searchClientFactory
}

// newInsightsResourceSearchCmd builds the `pulumi insights resource search`
// command. factory produces the cloud client and resolves the effective org;
// pass nil to use the production factory backed by [cloud.ResolveContext].
func newInsightsResourceSearchCmd(factory searchClientFactory) *cobra.Command {
	if factory == nil {
		factory = defaultSearchClientFactory
	}

	search := &insightsResourceSearchCmd{clientFactory: factory}
	var args insightsResourceSearchArgs
	output := outputflag.OutputFlag[insightsResourceSearchRenderFunc]{
		RenderForTerminal: renderSearchTable,
		RenderJSON:        renderSearchJSON,
	}

	cmd := &cobra.Command{
		Use:   "search",
		Short: "[EXPERIMENTAL] Search for resources discovered by Pulumi Insights",
		Long: "[EXPERIMENTAL] Search resources discovered by Pulumi Insights across an\n" +
			"organization, with advanced filtering, sorting, and pagination.\n" +
			"\n" +
			"--query accepts the Pulumi query syntax. --sort takes one or more fields and\n" +
			"may be repeated; --asc flips the direction to ascending (default: descending).\n" +
			"--page selects a 1-based page up to 10,000 total results; beyond that use --cursor\n" +
			"with the token surfaced in a previous response (Enterprise plans only).\n" +
			"--properties=true asks the server to include each resource's input/output\n" +
			"values — requires a supported subscription. --collapse consolidates resources\n" +
			"that exist in multiple sources (e.g. an IaC stack and an Insights scan).\n" +
			"\n" +
			"Wraps the `GetOrgResourceSearchV2Query` Pulumi Cloud REST endpoint.",
		Example: "  # Find every S3 bucket the org has discovered.\n" +
			"  pulumi insights resource search --query 'type:aws:s3/bucket:Bucket'\n\n" +
			"  # Page through results 50 at a time, sorted by modification time.\n" +
			"  pulumi insights resource search --sort modified --page-size 50 --page 1\n\n" +
			"  # Continue from a cursor surfaced by a previous response.\n" +
			"  pulumi insights resource search --cursor <opaque-cursor>\n\n" +
			"  # JSON output for scripting.\n" +
			"  pulumi insights resource search --query 'aws:s3' --output json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			args.renderOutput = output.Get()
			return search.Run(cmd.Context(), cmd.OutOrStdout(), args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.org, "org", "",
		"Organization to search within (defaults to the current default org)")
	cmd.Flags().StringVarP(&args.query, "query", "q", "",
		"Search query in Pulumi query syntax")
	cmd.Flags().StringSliceVar(&args.sort, "sort", nil,
		"Field(s) to sort results by; repeat or comma-separate for multiple. "+
			"Allowed values: "+strings.Join(sortedSortFields(), ", "))
	cmd.Flags().BoolVar(&args.asc, "asc", false,
		"Sort in ascending order (default: descending)")
	cmd.Flags().IntVar(&args.page, "page", 0,
		"1-based page of results to return (max 10,000 total results)")
	cmd.Flags().IntVar(&args.size, "page-size", 0,
		"Number of results per page")
	cmd.Flags().StringVar(&args.cursor, "cursor", "",
		"Opaque cursor to continue pagination from (Enterprise plans only)")
	cmd.Flags().BoolVar(&args.properties, "properties", false,
		"Include resource input/output values (requires a supported subscription)")
	cmd.Flags().BoolVar(&args.collapse, "collapse", false,
		"Consolidate resources that exist in multiple sources into a single result")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}

// Run executes the search operation. ctx and out are decoupled from cobra so
// the function is straightforward to drive from tests.
func (c *insightsResourceSearchCmd) Run(
	ctx context.Context, out io.Writer, args insightsResourceSearchArgs,
) error {
	// --output is validated at flag-parse time by outputflag; validate --sort here
	// before talking to the network so a typo doesn't burn an API call.
	if err := validateSortFields(args.sort); err != nil {
		return err
	}

	client, org, err := c.clientFactory(ctx, args.org)
	if err != nil {
		return err
	}

	params := apitype.InsightsResourceSearchParams{
		Query:      args.query,
		Sort:       args.sort,
		Ascending:  args.asc,
		Page:       args.page,
		Size:       args.size,
		Cursor:     args.cursor,
		Properties: args.properties,
		Collapse:   args.collapse,
	}
	resp, err := client.SearchInsightsResources(ctx, org, params)
	if err != nil {
		return fmt.Errorf("searching insights resources: %w", err)
	}
	return args.renderOutput(out, resp)
}

// sortedSortFields returns the valid sort fields in deterministic order, so
// `--help` output is stable across builds.
func sortedSortFields() []string {
	fields := make([]string, 0, len(validSortFields))
	for f := range validSortFields {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	return fields
}

// validateSortFields rejects unknown --sort values up front. Error message
// lists allowed values so the user doesn't have to look them up.
func validateSortFields(sorts []string) error {
	for _, s := range sorts {
		if _, ok := validSortFields[s]; !ok {
			return fmt.Errorf("invalid --sort value %q (allowed: %s)",
				s, strings.Join(sortedSortFields(), ", "))
		}
	}
	return nil
}

// searchTableFallbackCols is the column count used when stdout isn't a TTY
// (e.g. piped to a file). Picked to match `pulumi stack webhook list`.
const searchTableFallbackCols = 120

// minURNColWidth keeps the URN column readable even on narrow terminals;
// the table will wrap rather than truncate beyond this point.
const minURNColWidth = 30

// fixedTableColsWidth is the budgeted width for everything except URN. The
// non-URN columns are short (type/stack/account/modified all fit well under
// 25 chars in practice), so 60 leaves comfortable headroom; what's left
// after subtracting borders goes to URN.
const fixedTableColsWidth = 60

// renderSearchTable writes a box-drawn table summary of the response to w
// using go-pretty's StyleLight, matching the other cli/cloud commands. Empty
// optional columns are dropped so a search that only returned IaC resources
// doesn't show an empty ACCOUNT column (and vice-versa for Insights-only).
// URN is given the remaining terminal width with wrapping so the table
// stays readable when piped to a narrow terminal.
func renderSearchTable(w io.Writer, r apitype.InsightsResourceSearchResponse) error {
	if len(r.Resources) == 0 {
		fmt.Fprintln(w, "No resources found.")
		return nil
	}

	type searchRow struct {
		urn, typ, stack, account, modified string
	}
	rows := make([]searchRow, len(r.Resources))
	var hasStack, hasAccount, hasModified bool
	for i, res := range r.Resources {
		// Fall back to <type>::<id> when the row doesn't carry a URN (some
		// Insights-only records lack one because they weren't deployed via
		// Pulumi). Keeps every row identifiable.
		identifier := res.URN
		if identifier == "" && res.Type != "" {
			identifier = res.Type + "::" + res.ID
		}
		rows[i] = searchRow{
			urn:      identifier,
			typ:      res.Type,
			stack:    res.Stack,
			account:  res.Account,
			modified: res.Modified,
		}
		hasStack = hasStack || res.Stack != ""
		hasAccount = hasAccount || res.Account != ""
		hasModified = hasModified || res.Modified != ""
	}

	header := table.Row{"URN", "TYPE"}
	if hasStack {
		header = append(header, "STACK")
	}
	if hasAccount {
		header = append(header, "ACCOUNT")
	}
	if hasModified {
		header = append(header, "MODIFIED")
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(header)
	for _, row := range rows {
		tr := table.Row{row.urn, row.typ}
		if hasStack {
			tr = append(tr, row.stack)
		}
		if hasAccount {
			tr = append(tr, row.account)
		}
		if hasModified {
			tr = append(tr, row.modified)
		}
		t.AppendRow(tr)
	}

	// Let the URN column absorb whatever width is left after the other
	// columns and the borders. 3 chars per column separator + 1 outer border
	// each side = 3*ncols + 1.
	cols := termWidth(w, searchTableFallbackCols)
	borderWidth := 3*len(header) + 1
	urnWidth := max(cols-borderWidth-fixedTableColsWidth, minURNColWidth)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "URN", WidthMax: urnWidth, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()

	fmt.Fprintf(w, "\nShowing %d of %d resources.\n", len(r.Resources), r.Total)
	if r.Pagination != nil && r.Pagination.Next != "" {
		if hint := paginationHint(r.Pagination.Next); hint != "" {
			fmt.Fprintf(w, "More results available. Re-run with %s to continue.\n", hint)
		} else {
			// Defensive: the spec promises `next` is a link with at least a
			// cursor or page query param. Fall back to printing the URL so
			// the user can still act on it.
			fmt.Fprintf(w, "More results available. Next page: %s\n", r.Pagination.Next)
		}
	}
	return nil
}

// termWidth reports the column count of w when it is a terminal, falling
// back to fallback otherwise (piped output, CI, non-file writers).
func termWidth(w io.Writer, fallback int) int {
	f, ok := w.(*os.File)
	if !ok {
		return fallback
	}
	cols, _, err := term.GetSize(int(f.Fd()))
	if err != nil || cols <= 0 {
		return fallback
	}
	return cols
}

// renderSearchJSON writes the full response as indented JSON.
func renderSearchJSON(w io.Writer, r apitype.InsightsResourceSearchResponse) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// paginationHint translates a pagination `next` link into the CLI flag(s) the
// user should pass to fetch the next page. The v2 endpoint exposes two
// pagination modes: a `cursor` token (Enterprise) and an offset-style `page`
// number. Cursor wins when present because it encapsulates all the state the
// server needs to resume. For the page case we also surface --page-size so
// the re-run is self-contained — the server's default size could otherwise
// drift across releases.
//
// Returns "" when the link parses but carries no recognised parameter, so the
// caller can fall back to printing the raw URL.
func paginationHint(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	q := u.Query()
	if cursor := q.Get("cursor"); cursor != "" {
		return fmt.Sprintf("--cursor %q", cursor)
	}
	if page := q.Get("page"); page != "" {
		hint := "--page " + page
		if size := q.Get("size"); size != "" {
			hint += " --page-size " + size
		}
		return hint
	}
	return ""
}

// defaultSearchClientFactory is the production wiring for searchClientFactory.
// It resolves the cloud context via cloud.ResolveContext and surfaces the
// *client.Client directly — *client.Client already satisfies
// insightsResourceSearchClient through its SearchInsightsResources method.
func defaultSearchClientFactory(
	ctx context.Context, orgOverride string,
) (insightsResourceSearchClient, string, error) {
	resolved, err := cloud.ResolveContext(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("resolving cloud context: %w", err)
	}
	if !resolved.LoggedIn {
		return nil, "", errors.New("not logged in to Pulumi Cloud; run `pulumi login` first")
	}

	org := orgOverride
	if org == "" {
		org = resolved.OrgName
	}
	if org == "" {
		return nil, "", errors.New(
			"no organization available; pass --org or set a default with `pulumi org set-default`")
	}

	return resolved.Client, org, nil
}
