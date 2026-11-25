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
	"strings"

	"github.com/spf13/cobra"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
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
			"  pulumi policy exception approval get <id>\n" +
			"  pulumi policy exception approval approve <id> -m \"Looks good\"\n" +
			"  pulumi policy exception approval close <id> -m \"Fix the bucket instead\"",
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

// requestIDArgument is the single argument of the commands that act on one request: its ID, which
// `pulumi policy exception new` prints.
var requestIDArgument = &constrictor.Arguments{
	Arguments: []constrictor.Argument{{Name: "id"}},
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
	warn(stderr, msg)
}

func warn(stderr io.Writer, msg string) {
	fmt.Fprint(stderr, cmdutil.GetGlobalColorization().Colorize(
		fmt.Sprintf("%swarning: %s%s\n", colors.SpecWarning, msg, colors.Reset)))
}

type policyExceptionApprovalDecisionArgs struct {
	org     string
	comment string
	yes     bool
	dryRun  bool
}

// confirmPolicyExceptionDecision shows an open request, after a headline that says what the decision does, and asks
// whether to approve or close it. It returns false with --dry-run.
func confirmPolicyExceptionDecision(
	stderr io.Writer, r *policyExceptionRequest, verb, headline string, args policyExceptionApprovalDecisionArgs,
) (bool, error) {
	if r.Status == changeRequestApplied || r.Status == changeRequestClosed {
		return false, fmt.Errorf("policy exception request %s is %s", r.ID, r.Status)
	}
	fmt.Fprintf(stderr, "%s\n\n", headline)
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
