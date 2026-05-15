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

// AI Generated - needs human review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyIssueListClient is the narrow subset of cloud-API operations the list
// command needs. Defined here so tests can stub a thin interface instead of
// the full HTTP client surface.
type policyIssueListClient interface {
	ListPolicyIssues(
		ctx context.Context, orgName string, opts client.ListPolicyIssuesOptions,
	) (apitype.ListPolicyIssuesResponse, error)
}

// policyIssueListClientFactory resolves a cloud client and the organization
// to list issues for. orgFlag carries the raw value of `--org` (empty means
// "use the default org").
type policyIssueListClientFactory func(
	ctx context.Context, orgFlag string,
) (policyIssueListClient, string, error)

// policyIssueListArgs collects the flag values for the list command, in one
// struct so Run can be driven directly from tests.
type policyIssueListArgs struct {
	org          string
	count        int64
	all          bool
	outputFormat outputflag.OutputFlag[policyIssueListRenderFunc]
}

// defaultPolicyIssueListOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultPolicyIssueListOutputFormat() outputflag.OutputFlag[policyIssueListRenderFunc] {
	return outputflag.OutputFlag[policyIssueListRenderFunc]{
		RenderForTerminal: renderPolicyIssueListTable,
		RenderJSON:        renderPolicyIssueListJSON,
	}
}

// newPolicyIssueListCmd builds `pulumi policy issue list` with the production
// client factory. The factory is overridable via newPolicyIssueListCmdWith for
// tests.
func newPolicyIssueListCmd() *cobra.Command {
	return newPolicyIssueListCmdWith(defaultPolicyIssueListClientFactory)
}

func newPolicyIssueListCmdWith(factory policyIssueListClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyIssueListClientFactory must not be nil")
	var args policyIssueListArgs
	args.outputFormat = defaultPolicyIssueListOutputFormat()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "[EXPERIMENTAL] List all policy issues for an organization",
		Long: "[EXPERIMENTAL] List all policy issues for an organization.\n" +
			"\n" +
			"Returns a list of policy issues for the organization. Each issue\n" +
			"represents a violation detected by a Policy Pack during a stack update\n" +
			"or a continuous-compliance scan, and includes the violating resource,\n" +
			"policy details, and enforcement level.\n" +
			"\n" +
			"Default output is a human-readable table; pass --output=json for the\n" +
			"full response as a JSON envelope.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPolicyIssueList(cmd.Context(), cmd.OutOrStdout(), factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.org, "org", "", "The organization to list policy issues for")
	cmd.Flags().Int64Var(&args.count, "count", 0,
		"Maximum number of issues to return. Defaults to the size of the first page; "+
			"larger values auto-paginate")
	cmd.Flags().BoolVar(&args.all, "all", false, "Return all matching issues; mutually exclusive with --count")
	cmd.MarkFlagsMutuallyExclusive("count", "all")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultPolicyIssueListClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the
// underlying *client.Client.
func defaultPolicyIssueListClientFactory(
	ctx context.Context, orgFlag string,
) (policyIssueListClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"listing policy issues requires the Pulumi Cloud backend; run `pulumi login`")
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

// runPolicyIssueList is the cobra-decoupled command body so tests can drive
// it directly without spinning up the flag parser.
func runPolicyIssueList(
	ctx context.Context, w io.Writer,
	factory policyIssueListClientFactory, args policyIssueListArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	// Fetch the first page using the server's default page size, then
	// continue paginating until we satisfy --count (if set) or exhaust the
	// list (when --all). When neither is set, return the first page.
	first, err := c.ListPolicyIssues(ctx, org, client.ListPolicyIssuesOptions{Page: 1})
	if err != nil {
		return fmt.Errorf("listing policy issues: %w", err)
	}

	issues := first.Issues
	total := first.Total
	pageSize := first.ItemsPerPage
	want := args.count
	all := args.all

	// Continue paginating if --all or if --count exceeds the first page size.
	if all || (want > int64(len(issues)) && pageSize > 0) {
		for page := int64(2); all || int64(len(issues)) < want; page++ {
			if total > 0 && int64(len(issues)) >= total {
				break
			}
			next, err := c.ListPolicyIssues(ctx, org, client.ListPolicyIssuesOptions{
				Page:     page,
				PageSize: pageSize,
			})
			if err != nil {
				return fmt.Errorf("listing policy issues: %w", err)
			}
			if len(next.Issues) == 0 {
				break
			}
			issues = append(issues, next.Issues...)
		}
	}

	// Trim to --count if it bounds the response shorter than what we fetched.
	if !all && want > 0 && int64(len(issues)) > want {
		issues = issues[:want]
	}

	return args.outputFormat.Get()(w, apitype.ListPolicyIssuesResponse{
		Issues: issues,
		Total:  total,
	})
}

type policyIssueListRenderFunc func(
	w io.Writer, resp apitype.ListPolicyIssuesResponse,
) error

// policyIssueMessageMax is the maximum width of the message column in the
// human-readable table. Longer messages are truncated with an ellipsis so the
// table stays readable in narrow terminals.
const policyIssueMessageMax = 60

// truncateMessage shortens s to at most max runes, appending "..." when the
// input would otherwise exceed the limit. max must be greater than the length
// of the ellipsis or the function returns s unchanged.
func truncateMessage(s string, max int) string {
	const ellipsis = "..."
	if max <= len(ellipsis) {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-len(ellipsis)]) + ellipsis
}

func renderPolicyIssueListTable(
	w io.Writer, resp apitype.ListPolicyIssuesResponse,
) error {
	if len(resp.Issues) == 0 {
		fmt.Fprintln(w, "No policy issues found for this organization.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"ID", "POLICY PACK", "POLICY", "ENFORCEMENT", "STACK", "MESSAGE"})

	for _, issue := range resp.Issues {
		pack := issue.PolicyPack
		if issue.PolicyPackTag != "" {
			pack = fmt.Sprintf("%s@%s", issue.PolicyPack, issue.PolicyPackTag)
		}
		stack := issue.EntityID
		if issue.EntityProject != "" && issue.EntityID != "" {
			stack = fmt.Sprintf("%s/%s", issue.EntityProject, issue.EntityID)
		}
		enforcement := issue.Level
		if enforcement == "" {
			enforcement = "-"
		}
		if stack == "" {
			stack = "-"
		}
		t.AppendRow(table.Row{
			issue.ID,
			pack,
			issue.PolicyName,
			enforcement,
			stack,
			truncateMessage(issue.Message, policyIssueMessageMax),
		})
	}
	t.Render()

	fmt.Fprintf(w, "\nShowing %d of %d policy issue(s)\n", len(resp.Issues), resp.Total)
	return nil
}

// policyIssueListEnvelope is the JSON shape emitted by
// `pulumi policy issue list --output=json`.
type policyIssueListEnvelope struct {
	Issues []apitype.PolicyIssue `json:"issues"`
	Total  int64                 `json:"total"`
}

func renderPolicyIssueListJSON(
	w io.Writer, resp apitype.ListPolicyIssuesResponse,
) error {
	issues := resp.Issues
	if issues == nil {
		issues = []apitype.PolicyIssue{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(policyIssueListEnvelope{
		Issues: issues,
		Total:  resp.Total,
	})
}
