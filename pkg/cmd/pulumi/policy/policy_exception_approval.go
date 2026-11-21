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
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

const (
	// policyExceptionsEntityType is the entity type of the change requests that create policy exceptions.
	policyExceptionsEntityType = "policy_exceptions"
	changeRequestPending       = "pending"
	changeRequestClosed        = "closed"
)

// policyExceptionRequest is a change request that creates a policy exception, with the exception it creates.
type policyExceptionRequest struct {
	*esc_client.GetChangeRequestResponse
	Exception cloudapitype.CreatePolicyExceptionRequest `json:"exception"`
	// ExceptionID is the exception the request created, once it's applied.
	ExceptionID string `json:"exceptionId,omitempty"`
	URL         string `json:"url,omitempty"`
}

// policyExceptionRequestSummary is a listed change request, with the exception it creates.
type policyExceptionRequestSummary struct {
	esc_client.ChangeRequest
	Exception cloudapitype.CreatePolicyExceptionRequest `json:"exception"`
}

func isPolicyExceptionRequest(cr esc_client.ChangeRequest) bool {
	return cr.Entity.EntityType == policyExceptionsEntityType && cr.Action == "create"
}

func newPolicyExceptionApprovalCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approval",
		Short: "[EXPERIMENTAL] Review requests for policy exceptions",
		Long: "[EXPERIMENTAL] Review requests for policy exceptions.\n" +
			"\n" +
			"When an organization requires approval for policy exceptions, `pulumi policy exception new`\n" +
			"requests the exception in a change request. Once the request has the approvals it needs,\n" +
			"it's applied, which creates the exception.",
		Example: "  pulumi policy exception approval list\n" +
			"  pulumi policy exception approval get <change-request-id>\n" +
			"  pulumi policy exception approval approve <change-request-id> -m \"Looks good\"\n" +
			"  pulumi policy exception approval close <change-request-id> -m \"Fix the bucket instead\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	factory := newPolicyExceptionClientFactory(ws, lm)
	cmd.AddCommand(newPolicyExceptionApprovalListCmd(factory))
	cmd.AddCommand(newPolicyExceptionApprovalGetCmd(factory))
	cmd.AddCommand(newPolicyExceptionApprovalApproveCmd(factory))
	cmd.AddCommand(newPolicyExceptionApprovalUnapproveCmd(factory))
	cmd.AddCommand(newPolicyExceptionApprovalCloseCmd(factory))
	cmd.AddCommand(newPolicyExceptionApprovalCommentCmd(factory))
	cmd.AddCommand(newPolicyExceptionApprovalLogCmd(factory))
	return cmd
}

// changeRequestIDArgument is the single argument of the commands that act on one request.
var changeRequestIDArgument = &constrictor.Arguments{
	Arguments: []constrictor.Argument{{Name: "change-request-id"}},
	Required:  1,
}

func addPolicyExceptionApprovalOrgFlag(cmd *cobra.Command, org *string) {
	cmd.Flags().StringVar(org, "org", "", "The organization that owns the request")
}

// getPolicyExceptionRequest reads a change request and the exception it creates. It fails if the change request
// doesn't create a policy exception.
func getPolicyExceptionRequest(
	ctx context.Context, c policyExceptionClients, org, id string,
) (*policyExceptionRequest, error) {
	cr, err := c.changeRequests.GetChangeRequest(ctx, org, id)
	err = policyExceptionsError(err, org)
	if apiErr, ok := errors.AsType[*apitype.ErrorResponse](err); ok && apiErr.Code == http.StatusNotFound {
		return nil, fmt.Errorf("policy exception request %s not found in %s", id, org)
	} else if err != nil {
		return nil, err
	}
	if !isPolicyExceptionRequest(cr.ChangeRequest) {
		return nil, fmt.Errorf("change request %s isn't a policy exception request; "+
			"use `pulumi env approval` for environment change requests", id)
	}
	draft, err := c.GetPolicyExceptionDraft(ctx, org, id)
	if err := policyExceptionsError(err, org); err != nil {
		return nil, fmt.Errorf("getting the policy exception of change request %s: %w", id, err)
	}
	r := &policyExceptionRequest{
		GetChangeRequestResponse: cr,
		Exception:                draft.CreatePolicyExceptionRequest,
		URL:                      policyExceptionConsoleURL(c.URL(), org, draft.Match, id),
	}
	if cr.Status == changeRequestApplied {
		if r.ExceptionID, err = findRequestedPolicyException(ctx, c, org, r.Exception.PolicyPack, id); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// findRequestedPolicyException returns the ID of the exception that a change request created, or "" if there's none.
func findRequestedPolicyException(
	ctx context.Context, c policyExceptionClients, org, pack, changeRequestID string,
) (string, error) {
	opts := client.ListPolicyExceptionsOptions{PolicyPack: pack, Status: []string{"active", "removed"}}
	for {
		resp, err := c.ListPolicyExceptions(ctx, org, opts)
		if err := policyExceptionsError(err, org); err != nil {
			return "", fmt.Errorf("finding the exception created by change request %s: %w", changeRequestID, err)
		}
		for _, e := range resp.Exceptions {
			if e.ChangeRequestID != nil && *e.ChangeRequestID == changeRequestID {
				return e.ID, nil
			}
		}
		if resp.ContinuationToken == nil {
			return "", nil
		}
		opts.ContinuationToken = *resp.ContinuationToken
	}
}

// gateLines describes each gate of a request, and its approvals, as `pulumi env approval get` does.
func gateLines(gates esc_client.ChangeRequestGateEvaluation) []string {
	lines := make([]string, 0, len(gates.ApplicableGates))
	for _, g := range gates.ApplicableGates {
		state := "satisfied"
		if !g.Satisfied {
			state = "not satisfied"
		}
		line := fmt.Sprintf("%s (%s)", g.Name, state)
		if g.RuleDetails.RuleType == "approval_required" {
			approvers := make([]string, len(g.RuleDetails.Approvers))
			for i, a := range g.RuleDetails.Approvers {
				approvers[i] = a.GithubLogin
			}
			line += fmt.Sprintf(": %d of %d approvals", len(approvers), g.RuleDetails.RequiredApprovals)
			if len(approvers) != 0 {
				line += " (" + strings.Join(approvers, ", ") + ")"
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// warnNoop warns that a command did nothing, as `pulumi env approval` does.
func warnNoop(stderr io.Writer, msg, comment string) {
	if comment != "" {
		msg += " The comment was not posted."
	}
	fmt.Fprint(stderr, cmdutil.GetGlobalColorization().Colorize(
		fmt.Sprintf("%swarning: %s%s\n", colors.SpecWarning, msg, colors.Reset)))
}

// --- list ---

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

// --- get ---

type policyExceptionApprovalGetRenderFunc func(w io.Writer, r *policyExceptionRequest) error

type policyExceptionApprovalGetArgs struct {
	org          string
	outputFormat outputflag.OutputFlag[policyExceptionApprovalGetRenderFunc]
}

func newPolicyExceptionApprovalGetCmd(factory policyExceptionClientFactory) *cobra.Command {
	args := policyExceptionApprovalGetArgs{
		outputFormat: outputflag.OutputFlag[policyExceptionApprovalGetRenderFunc]{
			RenderForTerminal: renderPolicyExceptionRequest,
			RenderJSON: func(w io.Writer, r *policyExceptionRequest) error {
				return ui.FprintJSON(w, r)
			},
		},
	}
	cmd := &cobra.Command{
		Use:   "get <change-request-id>",
		Short: "[EXPERIMENTAL] Show a request for a policy exception",
		Long: "[EXPERIMENTAL] Show a request for a policy exception.\n" +
			"\n" +
			"Shows the request's status and approvals, and the exception it creates.",
		Example: "  pulumi policy exception approval get <change-request-id>\n" +
			"  pulumi policy exception approval get <change-request-id> --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			c, org, err := factory(cmd.Context(), args.org)
			if err != nil {
				return err
			}
			r, err := getPolicyExceptionRequest(cmd.Context(), c, org, posArgs[0])
			if err != nil {
				return err
			}
			return args.outputFormat.Get()(cmd.OutOrStdout(), r)
		},
	}

	constrictor.AttachArguments(cmd, changeRequestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &args.org)
	outputflag.VarP(cmd.Flags(), &args.outputFormat)
	return cmd
}

func renderPolicyExceptionRequest(w io.Writer, r *policyExceptionRequest) error {
	fmt.Fprintf(w, "%-14s %s\n", "ID:", r.ID)
	fmt.Fprintf(w, "%-14s %s\n", "Policy pack:", r.Exception.PolicyPack)
	fmt.Fprintf(w, "%-14s %s\n", "Status:", r.Status)
	if r.ExceptionID != "" {
		fmt.Fprintf(w, "%-14s %s\n", "Exception:", r.ExceptionID)
	}
	fmt.Fprintf(w, "%-14s %s by %s\n", "Requested:", r.CreatedAt.UTC().Format(time.RFC3339), r.CreatedBy.GithubLogin)
	if r.Exception.ExpiresInSeconds != nil {
		fmt.Fprintf(w, "%-14s %s after it's created\n", "Expires:", policyExceptionExpiresIn(*r.Exception.ExpiresInSeconds))
	}
	fmt.Fprintf(w, "%-14s %t\n", "Approved:", r.GateEvaluation.Satisfied)
	for _, line := range gateLines(r.GateEvaluation) {
		fmt.Fprintf(w, "  %s\n", line)
	}
	if r.URL != "" {
		fmt.Fprintf(w, "%-14s %s\n", "URL:", r.URL)
	}

	fmt.Fprintln(w, "\nPolicy Pack exceptions configuration:")
	return ui.FprintJSON(w, policyExceptionConfig(r.Exception))
}

// --- approve and close ---

type policyExceptionApprovalDecisionArgs struct {
	org     string
	comment string
	yes     bool
	dryRun  bool
}

func newPolicyExceptionApprovalApproveCmd(factory policyExceptionClientFactory) *cobra.Command {
	var args policyExceptionApprovalDecisionArgs
	cmd := &cobra.Command{
		Use:   "approve <change-request-id>",
		Short: "[EXPERIMENTAL] Approve a request for a policy exception",
		Long: "[EXPERIMENTAL] Approve a request for a policy exception.\n" +
			"\n" +
			"Shows the exception the request creates and approves the request's latest revision. Once\n" +
			"the request has the approvals it needs, it's applied, which creates the exception, and the\n" +
			"exception's ID is printed. You will be prompted to confirm unless --yes is passed.",
		Example: "  pulumi policy exception approval approve <change-request-id> -m \"Looks good\"\n" +
			"  pulumi policy exception approval approve <change-request-id> --dry-run",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.yes = args.yes || env.SkipConfirmations.Value()
			return runPolicyExceptionApprovalApprove(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, changeRequestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &args.org)
	cmd.Flags().StringVarP(&args.comment, "message", "m", "", "A comment to add with the approval")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false, "Skip the confirmation prompt")
	cmd.Flags().BoolVar(&args.dryRun, "dry-run", false, "Show the request without approving it")
	return cmd
}

// confirmPolicyExceptionDecision shows an open request and asks whether to approve or close it. It returns false
// with --dry-run.
func confirmPolicyExceptionDecision(
	stderr io.Writer, r *policyExceptionRequest, verb string, args policyExceptionApprovalDecisionArgs,
) (bool, error) {
	if r.Status == changeRequestApplied || r.Status == changeRequestClosed {
		return false, fmt.Errorf("policy exception request %s is %s", r.ID, r.Status)
	}
	fmt.Fprintf(stderr, "Policy exception request %s will create a policy exception for Policy Pack %s.\n\n",
		r.ID, r.Exception.PolicyPack)
	if err := renderPolicyExceptionRequest(stderr, r); err != nil {
		return false, err
	}
	fmt.Fprintln(stderr)
	if args.dryRun {
		return false, nil
	}
	if args.yes {
		return true, nil
	}
	if !cmdutil.Interactive() {
		return false, backenderr.ErrNonInteractiveRequiresYes
	}
	response, err := ui.PromptUserErr(fmt.Sprintf("Do you want to %s this request?", verb),
		[]string{"yes", "no"}, "no", cmdutil.GetGlobalColorization())
	if err != nil {
		return false, fmt.Errorf("confirmation cancelled: %w", err)
	}
	if response != "yes" {
		return false, errors.New("confirmation declined")
	}
	return true, nil
}

func runPolicyExceptionApprovalApprove(
	ctx context.Context, stdout, stderr io.Writer,
	factory policyExceptionClientFactory, id string, args policyExceptionApprovalDecisionArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}
	r, err := getPolicyExceptionRequest(ctx, c, org, id)
	if err != nil {
		return err
	}
	if ok, err := confirmPolicyExceptionDecision(stderr, r, "approve", args); !ok {
		return err
	}

	if err := c.changeRequests.ApproveChangeRequest(ctx, org, id, r.LatestRevisionNumber, args.comment); err != nil {
		return fmt.Errorf("approving policy exception request: %w", err)
	}
	fmt.Fprintf(stderr, "Approved policy exception request %s\n", id)
	// The request is applied, creating the exception, as soon as it has the approvals it needs.
	if r, err = getPolicyExceptionRequest(ctx, c, org, id); err != nil {
		return err
	}
	switch {
	case r.ExceptionID != "":
		fmt.Fprintln(stdout, r.ExceptionID)
		fmt.Fprintf(stderr, "Created policy exception %s\n", r.ExceptionID)
	case r.Status == changeRequestReady:
		fmt.Fprintln(stderr, "The request is pending to be applied.")
	}
	return nil
}

func newPolicyExceptionApprovalCloseCmd(factory policyExceptionClientFactory) *cobra.Command {
	var args policyExceptionApprovalDecisionArgs
	cmd := &cobra.Command{
		Use:   "close <change-request-id>",
		Short: "[EXPERIMENTAL] Close a request for a policy exception without creating it",
		Long: "[EXPERIMENTAL] Close a request for a policy exception without creating it.\n" +
			"\n" +
			"You will be prompted to confirm unless --yes is passed.",
		Example: "  pulumi policy exception approval close <change-request-id> -m \"Fix the bucket instead\"",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.yes = args.yes || env.SkipConfirmations.Value()
			return runPolicyExceptionApprovalClose(cmd.Context(), cmd.ErrOrStderr(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, changeRequestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &args.org)
	cmd.Flags().StringVarP(&args.comment, "message", "m", "", "Why the request is closed")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

func runPolicyExceptionApprovalClose(
	ctx context.Context, stderr io.Writer,
	factory policyExceptionClientFactory, id string, args policyExceptionApprovalDecisionArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}
	r, err := getPolicyExceptionRequest(ctx, c, org, id)
	if err != nil {
		return err
	}
	if r.Status == changeRequestClosed {
		warnNoop(stderr, fmt.Sprintf("Policy exception request %s is already closed.", id), args.comment)
		return nil
	}
	if ok, err := confirmPolicyExceptionDecision(stderr, r, "close", args); !ok {
		return err
	}
	if err := c.changeRequests.CloseChangeRequest(ctx, org, id, args.comment); err != nil {
		return fmt.Errorf("closing policy exception request: %w", err)
	}
	fmt.Fprintf(stderr, "Closed policy exception request %s\n", id)
	return nil
}

// --- unapprove and comment ---

func newPolicyExceptionApprovalUnapproveCmd(factory policyExceptionClientFactory) *cobra.Command {
	var org string
	cmd := &cobra.Command{
		Use:   "unapprove <change-request-id>",
		Short: "[EXPERIMENTAL] Remove an approval from a request for a policy exception",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			c, org, err := factory(cmd.Context(), org)
			if err != nil {
				return err
			}
			id := posArgs[0]
			if _, err := getPolicyExceptionRequest(cmd.Context(), c, org, id); err != nil {
				return err
			}
			if err := c.changeRequests.UnapproveChangeRequest(cmd.Context(), org, id); err != nil {
				return fmt.Errorf("removing approval: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Removed approval from policy exception request %s\n", id)
			return nil
		},
	}
	constrictor.AttachArguments(cmd, changeRequestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &org)
	return cmd
}

func newPolicyExceptionApprovalCommentCmd(factory policyExceptionClientFactory) *cobra.Command {
	var org, comment string
	cmd := &cobra.Command{
		Use:     "comment <change-request-id>",
		Short:   "[EXPERIMENTAL] Comment on a request for a policy exception",
		Example: "  pulumi policy exception approval comment <change-request-id> -m \"Which CDN needs this?\"",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			if strings.TrimSpace(comment) == "" {
				return cmdCmd.ConfigurationError{Message: "a comment is required: pass -m <comment>"}
			}
			c, org, err := factory(cmd.Context(), org)
			if err != nil {
				return err
			}
			id := posArgs[0]
			if _, err := getPolicyExceptionRequest(cmd.Context(), c, org, id); err != nil {
				return err
			}
			if err := c.changeRequests.AddChangeRequestComment(cmd.Context(), org, id, comment); err != nil {
				return fmt.Errorf("adding comment: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Added comment to policy exception request %s\n", id)
			return nil
		},
	}
	constrictor.AttachArguments(cmd, changeRequestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &org)
	cmd.Flags().StringVarP(&comment, "message", "m", "", "The comment")
	return cmd
}

// --- log ---

type policyExceptionApprovalLogRenderFunc func(w io.Writer, events []esc_client.ChangeRequestEvent) error

func newPolicyExceptionApprovalLogCmd(factory policyExceptionClientFactory) *cobra.Command {
	var org string
	outputFormat := outputflag.OutputFlag[policyExceptionApprovalLogRenderFunc]{
		RenderForTerminal: func(w io.Writer, events []esc_client.ChangeRequestEvent) error {
			for _, e := range events {
				fprintChangeRequestEvent(w, e)
			}
			return nil
		},
		RenderJSON: func(w io.Writer, events []esc_client.ChangeRequestEvent) error {
			return ui.FprintJSON(w, struct {
				Events []esc_client.ChangeRequestEvent `json:"events"`
			}{events})
		},
	}
	cmd := &cobra.Command{
		Use:   "log <change-request-id>",
		Short: "[EXPERIMENTAL] Show the history of a request for a policy exception",
		Long: "[EXPERIMENTAL] Show the history of a request for a policy exception: its approvals,\n" +
			"comments and status changes, oldest first.",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			ctx := cmd.Context()
			c, org, err := factory(ctx, org)
			if err != nil {
				return err
			}
			id := posArgs[0]
			if _, err := getPolicyExceptionRequest(ctx, c, org, id); err != nil {
				return err
			}
			events := []esc_client.ChangeRequestEvent{}
			token := ""
			for {
				page, next, err := c.changeRequests.ListChangeRequestEvents(ctx, org, id, token)
				if err != nil {
					return fmt.Errorf("listing policy exception request events: %w", err)
				}
				for _, e := range page {
					if e.ReplacedBy == "" {
						events = append(events, e)
					}
				}
				if next == "" || next == token {
					break
				}
				token = next
			}
			return outputFormat.Get()(cmd.OutOrStdout(), events)
		},
	}
	constrictor.AttachArguments(cmd, changeRequestIDArgument)
	addPolicyExceptionApprovalOrgFlag(cmd, &org)
	outputflag.VarP(cmd.Flags(), &outputFormat)
	return cmd
}

// fprintChangeRequestEvent writes one line per event, and its comment below it. It follows `pulumi env approval log`.
func fprintChangeRequestEvent(w io.Writer, e esc_client.ChangeRequestEvent) {
	var what string
	switch e.EventType {
	case "approved_by_user":
		what = fmt.Sprintf("approved revision %d", e.RevisionNumber)
	case "unapproved_by_user":
		what = "removed their approval"
	case "commented":
		what = "commented"
	case "description_updated":
		what = "updated the description"
	case "revision_added":
		what = fmt.Sprintf("added revision %d", e.RevisionNumber)
	case "status_changed":
		switch {
		case e.PreviousStatus == "" && e.NewStatus == changeRequestDraft:
			what = "requested the exception"
		case e.Reason == "submitted_for_approval":
			what = "submitted the request for approval"
		case e.Reason == "manually_closed":
			what = "closed the request"
		case e.Reason == "applied successfully":
			what = "created the exception"
		case e.Reason == "gate_evaluation" && e.NewStatus == changeRequestReady:
			what = "marked the request approved (approval rules met)"
		case e.Reason == "gate_evaluation" && e.NewStatus == changeRequestPending:
			what = "marked the request pending (approval rules no longer met)"
		default:
			what = fmt.Sprintf("changed the status from %s to %s", e.PreviousStatus, e.NewStatus)
			if e.Reason != "" {
				what += " (" + e.Reason + ")"
			}
		}
	default:
		what = e.EventType
	}
	fmt.Fprintf(w, "%s  %s %s\n", e.CreatedAt.UTC().Format(time.RFC3339), e.CreatedBy.GithubLogin, what)
	if e.Comment != "" {
		fmt.Fprintf(w, "    %s\n", strings.ReplaceAll(e.Comment, "\n", "\n    "))
	}
}
