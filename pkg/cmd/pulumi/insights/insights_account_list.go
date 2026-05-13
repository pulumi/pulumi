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
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// listPageLimit caps the number of pages we'll follow before bailing out, so a
// pathological server can't loop us forever. 100 pages × 1000 (the server-side
// max page size) covers 100k accounts — more than any real organization has.
const listPageLimit = 100

// insightsAccountListClient is the subset of cloud-API operations the list
// command needs. Defined inside this package so unit tests can stub it without
// touching the full HTTP client surface.
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

type insightsAccountListArgs struct {
	org    string
	parent string
	roleID string
	output string
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
	var args insightsAccountListArgs

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
			"Results are paginated by the server; the command follows continuation tokens\n" +
			"internally so all matching accounts are streamed in a single invocation.\n" +
			"\n" +
			"Wraps the `ListAccounts` Pulumi Cloud REST endpoint.",
		Example: "  # List every Insights account in the default organization.\n" +
			"  pulumi insights account list\n\n" +
			"  # Filter to child accounts of an AWS Organizations management account.\n" +
			"  pulumi insights account list --parent aws-management\n\n" +
			"  # Restrict to accounts accessible by a particular role.\n" +
			"  pulumi insights account list --role-id 01HXXXX\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi insights account list --output json",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
	cmd.Flags().StringVarP(&args.output, "output", "o", "default",
		"Output format. One of: default, json")

	return cmd
}

// Run executes the list operation. ctx and out are decoupled from cobra so the
// function is straightforward to drive from tests.
func (c *insightsAccountListCmd) Run(
	ctx context.Context, out io.Writer, args insightsAccountListArgs,
) error {
	// Validate --output before talking to the network so a typo doesn't burn an
	// API call.
	render, err := accountListRenderer(args.output)
	if err != nil {
		return err
	}

	client, org, err := c.clientFactory(ctx, args.org)
	if err != nil {
		return err
	}

	accounts, err := collectInsightsAccounts(ctx, client, org, args)
	if err != nil {
		return fmt.Errorf("listing insights accounts: %w", err)
	}
	return render(out, accounts)
}

// collectInsightsAccounts follows the server-side continuationToken cursor
// until it's empty, accumulating every page into a single slice. Bailing out
// after listPageLimit pages prevents a misbehaving server from spinning us
// forever; in practice no org has that many accounts.
func collectInsightsAccounts(
	ctx context.Context,
	client insightsAccountListClient,
	org string,
	args insightsAccountListArgs,
) ([]apitype.InsightsAccount, error) {
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
		cursor = resp.NextToken
	}
	return nil, fmt.Errorf("pagination exceeded %d pages; the server may be looping", listPageLimit)
}

// accountListRenderer maps --output to the corresponding render function.
func accountListRenderer(format string) (
	func(io.Writer, []apitype.InsightsAccount) error, error,
) {
	switch format {
	case "", "default":
		return renderAccountsTable, nil
	case "json":
		return renderAccountsJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

// renderAccountsTable writes a human-readable table view of the accounts. The
// chosen columns identify each row (Name/Provider/Owner) and surface the most
// useful operational context (Scheduled Scan, Last Scan, Resources). The full
// record is reachable via --output json.
func renderAccountsTable(w io.Writer, accounts []apitype.InsightsAccount) error {
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

// renderAccountsJSON writes the accounts as an indented JSON envelope.
// Indentation matches the rest of the cli/cloud commands so jq-style scripting
// feels consistent.
func renderAccountsJSON(w io.Writer, accounts []apitype.InsightsAccount) error {
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
