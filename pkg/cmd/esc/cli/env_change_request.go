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

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type changeRequestCommand struct {
	env *envCommand
	org string
}

func newEnvChangeRequestCmd(env *envCommand) *cobra.Command {
	cr := &changeRequestCommand{env: env}

	cmd := &cobra.Command{
		Use:   "change-request",
		Short: "[EXPERIMENTAL] Manage change requests",
		Long: "[EXPERIMENTAL] Manage change requests\n" +
			"\n" +
			"When an environment is protected by approval rules, changes to it go through a change request\n" +
			"that must be approved before it is applied. Change requests are created by commands that accept\n" +
			"--draft, such as `env edit` and `env set`. Use `env open --draft=<id>`\n" +
			"to open the draft version of an environment.\n" +
			"\n" +
			"Subcommands exist for listing, viewing, diffing, approving, applying, closing, commenting on, and\n" +
			"rebasing change requests.",
		Example: fmt.Sprintf("  # Propose a change, then review and apply it\n"+
			"  %[1]s env set my-project/prod app.replicas 3 --draft\n"+
			"  %[1]s env change-request list --env my-project/prod\n"+
			"  %[1]s env change-request get <change-request-id>\n"+
			"  %[1]s env change-request approve <change-request-id> -m \"Looks good\"\n"+
			"  %[1]s env change-request apply <change-request-id>", env.esc.command),
		Args: cobra.NoArgs,
	}

	cmd.PersistentFlags().StringVarP(&cr.org, "organization", "o", "",
		"the organization that owns the change request (defaults to your default organization)")

	cmd.AddCommand(newEnvChangeRequestLsCmd(cr))
	cmd.AddCommand(newEnvChangeRequestGetCmd(cr))
	cmd.AddCommand(newEnvChangeRequestApproveCmd(cr))
	cmd.AddCommand(newEnvChangeRequestUnapproveCmd(cr))
	cmd.AddCommand(newEnvChangeRequestApplyCmd(cr))
	cmd.AddCommand(newEnvChangeRequestCloseCmd(cr))
	cmd.AddCommand(newEnvChangeRequestCommentCmd(cr))
	cmd.AddCommand(newEnvChangeRequestDiffCmd(cr))
	cmd.AddCommand(newEnvChangeRequestRebaseCmd(cr))

	return cmd
}

func (cr *changeRequestCommand) orgName(ctx context.Context) (string, error) {
	if err := cr.env.esc.getCachedClient(ctx); err != nil {
		return "", err
	}
	if cr.org != "" {
		return cr.org, nil
	}
	return cr.env.esc.account.DefaultOrg, nil
}

func (cr *changeRequestCommand) warnNoop(msg, comment string) {
	if comment != "" {
		msg += " The comment was not posted."
	}
	fmt.Fprint(cr.env.esc.stderr,
		cr.env.esc.colors.Colorize(fmt.Sprintf("%swarning: %v%s\n", colors.SpecWarning, msg, colors.Reset)))
}

func (cr *changeRequestCommand) hasApproved(ctx context.Context, c *client.GetChangeRequestResponse) (bool, error) {
	user, _, _, err := cr.env.esc.client.GetPulumiAccountDetails(ctx)
	if err != nil {
		return false, fmt.Errorf("getting current user: %w", err)
	}
	for _, g := range c.GateEvaluation.ApplicableGates {
		for _, a := range g.RuleDetails.Approvers {
			if a.GithubLogin == user {
				return true, nil
			}
		}
	}
	return false, nil
}

// confirm shows the change a change request proposes and asks the user whether to proceed.
// It returns false if the caller should stop, either because of --dry-run or because the user declined.
func (cr *changeRequestCommand) confirm(
	ctx context.Context, w io.Writer, orgName string, c *client.ChangeRequest, verb string, yes, dryRun bool,
) (bool, error) {
	if c.Status == "applied" || c.Status == "closed" {
		return false, fmt.Errorf("change request %v is %v", c.ID, c.Status)
	}

	ref := environmentRef{orgName: orgName, projectName: c.Entity.Project, envName: c.Entity.Name}
	fmt.Fprintf(w, "Change request %v will update environment %v.\n\n", c.ID, ref.String())
	if err := cr.printDraftDiff(ctx, w, ref, c.ID, ""); err != nil {
		return false, err
	}
	fmt.Fprintln(w)

	if dryRun {
		return false, nil
	}
	if yes || cmdutil.IsTruthy(os.Getenv(PulumiSkipConfirmationsEnvVar)) {
		return true, nil
	}
	if !cmdutil.Interactive() {
		return false, backenderr.ErrNonInteractiveRequiresYes
	}
	response, err := ui.PromptUserErr(
		fmt.Sprintf("Do you want to %v this change request?", verb),
		[]string{"yes", "no"},
		"no",
		cr.env.esc.colors,
		ui.SurveyStdio(cr.env.esc.stdin, w)...,
	)
	if err != nil {
		return false, fmt.Errorf("confirmation cancelled: %w", err)
	}
	if response != "yes" {
		return false, errors.New("confirmation declined")
	}
	return true, nil
}

// printDraftDiff prints the difference between the given version of an environment and a change request's draft.
func (cr *changeRequestCommand) printDraftDiff(
	ctx context.Context, w io.Writer, ref environmentRef, changeRequestID, version string,
) error {
	old, _, _, err := cr.env.esc.client.GetEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, version, false)
	if err != nil {
		return fmt.Errorf("getting environment definition: %w", err)
	}
	draft, _, err := cr.env.esc.client.GetEnvironmentDraft(
		ctx, ref.orgName, ref.projectName, ref.envName, changeRequestID)
	if err != nil {
		return fmt.Errorf("getting draft definition: %w", err)
	}
	oldName := ref.String()
	if version != "" {
		oldName += "@" + version
	}
	if d := diff(oldName, string(old), "draft "+changeRequestID, string(draft)); d != "" {
		fmt.Fprint(w, d)
	} else {
		fmt.Fprintln(w, "The draft does not change the environment definition.")
	}
	return nil
}

func newEnvChangeRequestLsCmd(cr *changeRequestCommand) *cobra.Command {
	var status string
	var output string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "[EXPERIMENTAL] List change requests.",
		Long: "[EXPERIMENTAL] List change requests\n" +
			"\n" +
			"This command lists the open change requests in an organization, optionally filtered to a single environment.\n" +
			"Use --status to list change requests with a different status.\n",
		Example: fmt.Sprintf("  # List open change requests in your default organization\n"+
			"  %[1]s env change-request list\n\n"+
			"  # List open change requests for one environment\n"+
			"  %[1]s env change-request list --env my-project/prod\n\n"+
			"  # List change requests with any status in another organization, as JSON\n"+
			"  %[1]s env change-request list --organization my-org --status all --output json", cr.env.esc.command),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			refStr := cr.env.envNameFlag
			var ref *environmentRef
			if refStr != "" {
				if cr.org != "" && strings.Count(refStr, "/") < 2 {
					if !strings.Contains(refStr, "/") {
						refStr = client.DefaultProject + "/" + refStr
					}
					refStr = cr.org + "/" + refStr
				}
				r, err := cr.env.getExistingEnvRefWithRelative(ctx, refStr, nil)
				if err != nil {
					return err
				}
				if r.version != "" {
					return errors.New("the list command does not accept versions")
				}
				if cr.org != "" && !strings.EqualFold(r.orgName, cr.org) {
					return fmt.Errorf("environment %v is not in organization %v", r.String(), cr.org)
				}
				ref, orgName = &r, r.orgName
			}

			changeRequests := []client.ChangeRequest{}
			token := ""
			for {
				page, next, err := cr.env.esc.client.ListChangeRequests(ctx, orgName, token)
				if err != nil {
					return fmt.Errorf("listing change requests: %w", err)
				}
				for _, c := range page {
					if !matchesStatusFilter(c, status) {
						continue
					}
					matchesEnvFilter := ref == nil ||
						(strings.EqualFold(c.Entity.Project, ref.projectName) && strings.EqualFold(c.Entity.Name, ref.envName))
					if !matchesEnvFilter {
						continue
					}
					changeRequests = append(changeRequests, c)
				}
				if next == "" || next == token {
					break
				}
				token = next
			}

			if format == outputJSON {
				return writeJSON(cr.env.esc.stdout, struct {
					ChangeRequests []client.ChangeRequest `json:"changeRequests"`
				}{changeRequests})
			}

			if len(changeRequests) == 0 {
				fmt.Fprintln(cr.env.esc.stderr, "No change requests found.")
				return nil
			}

			header := []any{"ID"}
			if ref == nil {
				header = append(header, "Environment")
			}
			if status != "open" {
				header = append(header, "Status")
			}
			t := newTable(cr.env.esc.stdout)
			t.AppendHeader(append(header, "Author"))
			for _, c := range changeRequests {
				row := []any{c.ID}
				if ref == nil {
					row = append(row, c.Entity.Project+"/"+c.Entity.Name)
				}
				if status != "open" {
					row = append(row, c.Status)
				}
				t.AppendRow(append(row, c.CreatedBy.GithubLogin))
			}
			t.Render()
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "open",
		"only list change requests with this status: open (draft, pending, or ready), applied, closed, or all")
	addOutputFlag(cmd, &output)

	return cmd
}

func matchesStatusFilter(c client.ChangeRequest, filter string) bool {
	switch filter {
	case "all":
		return true
	case "open":
		return c.Status == "draft" || c.Status == "pending" || c.Status == "ready"
	default:
		return strings.EqualFold(c.Status, filter)
	}
}

func newEnvChangeRequestGetCmd(cr *changeRequestCommand) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "get <change-request-id>",
		Short: "[EXPERIMENTAL] Show a change request.",
		Long: "[EXPERIMENTAL] Show a change request\n" +
			"\n" +
			"This command shows a change request's status, description, and approvals, and the environment\n" +
			"revision the draft is based on. If the environment has changed since then, use\n" +
			"`env change-request rebase` to bring the draft up to date.\n",
		Example: fmt.Sprintf("  # Show a change request\n"+
			"  %[1]s env change-request get <change-request-id>\n\n"+
			"  # Show a change request as JSON\n"+
			"  %[1]s env change-request get <change-request-id> --output json", cr.env.esc.command),
		Args: cmdutil.SpecificArgs([]string{"change-request-id"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			c, err := cr.env.esc.client.GetChangeRequest(ctx, orgName, args[0])
			if err != nil {
				return fmt.Errorf("getting change request: %w", err)
			}
			ref := environmentRef{orgName: orgName, projectName: c.Entity.Project, envName: c.Entity.Name}

			var draftStatus *client.EnvironmentDraftStatus
			if c.Action == "update" {
				draftStatus, err = cr.env.esc.client.GetEnvironmentDraftStatus(
					ctx, orgName, ref.projectName, ref.envName, c.ID)
				if err != nil {
					return fmt.Errorf("getting draft status: %w", err)
				}
			}
			url := cr.env.esc.changeRequestURL(ref, c.ID)

			if format == outputJSON {
				return writeJSON(cr.env.esc.stdout, struct {
					*client.GetChangeRequestResponse
					DraftStatus *client.EnvironmentDraftStatus `json:"draftStatus,omitempty"`
					URL         string                         `json:"url,omitempty"`
				}{c, draftStatus, url})
			}

			printChangeRequest(cr.env.esc.stdout, ref, c, draftStatus, url)
			return nil
		},
	}

	addOutputFlag(cmd, &output)

	return cmd
}

func printChangeRequest(
	w io.Writer,
	ref environmentRef,
	c *client.GetChangeRequestResponse,
	draftStatus *client.EnvironmentDraftStatus,
	url string,
) {
	fmt.Fprintf(w, "ID: %v\n", c.ID)
	fmt.Fprintf(w, "Environment: %v\n", ref.String())
	fmt.Fprintf(w, "Action: %v\n", c.Action)
	fmt.Fprintf(w, "Status: %v\n", c.Status)
	fmt.Fprintf(w, "Author: %v <%v>\n", c.CreatedBy.Name, c.CreatedBy.GithubLogin)
	fmt.Fprintf(w, "Created: %v\n", humanize.Time(c.CreatedAt))
	fmt.Fprintf(w, "Revision: %v\n", c.LatestRevisionNumber)
	if draftStatus != nil {
		fmt.Fprintf(w, "Base environment revision: %v\n", draftStatus.BaseRevision)
		fmt.Fprintf(w, "Current environment revision: %v\n", draftStatus.EnvironmentRevision)
	}
	if c.Description != "" {
		fmt.Fprintf(w, "Description:\n  %v\n", strings.ReplaceAll(c.Description, "\n", "\n  "))
	}

	gates := c.GateEvaluation
	fmt.Fprintf(w, "Approved: %v\n", gates.Satisfied)
	for _, g := range gates.ApplicableGates {
		state := "satisfied"
		if !g.Satisfied {
			state = "not satisfied"
		}
		fmt.Fprintf(w, "  %v (%v)", g.Name, state)
		if g.RuleDetails.RuleType == "approval_required" {
			approvers := make([]string, len(g.RuleDetails.Approvers))
			for i, a := range g.RuleDetails.Approvers {
				approvers[i] = a.GithubLogin
			}
			fmt.Fprintf(w, ": %v of %v approvals", len(approvers), g.RuleDetails.RequiredApprovals)
			if len(approvers) != 0 {
				fmt.Fprintf(w, " (%v)", strings.Join(approvers, ", "))
			}
		}
		fmt.Fprintln(w)
	}

	if url != "" {
		fmt.Fprintf(w, "URL: %v\n", url)
	}
}

func newEnvChangeRequestApproveCmd(cr *changeRequestCommand) *cobra.Command {
	var comment string
	var yes, dryRun bool

	cmd := &cobra.Command{
		Use:   "approve <change-request-id>",
		Short: "[EXPERIMENTAL] Approve a change request.",
		Long: "[EXPERIMENTAL] Approve a change request\n" +
			"\n" +
			"This command shows the change that the change request proposes and asks for confirmation,\n" +
			"then approves its latest revision. Use `env change-request get` to see how many approvals\n" +
			"are still required.\n",
		Example: fmt.Sprintf("  # Approve a change request\n"+
			"  %[1]s env change-request approve <change-request-id>\n\n"+
			"  # Approve with a comment\n"+
			"  %[1]s env change-request approve <change-request-id> -m \"Looks good\"\n\n"+
			"  # Show the proposed change without approving it\n"+
			"  %[1]s env change-request approve <change-request-id> --dry-run", cr.env.esc.command),
		Args: cmdutil.SpecificArgs([]string{"change-request-id"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			c, err := cr.env.esc.client.GetChangeRequest(ctx, orgName, args[0])
			if err != nil {
				return fmt.Errorf("getting change request: %w", err)
			}
			approved, err := cr.hasApproved(ctx, c)
			if err != nil {
				return err
			}
			if approved {
				cr.warnNoop(fmt.Sprintf("You already approved change request %v.", c.ID), comment)
				return nil
			}
			if ok, err := cr.confirm(ctx, cr.env.esc.stdout, orgName, &c.ChangeRequest, "approve", yes, dryRun); !ok {
				return err
			}

			err = cr.env.esc.client.ApproveChangeRequest(ctx, orgName, c.ID, c.LatestRevisionNumber, comment)
			if err != nil {
				return fmt.Errorf("approving change request: %w", err)
			}

			fmt.Fprintf(cr.env.esc.stderr, "Approved change request %v\n", c.ID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&comment, "message", "m", "", "a comment to add with the approval")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the proposed change without approving it")

	return cmd
}

func newEnvChangeRequestUnapproveCmd(cr *changeRequestCommand) *cobra.Command {
	return &cobra.Command{
		Use:   "unapprove <change-request-id>",
		Short: "[EXPERIMENTAL] Remove your approval from a change request.",
		Long: "[EXPERIMENTAL] Remove your approval from a change request\n" +
			"\n" +
			"This command withdraws an approval you gave earlier, for example if you approved by mistake.\n" +
			"To reject a change request outright, use `env change-request close`.\n",
		Example: fmt.Sprintf("  %[1]s env change-request unapprove <change-request-id>", cr.env.esc.command),
		Args:    cmdutil.SpecificArgs([]string{"change-request-id"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			c, err := cr.env.esc.client.GetChangeRequest(ctx, orgName, args[0])
			if err != nil {
				return fmt.Errorf("getting change request: %w", err)
			}
			approved, err := cr.hasApproved(ctx, c)
			if err != nil {
				return err
			}
			if !approved {
				cr.warnNoop(fmt.Sprintf("You have not approved change request %v.", c.ID), "")
				return nil
			}

			if err := cr.env.esc.client.UnapproveChangeRequest(ctx, orgName, c.ID); err != nil {
				return fmt.Errorf("removing approval: %w", err)
			}

			fmt.Fprintf(cr.env.esc.stderr, "Removed approval from change request %v\n", c.ID)
			return nil
		},
	}
}

func newEnvChangeRequestApplyCmd(cr *changeRequestCommand) *cobra.Command {
	var output string
	var yes, dryRun bool

	cmd := &cobra.Command{
		Use:   "apply <change-request-id>",
		Short: "[EXPERIMENTAL] Apply a change request.",
		Long: "[EXPERIMENTAL] Apply a change request\n" +
			"\n" +
			"This command shows the change that the change request proposes and asks for confirmation,\n" +
			"then applies it to the environment. The change request must have all required approvals.\n",
		Example: fmt.Sprintf("  # Apply a change request\n"+
			"  %[1]s env change-request apply <change-request-id>\n\n"+
			"  # Apply without a confirmation prompt\n"+
			"  %[1]s env change-request apply <change-request-id> --yes\n\n"+
			"  # Show the proposed change without applying it\n"+
			"  %[1]s env change-request apply <change-request-id> --dry-run", cr.env.esc.command),
		Args: cmdutil.SpecificArgs([]string{"change-request-id"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			c, err := cr.env.esc.client.GetChangeRequest(ctx, orgName, args[0])
			if err != nil {
				return fmt.Errorf("getting change request: %w", err)
			}
			if c.Status == "applied" {
				cr.warnNoop(fmt.Sprintf("Change request %v is already applied.", c.ID), "")
				if format == outputJSON {
					return writeJSON(cr.env.esc.stdout, &client.ChangeRequestApplyResult{})
				}
				return nil
			}
			w := cr.env.esc.stdout
			if format == outputJSON {
				w = cr.env.esc.stderr
			}
			if ok, err := cr.confirm(ctx, w, orgName, &c.ChangeRequest, "apply", yes, dryRun); !ok {
				return err
			}

			result, err := cr.env.esc.client.ApplyChangeRequest(ctx, orgName, c.ID)
			if err != nil {
				return fmt.Errorf("applying change request: %w", err)
			}

			if format == outputJSON {
				return writeJSON(cr.env.esc.stdout, result)
			}

			fmt.Fprintf(cr.env.esc.stderr, "Applied change request %v\n", c.ID)
			if result.Message != "" {
				fmt.Fprintln(cr.env.esc.stderr, result.Message)
			}
			return nil
		},
	}

	addOutputFlag(cmd, &output)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the proposed change without applying it")

	return cmd
}

func newEnvChangeRequestCloseCmd(cr *changeRequestCommand) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:   "close <change-request-id>",
		Short: "[EXPERIMENTAL] Close a change request.",
		Long: "[EXPERIMENTAL] Close a change request\n" +
			"\n" +
			"This command closes a change request without applying it. A closed change request cannot be\n" +
			"reopened.\n",
		Example: fmt.Sprintf("  # Close a change request\n"+
			"  %[1]s env change-request close <change-request-id>\n\n"+
			"  # Close with a comment explaining why\n"+
			"  %[1]s env change-request close <change-request-id> -m \"Superseded by another change\"",
			cr.env.esc.command),
		Args: cmdutil.SpecificArgs([]string{"change-request-id"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			c, err := cr.env.esc.client.GetChangeRequest(ctx, orgName, args[0])
			if err != nil {
				return fmt.Errorf("getting change request: %w", err)
			}
			if c.Status == "closed" {
				cr.warnNoop(fmt.Sprintf("Change request %v is already closed.", c.ID), comment)
				return nil
			}

			if err := cr.env.esc.client.CloseChangeRequest(ctx, orgName, c.ID, comment); err != nil {
				return fmt.Errorf("closing change request: %w", err)
			}

			fmt.Fprintf(cr.env.esc.stderr, "Closed change request %v\n", c.ID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&comment, "message", "m", "", "a comment explaining why the change request was closed")

	return cmd
}

func newEnvChangeRequestCommentCmd(cr *changeRequestCommand) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:   "comment <change-request-id>",
		Short: "[EXPERIMENTAL] Comment on a change request.",
		Long: "[EXPERIMENTAL] Comment on a change request\n" +
			"\n" +
			"This command adds a comment to a change request. To comment while approving or closing,\n" +
			"use -m on `env change-request approve` or `env change-request close`.\n",
		Example: fmt.Sprintf("  %[1]s env change-request comment <change-request-id> -m \"Can we lower the TTL?\"",
			cr.env.esc.command),
		Args: cmdutil.SpecificArgs([]string{"change-request-id"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			if err := cr.env.esc.client.AddChangeRequestComment(ctx, orgName, args[0], comment); err != nil {
				return fmt.Errorf("adding comment: %w", err)
			}

			fmt.Fprintf(cr.env.esc.stderr, "Added comment to change request %v\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVarP(&comment, "message", "m", "", "the comment")
	_ = cmd.MarkFlagRequired("message")

	return cmd
}

func newEnvChangeRequestDiffCmd(cr *changeRequestCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <change-request-id>",
		Short: "[EXPERIMENTAL] Show the change a change request proposes.",
		Long: "[EXPERIMENTAL] Show the change a change request proposes\n" +
			"\n" +
			"This command shows the difference between the environment revision the draft is based on and\n" +
			"the draft.\n",
		Example: fmt.Sprintf("  %[1]s env change-request diff <change-request-id>", cr.env.esc.command),
		Args:    cmdutil.SpecificArgs([]string{"change-request-id"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			c, err := cr.env.esc.client.GetChangeRequest(ctx, orgName, args[0])
			if err != nil {
				return fmt.Errorf("getting change request: %w", err)
			}
			ref := environmentRef{orgName: orgName, projectName: c.Entity.Project, envName: c.Entity.Name}

			status, err := cr.env.esc.client.GetEnvironmentDraftStatus(ctx, orgName, ref.projectName, ref.envName, c.ID)
			if err != nil {
				return fmt.Errorf("getting draft status: %w", err)
			}
			return cr.printDraftDiff(ctx, cr.env.esc.stdout, ref, c.ID, strconv.Itoa(status.BaseRevision))
		},
	}

	return cmd
}

func newEnvChangeRequestRebaseCmd(cr *changeRequestCommand) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "rebase <change-request-id>",
		Short: "[EXPERIMENTAL] Rebase a draft onto the environment's current revision.",
		Long: "[EXPERIMENTAL] Rebase a draft onto the environment's current revision\n" +
			"\n" +
			"This command replays a draft's changes onto the environment's current revision.\n" +
			"If the draft conflicts with the current revision, the draft is left unchanged and\n" +
			"each conflicting line range is printed from the draft.\n",
		Example: fmt.Sprintf("  %[1]s env change-request rebase <change-request-id>", cr.env.esc.command),
		Args:    cmdutil.SpecificArgs([]string{"change-request-id"}),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			c, err := cr.env.esc.client.GetChangeRequest(ctx, orgName, args[0])
			if err != nil {
				return fmt.Errorf("getting change request: %w", err)
			}
			if c.Action != "update" {
				return fmt.Errorf("change request %v is not an environment draft", c.ID)
			}
			ref := environmentRef{orgName: orgName, projectName: c.Entity.Project, envName: c.Entity.Name}

			status, err := cr.env.esc.client.GetEnvironmentDraftStatus(ctx, orgName, ref.projectName, ref.envName, c.ID)
			if err != nil {
				return fmt.Errorf("getting draft status: %w", err)
			}
			if status.BaseRevision == status.EnvironmentRevision {
				cr.warnNoop(fmt.Sprintf("Change request %v is already based on the current environment revision.", c.ID), "")
				if format == outputJSON {
					return writeJSON(cr.env.esc.stdout, &client.EnvironmentDraftRebaseResponse{
						ChangeRequestID:     c.ID,
						DraftRevisionNumber: status.DraftRevisionNumber,
						Conflicts:           []client.EnvironmentDraftConflict{},
					})
				}
				return nil
			}

			resp, err := cr.env.esc.client.RebaseEnvironmentDraft(ctx, orgName, ref.projectName, ref.envName, c.ID)
			if err != nil {
				return fmt.Errorf("rebasing draft: %w", err)
			}

			if format == outputJSON {
				if err := writeJSON(cr.env.esc.stdout, resp); err != nil {
					return err
				}
			} else if len(resp.Conflicts) == 0 {
				fmt.Fprintf(cr.env.esc.stderr, "Rebased change request %v (revision %v)\n", c.ID, resp.DraftRevisionNumber)
			} else {
				draftYAML, _, err := cr.env.esc.client.GetEnvironmentDraft(
					ctx, orgName, ref.projectName, ref.envName, c.ID)
				if err != nil {
					return fmt.Errorf("getting draft definition: %w", err)
				}
				printRebaseConflicts(cr.env.esc.stdout, resp.Conflicts, draftYAML)
			}

			if len(resp.Conflicts) != 0 {
				return fmt.Errorf("rebase of change request %v has %v conflict(s); the draft was not changed",
					c.ID, len(resp.Conflicts))
			}
			return nil
		},
	}

	addOutputFlag(cmd, &output)

	return cmd
}

func printRebaseConflicts(w io.Writer, conflicts []client.EnvironmentDraftConflict, draftYAML []byte) {
	draftLines := strings.Split(string(draftYAML), "\n")

	printRange := func(label string, lines []string, start, end int) {
		fmt.Fprintf(w, "  %v (lines %v-%v):\n", label, start, end)
		for i := start; i <= end && i <= len(lines); i++ {
			fmt.Fprintf(w, "    %4d | %v\n", i, lines[i-1])
		}
	}

	for i, c := range conflicts {
		fmt.Fprintf(w, "Conflict %v:\n", i+1)
		printRange("draft", draftLines, c.DraftStartLine, c.DraftEndLine)
	}
}
