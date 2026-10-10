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
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
)

type changeRequestCommand struct {
	env *envCommand
	org string
}

func newEnvChangeRequestCmd(env *envCommand) *cobra.Command {
	cr := &changeRequestCommand{env: env}

	cmd := &cobra.Command{
		Use:   "change-request",
		Short: "Manage change requests",
		Long: "Manage change requests\n" +
			"\n" +
			"Change requests are created by commands that accept --draft, such as `env edit` and `env set`.\n" +
			"Subcommands exist for listing, viewing, approving, applying, closing, and rebasing change requests.",
		Args: cobra.NoArgs,
	}

	cmd.PersistentFlags().StringVar(&cr.org, "org", "",
		"the organization that owns the change request (defaults to your default organization)")

	cmd.AddCommand(newEnvChangeRequestLsCmd(cr))
	cmd.AddCommand(newEnvChangeRequestGetCmd(cr))
	cmd.AddCommand(newEnvChangeRequestApproveCmd(cr))
	cmd.AddCommand(newEnvChangeRequestUnapproveCmd(cr))
	cmd.AddCommand(newEnvChangeRequestApplyCmd(cr))
	cmd.AddCommand(newEnvChangeRequestCloseCmd(cr))
	cmd.AddCommand(newEnvChangeRequestCommentCmd(cr))
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

func (cr *changeRequestCommand) envRef(orgName string, c *client.ChangeRequest) environmentRef {
	return environmentRef{orgName: orgName, projectName: c.Entity.Project, envName: c.Entity.Name}
}

func newEnvChangeRequestLsCmd(cr *changeRequestCommand) *cobra.Command {
	var status string
	var output string

	cmd := &cobra.Command{
		Use:     "list [[<org-name>/][<project-name>/]<environment-name>]",
		Aliases: []string{"ls"},
		Short:   "List change requests.",
		Long: "List change requests\n" +
			"\n" +
			"This command lists the change requests in an organization, optionally filtered to a single environment.\n",
		Args: cobra.MaximumNArgs(1),
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

			var ref *environmentRef
			if len(args) != 0 || cr.env.envNameFlag != "" {
				r, _, err := cr.env.getExistingEnvRef(ctx, args)
				if err != nil {
					return err
				}
				if r.version != "" {
					return errors.New("the list command does not accept versions")
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
					if status != "" && !strings.EqualFold(c.Status, status) {
						continue
					}
					if ref != nil && (!strings.EqualFold(c.Entity.Project, ref.projectName) ||
						!strings.EqualFold(c.Entity.Name, ref.envName)) {
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
				fmt.Fprintln(cr.env.esc.stdout, "No change requests found.")
				return nil
			}

			t := newTable(cr.env.esc.stdout)
			t.AppendHeader([]any{"ID", "Environment", "Status", "Author", "Description", "Age"})
			for _, c := range changeRequests {
				description, _, _ := strings.Cut(c.Description, "\n")
				t.AppendRow([]any{
					c.ID,
					c.Entity.Project + "/" + c.Entity.Name,
					c.Status,
					c.CreatedBy.GithubLogin,
					description,
					humanize.Time(c.CreatedAt),
				})
			}
			t.Render()
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "",
		"only list change requests with this status (draft, pending, ready, applied, or closed)")
	addOutputFlag(cmd, &output)

	return cmd
}

func newEnvChangeRequestGetCmd(cr *changeRequestCommand) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "get <change-request-id>",
		Short: "Show a change request.",
		Long: "Show a change request\n" +
			"\n" +
			"This command shows a change request's status, description, approvals, and revisions.\n",
		Args: cobra.ExactArgs(1),
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
			ref := cr.envRef(orgName, &c.ChangeRequest)

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

	cmd := &cobra.Command{
		Use:   "approve <change-request-id>",
		Short: "Approve a change request.",
		Long: "Approve a change request\n" +
			"\n" +
			"This command approves the latest revision of a change request.\n",
		Args: cobra.ExactArgs(1),
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
			err = cr.env.esc.client.ApproveChangeRequest(ctx, orgName, c.ID, c.LatestRevisionNumber, comment)
			if err != nil {
				return fmt.Errorf("approving change request: %w", err)
			}

			fmt.Fprintf(cr.env.esc.stdout, "Approved revision %v of change request %v\n", c.LatestRevisionNumber, c.ID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&comment, "message", "m", "", "a comment to add with the approval")

	return cmd
}

func newEnvChangeRequestUnapproveCmd(cr *changeRequestCommand) *cobra.Command {
	return &cobra.Command{
		Use:   "unapprove <change-request-id>",
		Short: "Remove your approval from a change request.",
		Long: "Remove your approval from a change request\n" +
			"\n" +
			"This command removes your approval from a change request.\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			if err := cr.env.esc.client.UnapproveChangeRequest(ctx, orgName, args[0]); err != nil {
				return fmt.Errorf("removing approval: %w", err)
			}

			fmt.Fprintf(cr.env.esc.stdout, "Removed approval from change request %v\n", args[0])
			return nil
		},
	}
}

func newEnvChangeRequestApplyCmd(cr *changeRequestCommand) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "apply <change-request-id>",
		Short: "Apply a change request.",
		Long: "Apply a change request\n" +
			"\n" +
			"This command applies an approved change request.\n",
		Args: cobra.ExactArgs(1),
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

			result, err := cr.env.esc.client.ApplyChangeRequest(ctx, orgName, args[0])
			if err != nil {
				return fmt.Errorf("applying change request: %w", err)
			}

			if format == outputJSON {
				return writeJSON(cr.env.esc.stdout, result)
			}

			fmt.Fprintf(cr.env.esc.stdout, "Applied change request %v\n", args[0])
			if result.Message != "" {
				fmt.Fprintln(cr.env.esc.stdout, result.Message)
			}
			return nil
		},
	}

	addOutputFlag(cmd, &output)

	return cmd
}

func newEnvChangeRequestCloseCmd(cr *changeRequestCommand) *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:   "close <change-request-id>",
		Short: "Close a change request.",
		Long: "Close a change request\n" +
			"\n" +
			"This command closes a change request without applying it.\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			if err := cr.env.esc.client.CloseChangeRequest(ctx, orgName, args[0], comment); err != nil {
				return fmt.Errorf("closing change request: %w", err)
			}

			fmt.Fprintf(cr.env.esc.stdout, "Closed change request %v\n", args[0])
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
		Short: "Comment on a change request.",
		Long: "Comment on a change request\n" +
			"\n" +
			"This command adds a comment to a change request.\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			orgName, err := cr.orgName(ctx)
			if err != nil {
				return err
			}

			if err := cr.env.esc.client.AddChangeRequestComment(ctx, orgName, args[0], comment); err != nil {
				return fmt.Errorf("adding comment: %w", err)
			}

			fmt.Fprintf(cr.env.esc.stdout, "Added comment to change request %v\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVarP(&comment, "message", "m", "", "the comment")
	_ = cmd.MarkFlagRequired("message")

	return cmd
}

func newEnvChangeRequestRebaseCmd(cr *changeRequestCommand) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "rebase <change-request-id>",
		Short: "Rebase a draft onto the environment's current revision.",
		Long: "Rebase a draft onto the environment's current revision\n" +
			"\n" +
			"This command replays a draft's changes onto the environment's current revision.\n" +
			"If the draft conflicts with the current revision, the draft is left unchanged and\n" +
			"each conflicting line range is printed from both the environment and the draft.\n",
		Args: cobra.ExactArgs(1),
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
			ref := cr.envRef(orgName, &c.ChangeRequest)

			resp, err := cr.env.esc.client.RebaseEnvironmentDraft(ctx, orgName, ref.projectName, ref.envName, c.ID)
			if err != nil {
				return fmt.Errorf("rebasing draft: %w", err)
			}

			if format == outputJSON {
				if err := writeJSON(cr.env.esc.stdout, resp); err != nil {
					return err
				}
			} else if len(resp.Conflicts) == 0 {
				fmt.Fprintf(cr.env.esc.stdout, "Rebased change request %v (revision %v)\n", c.ID, resp.DraftRevisionNumber)
			} else {
				envYAML, _, _, err := cr.env.esc.client.GetEnvironment(
					ctx, orgName, ref.projectName, ref.envName, "", false)
				if err != nil {
					return fmt.Errorf("getting environment definition: %w", err)
				}
				draftYAML, _, err := cr.env.esc.client.GetEnvironmentDraft(
					ctx, orgName, ref.projectName, ref.envName, c.ID)
				if err != nil {
					return fmt.Errorf("getting draft definition: %w", err)
				}
				printRebaseConflicts(cr.env.esc.stdout, resp.Conflicts, envYAML, draftYAML)
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

func printRebaseConflicts(w io.Writer, conflicts []client.EnvironmentDraftConflict, envYAML, draftYAML []byte) {
	envLines := strings.Split(string(envYAML), "\n")
	draftLines := strings.Split(string(draftYAML), "\n")

	printRange := func(label string, lines []string, start, end int) {
		fmt.Fprintf(w, "  %v (lines %v-%v):\n", label, start, end)
		for i := start; i <= end && i <= len(lines); i++ {
			fmt.Fprintf(w, "    %4d | %v\n", i, lines[i-1])
		}
	}

	for i, c := range conflicts {
		fmt.Fprintf(w, "Conflict %v:\n", i+1)
		printRange("environment", envLines, c.StartLine, c.EndLine)
		printRange("draft", draftLines, c.DraftStartLine, c.DraftEndLine)
	}
}
