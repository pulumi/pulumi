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

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type complianceListClient interface {
	GetPolicyComplianceResults(
		ctx context.Context, orgName string, req apitype.GetPolicyComplianceResultsRequest,
	) (apitype.GetPolicyComplianceResultsResponse, error)
}

type complianceListClientFactory func(
	ctx context.Context, orgFlag string,
) (complianceListClient, string, error)

type complianceListRender func(
	cmd *complianceListCmd,
	columns []string, rows []apitype.PolicyComplianceResult,
) error

var validGroupByValues = []string{"stack", "account", "severity"}

type complianceListCmd struct {
	org     string
	groupBy string
	count   int
	output  outputflag.OutputFlag[complianceListRender]
	w       io.Writer
}

func newPolicyComplianceListCmd() *cobra.Command {
	return newPolicyComplianceListCmdWith(defaultComplianceListClientFactory)
}

func newPolicyComplianceListCmdWith(factory complianceListClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "complianceListClientFactory must not be nil")

	clcmd := &complianceListCmd{
		output: outputflag.OutputFlag[complianceListRender]{
			RenderForTerminal: (*complianceListCmd).renderTable,
			RenderJSON:        (*complianceListCmd).renderJSON,
		},
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[EXPERIMENTAL] List compliance results grouped by entity",
		Long: "[EXPERIMENTAL] List compliance results grouped by entity.\n" +
			"\n" +
			"Returns compliance results for policy issues grouped by stack,\n" +
			"cloud account, or severity. Each row shows a compliance score\n" +
			"(0-100%) per policy group/pack column.\n" +
			"\n" +
			"Score values: 0-100 = compliance %, N/A = not applicable,\n" +
			"ERR = configuration error.",
		Example: "  # List compliance by stack (default)\n" +
			"  pulumi policy compliance list\n\n" +
			"  # List compliance by severity\n" +
			"  pulumi policy compliance list --group-by severity\n\n" +
			"  # List compliance by account as JSON\n" +
			"  pulumi policy compliance list --group-by account --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			clcmd.w = cmd.OutOrStdout()
			return clcmd.run(cmd.Context(), factory)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&clcmd.org, "org", "",
		"The organization to fetch compliance results for")
	cmd.Flags().StringVar(&clcmd.groupBy, "group-by", "stack",
		"How to group results: stack, account, or severity")
	cmd.Flags().IntVar(&clcmd.count, "count", 0,
		"Maximum number of rows to display (default: all)")
	outputflag.VarP(cmd.Flags(), &clcmd.output)

	return cmd
}

func defaultComplianceListClientFactory(
	ctx context.Context, orgFlag string,
) (complianceListClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"listing compliance results requires the Pulumi Cloud backend; run `pulumi login`")
	}

	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return nil, "", err
	}

	org := orgFlag
	if org == "" {
		defaultOrg, err := cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return nil, "", err
		}
		org = defaultOrg
	}
	if org == "" {
		org = userName
	}
	if !slices.Contains(orgs, org) && org != userName {
		return nil, "", fmt.Errorf("user %s is not a member of organization %s", userName, org)
	}

	return cloudBackend.Client(), org, nil
}

func (c *complianceListCmd) run(ctx context.Context, factory complianceListClientFactory) error {
	if !slices.Contains(validGroupByValues, c.groupBy) {
		return fmt.Errorf(
			"invalid --group-by %q; must be \"stack\", \"account\", or \"severity\"",
			c.groupBy)
	}

	cl, org, err := factory(ctx, c.org)
	if err != nil {
		return err
	}

	// Fetch all pages.
	var allRows []apitype.PolicyComplianceResult
	var columns []string
	var continuationToken *string

	for {
		pageSize := 1000
		if c.count > 0 && c.count-len(allRows) < pageSize {
			pageSize = c.count - len(allRows)
		}
		size := pageSize

		resp, err := cl.GetPolicyComplianceResults(ctx, org,
			apitype.GetPolicyComplianceResultsRequest{
				Entity:            c.groupBy,
				ContinuationToken: continuationToken,
				Size:              &size,
			})
		if err != nil {
			return fmt.Errorf("listing compliance results: %w", err)
		}

		columns = resp.Columns
		allRows = append(allRows, resp.Rows...)

		if c.count > 0 && len(allRows) >= c.count {
			allRows = allRows[:c.count]
			break
		}
		if resp.ContinuationToken == nil || *resp.ContinuationToken == "" {
			break
		}
		continuationToken = resp.ContinuationToken
	}

	return c.output.Get()(c, columns, allRows)
}

func scoreString(score int) string {
	switch score {
	case -1:
		return "N/A"
	case -2:
		return "ERR"
	default:
		return strconv.Itoa(score) + "%"
	}
}

func (c *complianceListCmd) renderTable(
	columns []string, rows []apitype.PolicyComplianceResult,
) error {
	if len(rows) == 0 {
		fmt.Fprintln(c.w, "No compliance results found.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(c.w)
	t.SetStyle(table.StyleLight)

	header := table.Row{"ENTITY"}
	for _, col := range columns {
		header = append(header, col)
	}
	t.AppendHeader(header)

	for _, row := range rows {
		r := table.Row{row.EntityName}
		for _, score := range row.Scores {
			r = append(r, scoreString(score))
		}
		t.AppendRow(r)
	}

	// Score columns are short (e.g. "100%", "N/A"). Let the ENTITY column
	// and any long column headers wrap to fit the terminal.
	cols := cmdCmd.StdoutWidth()
	// Each score column needs ~6 chars + borders. Reserve the rest for ENTITY.
	scoreCols := len(columns)
	borders := 3*(scoreCols+1) + 1
	scoreWidth := 6 * scoreCols
	entityWidth := cols - borders - scoreWidth
	if entityWidth < 15 {
		entityWidth = 15
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "ENTITY", WidthMax: entityWidth, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()

	fmt.Fprintf(c.w, "\n%d result(s)\n", len(rows))
	return nil
}

type complianceJSONRow struct {
	Entity string         `json:"entity"`
	Scores map[string]int `json:"scores"`
}

func (c *complianceListCmd) renderJSON(
	columns []string, rows []apitype.PolicyComplianceResult,
) error {
	jsonRows := make([]complianceJSONRow, 0, len(rows))
	for _, row := range rows {
		scores := make(map[string]int, len(columns))
		for i, col := range columns {
			if i < len(row.Scores) {
				scores[col] = row.Scores[i]
			}
		}
		jsonRows = append(jsonRows, complianceJSONRow{
			Entity: row.EntityName,
			Scores: scores,
		})
	}
	enc := json.NewEncoder(c.w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Results []complianceJSONRow `json:"results"`
		Count   int                 `json:"count"`
	}{
		Results: jsonRows,
		Count:   len(jsonRows),
	})
}
