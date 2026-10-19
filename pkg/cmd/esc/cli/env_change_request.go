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
			"Subcommands exist for listing and viewing change requests.",
		Example: fmt.Sprintf("  # Propose a change, then review and apply it\n"+
			"  %[1]s env set my-project/prod app.replicas 3 --draft\n"+
			"  %[1]s env change-request list --env my-project/prod\n"+
			"  %[1]s env change-request get <change-request-id>", env.esc.command),
		Args: cobra.NoArgs,
	}

	cmd.PersistentFlags().StringVarP(&cr.org, "organization", "o", "",
		"the organization that owns the change request (defaults to your default organization)")

	cmd.AddCommand(newEnvChangeRequestLsCmd(cr))
	cmd.AddCommand(newEnvChangeRequestGetCmd(cr))

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
			"revision the draft is based on.\n",
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
			if c.Action != "update" {
				return fmt.Errorf("change request %v is not an environment draft", c.ID)
			}
			ref := environmentRef{orgName: orgName, projectName: c.Entity.Project, envName: c.Entity.Name}

			draftStatus, err := cr.env.esc.client.GetEnvironmentDraftStatus(
				ctx, orgName, ref.projectName, ref.envName, c.ID)
			if err != nil {
				return fmt.Errorf("getting draft status: %w", err)
			}
			url := cr.env.esc.changeRequestURL(ref, c.ID)

			if format == outputJSON {
				return writeJSON(cr.env.esc.stdout, struct {
					*client.GetChangeRequestResponse
					DraftStatus *client.EnvironmentDraftStatus `json:"draftStatus"`
					URL         string                         `json:"url,omitempty"`
				}{c, draftStatus, url})
			}

			printChangeRequest(cr.env.esc.stdout, ref, c, draftStatus, cr.env.esc.colors.Hyperlink(url, url))
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
	fmt.Fprintf(w, "Base environment revision: %v\n", draftStatus.BaseRevision)
	fmt.Fprintf(w, "Current environment revision: %v\n", draftStatus.EnvironmentRevision)
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
