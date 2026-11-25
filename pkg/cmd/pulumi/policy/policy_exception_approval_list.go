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
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
)

type policyExceptionApprovalListRenderFunc func(w, stderr io.Writer, requests []policyExceptionRequestSummary) error

type policyExceptionApprovalListArgs struct {
	org          string
	status       string
	outputFormat outputflag.OutputFlag[policyExceptionApprovalListRenderFunc]
}

func newPolicyExceptionApprovalListCmd(factory policyExceptionClientFactory) *cobra.Command {
	var args policyExceptionApprovalListArgs
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "[EXPERIMENTAL] List requests for policy exceptions",
		Long: "[EXPERIMENTAL] List requests for policy exceptions.\n" +
			"\n" +
			"Lists the organization's open requests for policy exceptions. Use --status to list\n" +
			"requests with another status.",
		Example: "  pulumi policy exception approval list\n" +
			"  pulumi policy exception approval list --status all --output json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPolicyExceptionApprovalList(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), factory, args)
		},
	}
	args.outputFormat = outputflag.OutputFlag[policyExceptionApprovalListRenderFunc]{
		RenderForTerminal: func(w, stderr io.Writer, requests []policyExceptionRequestSummary) error {
			return renderPolicyExceptionApprovalListTable(w, stderr, requests, args.status)
		},
		RenderJSON: func(w, _ io.Writer, requests []policyExceptionRequestSummary) error {
			return ui.FprintJSON(w, struct {
				ChangeRequests []policyExceptionRequestSummary `json:"changeRequests"`
			}{requests})
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	addPolicyExceptionApprovalOrgFlag(cmd, &args.org)
	cmd.Flags().StringVar(&args.status, "status", "open",
		"Only list requests with this status: open (draft, pending or ready), applied, closed, or all")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)
	return cmd
}

func matchesChangeRequestStatus(status, filter string) bool {
	switch filter {
	case "all":
		return true
	case "open":
		return status == changeRequestDraft || status == changeRequestPending || status == changeRequestReady
	default:
		return status == filter
	}
}

func runPolicyExceptionApprovalList(
	ctx context.Context, w, stderr io.Writer, factory policyExceptionClientFactory, args policyExceptionApprovalListArgs,
) error {
	if !slices.Contains([]string{"open", "applied", "closed", "all"}, args.status) {
		return cmdCmd.ConfigurationError{Message: fmt.Sprintf(
			"invalid --status %q: expected open, applied, closed or all", args.status)}
	}
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	requests := []policyExceptionRequestSummary{}
	token := ""
	for {
		page, next, err := c.changeRequests.ListChangeRequestsForEntity(ctx, org, policyExceptionsEntityType, token)
		if err := policyExceptionsError(err, org); err != nil {
			return fmt.Errorf("listing policy exception requests: %w", err)
		}
		for _, cr := range page {
			if !isPolicyExceptionRequest(cr) || !matchesChangeRequestStatus(cr.Status, args.status) {
				continue
			}
			draft, err := c.GetPolicyExceptionDraft(ctx, org, cr.ID)
			if err := policyExceptionsError(err, org); err != nil {
				return fmt.Errorf("getting the policy exception of change request %s: %w", cr.ID, err)
			}
			requests = append(requests, policyExceptionRequestSummary{cr, draft.CreatePolicyExceptionRequest})
		}
		if next == "" || next == token {
			break
		}
		token = next
	}
	return args.outputFormat.Get()(w, stderr, requests)
}

func renderPolicyExceptionApprovalListTable(
	w, stderr io.Writer, requests []policyExceptionRequestSummary, status string,
) error {
	if len(requests) == 0 {
		fmt.Fprintln(stderr, "No policy exception requests found.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	header := table.Row{"ID", "POLICY PACK"}
	if status != "open" {
		header = append(header, "STATUS")
	}
	t.AppendHeader(append(header, "POLICIES", "TARGETS", "AUTHOR", "REASON"))
	for _, r := range requests {
		var policies, targets []string
		for _, m := range r.Exception.Match {
			policies = append(policies, m.Policies...)
			targets = append(targets, m.Targets...)
		}
		slices.Sort(policies)
		slices.Sort(targets)
		row := table.Row{r.ID, r.Exception.PolicyPack}
		if status != "open" {
			row = append(row, r.Status)
		}
		t.AppendRow(append(row,
			strings.Join(slices.Compact(policies), ", "),
			len(slices.Compact(targets)),
			r.CreatedBy.GithubLogin,
			truncateMessage(r.Exception.Reason, policyIssueMessageMax),
		))
	}
	t.Render()
	return nil
}
