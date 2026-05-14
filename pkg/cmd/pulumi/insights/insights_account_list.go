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
	"strconv"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// listPageLimit caps the number of pages we'll follow before bailing out, so a
// pathological server can't loop us forever. 1000 pages × the server-side max
// of 1000 covers a million accounts — many orders of magnitude beyond any real
// organization.
const listPageLimit = 1000

// insightsAccountListClient is the subset of cloud-API operations the list
// command needs.
type insightsAccountListClient interface {
	ListInsightsAccounts(
		ctx context.Context, org string, params apitype.ListInsightsAccountsParams,
	) (apitype.ListInsightsAccountsResponse, error)
}

// accountListClientFactory resolves the cloud client and the effective org for
// the call. orgOverride wins when non-empty; otherwise the default org from the
// cloud context is used.
type accountListClientFactory func(
	ctx context.Context, orgOverride string,
) (insightsAccountListClient, string, error)

// accountListRender is the function signature stored in the OutputFlag — one
// per supported output format. Sharing the signature keeps the renderer table
// in [defaultAccountListOutputFormat] readable.
type accountListRender func(*insightsAccountListCmd, io.Writer, []apitype.InsightsAccount) error

// defaultAccountListOutputFormat wires the output flag up with every supported
// format. Sharing this between the cobra constructor and the tests keeps the
// renderer table in one place.
func defaultAccountListOutputFormat() outputflag.OutputFlag[accountListRender] {
	return outputflag.OutputFlag[accountListRender]{
		RenderForTerminal: (*insightsAccountListCmd).renderTable,
		RenderJSON:        (*insightsAccountListCmd).renderJSON,
	}
}

type insightsAccountListArgs struct {
	org    string
	parent string
	roleID string
	// count and countSet form a ternary flag, per #22959: unset means
	// "one server page", --count 0 is a synonym for --all, --count N>0
	// means "at most N results".
	count    int
	countSet bool
	all      bool
	output   outputflag.OutputFlag[accountListRender]
}

type insightsAccountListCmd struct {
	clientFactory accountListClientFactory
}

// newInsightsAccountListCmd builds the `pulumi insights account list` command.
// factory produces the cloud client and resolves the effective org; pass nil to
// use the production factory backed by [cloud.ResolveContext].
func newInsightsAccountListCmd(factory accountListClientFactory) *cobra.Command {
	if factory == nil {
		factory = defaultAccountListClientFactory
	}

	list := &insightsAccountListCmd{clientFactory: factory}
	args := insightsAccountListArgs{output: defaultAccountListOutputFormat()}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List Insights accounts available to the authenticated user",
		Long: "[EXPERIMENTAL] List Pulumi Insights accounts within an organization.\n" +
			"\n" +
			"The organization defaults to the current default org and can be overridden\n" +
			"with --org. --parent restricts results to child accounts of the named parent\n" +
			"(e.g. an AWS Organizations management account). --role-id restricts results to\n" +
			"accounts accessible by a particular role.\n" +
			"\n" +
			"By default the command returns a single page of results. --count N returns at\n" +
			"most N results. --all (equivalent to --count 0) returns every matching\n" +
			"account. --count and --all are mutually exclusive.\n" +
			"\n" +
			"Wraps the `ListAccounts` Pulumi Cloud REST endpoint.",
		Example: "  # List the first page of Insights accounts in the default organization.\n" +
			"  pulumi insights account list\n\n" +
			"  # Return every matching account.\n" +
			"  pulumi insights account list --all\n\n" +
			"  # Return at most 250 accounts.\n" +
			"  pulumi insights account list --count 250\n\n" +
			"  # Filter to child accounts of an AWS Organizations management account.\n" +
			"  pulumi insights account list --parent aws-management\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi insights account list --output json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			args.countSet = cmd.Flags().Changed("count")
			return list.Run(cmd.Context(), cmd.OutOrStdout(), args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.org, "org", "",
		"Organization to list accounts for (defaults to the current default org)")
	cmd.Flags().StringVar(&args.parent, "parent", "",
		"Filter to child accounts of the named parent account")
	cmd.Flags().StringVar(&args.roleID, "role-id", "",
		"Filter to accounts accessible by the named role")
	cmd.Flags().IntVar(&args.count, "count", 0,
		"Return at most this many accounts (--count 0 is equivalent to --all)")
	cmd.Flags().BoolVar(&args.all, "all", false,
		"Return every matching account")
	cmd.MarkFlagsMutuallyExclusive("count", "all")
	outputflag.VarP(cmd.Flags(), &args.output)

	return cmd
}

func (c *insightsAccountListCmd) Run(
	ctx context.Context, out io.Writer, args insightsAccountListArgs,
) error {
	if args.countSet && args.count < 0 {
		return fmt.Errorf("--count must be non-negative, got %d", args.count)
	}

	client, org, err := c.clientFactory(ctx, args.org)
	if err != nil {
		return err
	}

	accounts, err := collectInsightsAccounts(ctx, client, org, args)
	if err != nil {
		return fmt.Errorf("listing insights accounts: %w", err)
	}
	return args.output.Get()(c, out, accounts)
}

// collectInsightsAccounts pulls accounts from the server, following pagination
// cursors when more than one page is required.
//
// limit caps the result set (0 = no cap). exhaust is true when the caller has
// asked for every matching account — either via --all or --count 0. Both are
// false in the default single-page mode. listPageLimit is a safety net against
// a misbehaving server reporting a non-empty cursor forever.
func collectInsightsAccounts(
	ctx context.Context,
	client insightsAccountListClient,
	org string,
	args insightsAccountListArgs,
) ([]apitype.InsightsAccount, error) {
	// Derive the two booleans the loop actually cares about.
	limit := 0
	exhaust := args.all
	if args.countSet {
		if args.count == 0 {
			// --count 0 is a synonym for --all (per #22959).
			exhaust = true
		} else {
			limit = args.count
		}
	}

	var (
		accounts []apitype.InsightsAccount
		cursor   string
	)
	for page := 0; page < listPageLimit; page++ {
		resp, err := client.ListInsightsAccounts(ctx, org, apitype.ListInsightsAccountsParams{
			ContinuationToken: cursor,
			Parent:            args.parent,
			RoleID:            args.roleID,
		})
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, resp.Accounts...)

		if resp.NextToken == "" {
			return accounts, nil
		}
		if !exhaust && limit == 0 {
			// Default single-page mode.
			return accounts, nil
		}
		if limit > 0 && len(accounts) >= limit {
			return accounts[:limit], nil
		}
		cursor = resp.NextToken
	}
	return nil, fmt.Errorf("pagination exceeded %d pages; the server may be looping", listPageLimit)
}

// renderTable writes a human-readable table view of the accounts. The chosen
// columns identify each row (Name/Provider/Owner) and surface the most useful
// operational context (Scheduled Scan, Last Scan, Resources). The full record
// is reachable via --output json.
func (c *insightsAccountListCmd) renderTable(w io.Writer, accounts []apitype.InsightsAccount) error {
	if len(accounts) == 0 {
		fmt.Fprintln(w, "No accounts found.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	// StyleLight upper-cases headers by default; mirror existing list commands
	// (e.g. `pulumi template list`) which preserve title case.
	t.Style().Format.Header = text.FormatDefault
	t.AppendHeader(table.Row{"Name", "Provider", "Owner", "Scheduled Scan", "Last Scan", "Resources"})
	for _, account := range accounts {
		scheduled := "no"
		if account.ScheduledScanEnabled {
			scheduled = "yes"
		}
		lastScan, resources := "-", "-"
		if account.ScanStatus != nil {
			if account.ScanStatus.Status != "" {
				lastScan = account.ScanStatus.Status
				// Append the finish time when available — a stale "succeeded" is
				// much less useful without the date attached.
				if account.ScanStatus.FinishedAt != nil && !account.ScanStatus.FinishedAt.IsZero() {
					lastScan = fmt.Sprintf("%s (%s)",
						account.ScanStatus.Status,
						account.ScanStatus.FinishedAt.UTC().Format("2006-01-02"))
				}
			}
			if account.ScanStatus.ResourceCount > 0 {
				resources = strconv.FormatInt(account.ScanStatus.ResourceCount, 10)
			}
		}
		t.AppendRow(table.Row{
			account.Name,
			account.Provider,
			account.OwnedBy.GitHubLogin,
			scheduled,
			lastScan,
			resources,
		})
	}
	t.Render()
	return nil
}

// renderJSON writes the accounts as an indented JSON envelope. Indentation
// matches the rest of the cli/cloud commands so jq-style scripting feels
// consistent.
func (c *insightsAccountListCmd) renderJSON(w io.Writer, accounts []apitype.InsightsAccount) error {
	// Make the JSON shape stable: an empty list serialises to `[]`, not `null`,
	// so consumers can iterate without a nil-check.
	if accounts == nil {
		accounts = []apitype.InsightsAccount{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Accounts []apitype.InsightsAccount `json:"accounts"`
	}{Accounts: accounts})
}

// defaultAccountListClientFactory is the production wiring for
// accountListClientFactory. It resolves the cloud context via
// cloud.ResolveContext and surfaces the *client.Client directly —
// *client.Client already satisfies insightsAccountListClient through its
// ListInsightsAccounts method.
func defaultAccountListClientFactory(
	ctx context.Context, orgOverride string,
) (insightsAccountListClient, string, error) {
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
