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
	org      string
	page     int64
	pageSize int64
	output   string
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

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "[EXPERIMENTAL] List all policy issues for an organization",
		Long: "[EXPERIMENTAL] List all policy issues for an organization.\n" +
			"\n" +
			"Returns a paginated list of policy issues for the organization. Each\n" +
			"issue represents a violation detected by a Policy Pack during a stack\n" +
			"update or a continuous-compliance scan, and includes the violating\n" +
			"resource, policy details, and enforcement level.\n" +
			"\n" +
			"Wraps the `ListPolicyIssues` Pulumi Cloud REST endpoint. Default output\n" +
			"is a human-readable table; pass --output=json for the full response as\n" +
			"a JSON envelope.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPolicyIssueList(cmd.Context(), cmd.OutOrStdout(), factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.org, "org", "", "The organization to list policy issues for")
	cmd.Flags().Int64Var(&args.page, "page", 1, "The page of results to return (min 1)")
	cmd.Flags().Int64Var(&args.pageSize, "page-size", 10, "The number of results per page")
	cmd.Flags().StringVarP(&args.output, "output", "o", "default",
		"Output format. One of: default, json")

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
	render, err := policyIssueListRenderer(args.output)
	if err != nil {
		return err
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	resp, err := c.ListPolicyIssues(ctx, org, client.ListPolicyIssuesOptions{
		Page:     args.page,
		PageSize: args.pageSize,
	})
	if err != nil {
		return fmt.Errorf("listing policy issues: %w", err)
	}

	return render(w, args, resp)
}

type policyIssueListRenderFunc func(
	w io.Writer, args policyIssueListArgs, resp apitype.ListPolicyIssuesResponse,
) error

func policyIssueListRenderer(format string) (policyIssueListRenderFunc, error) {
	switch format {
	case "", "default", "table":
		return renderPolicyIssueListTable, nil
	case "json":
		return renderPolicyIssueListJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

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
	w io.Writer, args policyIssueListArgs, resp apitype.ListPolicyIssuesResponse,
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
		pack := issue.PolicyPackName
		if issue.PolicyPackVersion != "" {
			pack = fmt.Sprintf("%s@%s", issue.PolicyPackName, issue.PolicyPackVersion)
		}
		stack := issue.StackName
		if issue.ProjectName != "" && issue.StackName != "" {
			stack = fmt.Sprintf("%s/%s", issue.ProjectName, issue.StackName)
		}
		enforcement := string(issue.EnforcementLevel)
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

	fmt.Fprintf(w, "\nShowing %d of %d policy issue(s)", len(resp.Issues), resp.Total)
	if args.page > 0 {
		fmt.Fprintf(w, " (page %d)", args.page)
	}
	fmt.Fprintln(w)
	return nil
}

// policyIssueListEnvelope is the JSON shape emitted by
// `pulumi policy issue list --output=json`. It mirrors the API response but
// adds the page number the client asked for, which the server doesn't echo
// back, so scripts can keep paginating without remembering their own state.
type policyIssueListEnvelope struct {
	Issues       []apitype.PolicyIssue `json:"issues"`
	Page         int64                 `json:"page"`
	ItemsPerPage int64                 `json:"itemsPerPage"`
	Total        int64                 `json:"total"`
}

func renderPolicyIssueListJSON(
	w io.Writer, args policyIssueListArgs, resp apitype.ListPolicyIssuesResponse,
) error {
	issues := resp.Issues
	if issues == nil {
		issues = []apitype.PolicyIssue{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(policyIssueListEnvelope{
		Issues:       issues,
		Page:         args.page,
		ItemsPerPage: resp.ItemsPerPage,
		Total:        resp.Total,
	})
}
