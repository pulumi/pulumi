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
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// scanListPageLimit caps the number of pages we'll follow before bailing out,
// so a pathological server can't loop us forever. See the matching constant
// listPageLimit in insights_account_list.go for the rationale.
const scanListPageLimit = 1000

// insightsAccountScanListClient is the subset of cloud-API operations the
// scan list command needs.
type insightsAccountScanListClient interface {
	ListInsightsAccountScans(
		ctx context.Context, org, account string, params apitype.ListInsightsAccountScansParams,
	) (apitype.ListInsightsAccountScansResponse, error)
}

// accountScanListClientFactory resolves the cloud client and the effective
// org for the call. orgOverride wins when non-empty; otherwise the default
// org from the cloud context is used.
type accountScanListClientFactory func(
	ctx context.Context, orgOverride string,
) (insightsAccountScanListClient, string, error)

// accountScanListRender is the function signature stored in the OutputFlag —
// one per supported output format.
type accountScanListRender func(*insightsAccountScanListCmd, io.Writer, []apitype.InsightsAccountScanStatus) error

func defaultAccountScanListOutputFormat() outputflag.OutputFlag[accountScanListRender] {
	return outputflag.OutputFlag[accountScanListRender]{
		RenderForTerminal: (*insightsAccountScanListCmd).renderTable,
		RenderJSON:        (*insightsAccountScanListCmd).renderJSON,
	}
}

type insightsAccountScanListArgs struct {
	org string
	// count and countSet form a ternary flag (see insights_account_list.go):
	// unset means "one server page", --count 0 is a synonym for --all,
	// --count N>0 means "at most N results".
	count    int
	countSet bool
	all      bool
	output   outputflag.OutputFlag[accountScanListRender]
}

type insightsAccountScanListCmd struct {
	clientFactory accountScanListClientFactory
}

// newInsightsAccountScanListCmd builds the `pulumi insights account scan list`
// command. factory produces the cloud client and resolves the effective org;
// pass nil to use the production factory backed by [cloud.ResolveContext].
func newInsightsAccountScanListCmd(factory accountScanListClientFactory) *cobra.Command {
	if factory == nil {
		factory = defaultScanListClientFactory
	}

	list := &insightsAccountScanListCmd{clientFactory: factory}
	args := insightsAccountScanListArgs{output: defaultAccountScanListOutputFormat()}

	cmd := &cobra.Command{
		Use:     "list <account>",
		Aliases: []string{"ls"},
		Short:   "[EXPERIMENTAL] List recent scans for an Insights account",
		Long: "[EXPERIMENTAL] List recent scans for a Pulumi Insights account.\n" +
			"\n" +
			"The positional argument is the Insights account. For parent accounts the\n" +
			"endpoint returns scans across every child account, so this is the\n" +
			"recommended way to discover scan IDs to feed into `account scan log`.\n" +
			"\n" +
			"By default the command returns a single page of results. --count N\n" +
			"returns at most N results. --all (equivalent to --count 0) returns every\n" +
			"matching scan. --count and --all are mutually exclusive.",
		Example: "  # List the most recent scans for an account.\n" +
			"  pulumi insights account scan list prod-aws\n\n" +
			"  # For a parent account, the result spans every child account.\n" +
			"  pulumi insights account scan list prod-aws\n\n" +
			"  # Narrow to a single child by passing its full account path.\n" +
			"  pulumi insights account scan list prod-aws/us-east-1\n\n" +
			"  # Return at most 25 scans.\n" +
			"  pulumi insights account scan list prod-aws --count 25\n\n" +
			"  # Return every scan.\n" +
			"  pulumi insights account scan list prod-aws --all\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi insights account scan list prod-aws --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.countSet = cmd.Flags().Changed("count")
			return list.Run(cmd.Context(), cmd.OutOrStdout(), posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "account"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&args.org, "org", "",
		"Organization that owns the Insights account (defaults to the current default org)")
	cmd.Flags().IntVar(&args.count, "count", 0,
		"Return at most this many scans (--count 0 is equivalent to --all)")
	cmd.Flags().BoolVar(&args.all, "all", false,
		"Return every matching scan")
	cmd.MarkFlagsMutuallyExclusive("count", "all")
	outputflag.Var(cmd.Flags(), &args.output)

	return cmd
}

// Run executes the list operation. ctx and out are decoupled from cobra so
// the function is straightforward to drive from tests.
func (c *insightsAccountScanListCmd) Run(
	ctx context.Context, out io.Writer, account string, args insightsAccountScanListArgs,
) error {
	if args.countSet && args.count < 0 {
		return fmt.Errorf("--count must be non-negative, got %d", args.count)
	}

	client, org, err := c.clientFactory(ctx, args.org)
	if err != nil {
		return err
	}

	scans, err := collectInsightsAccountScans(ctx, client, org, account, args)
	if err != nil {
		return fmt.Errorf("listing insights scans: %w", err)
	}
	return args.output.Get()(c, out, scans)
}

// collectInsightsAccountScans pulls scans from the server, following the
// continuation cursor when more than one page is required. The shape mirrors
// collectInsightsAccounts in insights_account_list.go.
func collectInsightsAccountScans(
	ctx context.Context,
	client insightsAccountScanListClient,
	org, account string,
	args insightsAccountScanListArgs,
) ([]apitype.InsightsAccountScanStatus, error) {
	limit := 0
	exhaust := args.all
	if args.countSet {
		if args.count == 0 {
			exhaust = true
		} else {
			limit = args.count
		}
	}

	var (
		scans  []apitype.InsightsAccountScanStatus
		cursor string
	)
	for page := 0; page < scanListPageLimit; page++ {
		resp, err := client.ListInsightsAccountScans(ctx, org, account, apitype.ListInsightsAccountScansParams{
			ContinuationToken: cursor,
		})
		if err != nil {
			return nil, err
		}
		scans = append(scans, resp.ScanStatuses...)

		if resp.ContinuationToken == "" {
			return scans, nil
		}
		if !exhaust && limit == 0 {
			// Default single-page mode.
			return scans, nil
		}
		if limit > 0 && len(scans) >= limit {
			return scans[:limit], nil
		}
		cursor = resp.ContinuationToken
	}
	return nil, fmt.Errorf("pagination exceeded %d pages; the server may be looping", scanListPageLimit)
}

// renderTable writes a human-readable view of the scans. The chosen columns
// identify each row (Scan ID, Account) and surface the most useful
// operational context (Status, Started, Finished, Duration).
func (c *insightsAccountScanListCmd) renderTable(
	w io.Writer, scans []apitype.InsightsAccountScanStatus,
) error {
	if len(scans) == 0 {
		fmt.Fprintln(w, "No scans found.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.Style().Format.Header = text.FormatDefault
	t.AppendHeader(table.Row{"Scan ID", "Account", "Status", "Started", "Duration"})
	for _, s := range scans {
		started := "-"
		if !s.StartedAt.IsZero() {
			started = s.StartedAt.UTC().Format(time.RFC3339)
		}
		// Duration is more useful than the finish timestamp at a glance —
		// Started + Duration tells you the same story without a redundant
		// column. For an in-flight scan we fall back to "-" instead of a
		// number computed from `now`, so a quick glance still distinguishes
		// completed runs from running ones.
		duration := "-"
		if s.FinishedAt != nil && !s.FinishedAt.IsZero() && !s.StartedAt.IsZero() {
			duration = s.FinishedAt.Sub(s.StartedAt).Round(time.Second).String()
		}
		t.AppendRow(table.Row{
			s.ID,
			s.AccountName,
			s.Status,
			started,
			duration,
		})
	}
	t.Render()
	return nil
}

// renderJSON writes the scans as indented JSON. Wrapping in a `{"scans": ...}`
// envelope leaves room to add pagination metadata later without a breaking
// change, matching the shape used by `account list`.
func (c *insightsAccountScanListCmd) renderJSON(
	w io.Writer, scans []apitype.InsightsAccountScanStatus,
) error {
	if scans == nil {
		scans = []apitype.InsightsAccountScanStatus{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Scans []apitype.InsightsAccountScanStatus `json:"scans"`
	}{Scans: scans})
}

func defaultScanListClientFactory(
	ctx context.Context, orgOverride string,
) (insightsAccountScanListClient, string, error) {
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
