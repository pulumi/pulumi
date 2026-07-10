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

package org

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
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// orgMemberListClient is the subset of cloud-API operations the list command
// needs. Defined here so tests can stub a thin interface instead of the full
// HTTP client surface.
type orgMemberListClient interface {
	ListOrganizationMembers(
		ctx context.Context, orgName, mode string, continuationToken *string,
	) (apitype.ListOrganizationMembersResponse, error)
}

// orgMemberListClientFactory resolves a cloud client and the organization name
// to list members for. orgFlag carries the raw value of `--org` (empty means
// "use the user's default organization").
type orgMemberListClientFactory func(
	ctx context.Context, orgFlag string,
) (orgMemberListClient, string, error)

// orgMemberListRenderFunc is the per-format renderer signature, parameterised
// over the outputflag wiring so `--output` selects between the registered
// renderers at runtime.
type orgMemberListRenderFunc func(w io.Writer, members []apitype.OrganizationMember) error

// orgMemberListArgs collects the flag values for the list command, in one
// struct so Run can be driven directly from tests.
type orgMemberListArgs struct {
	org          string
	count        int
	all          bool
	renderOutput orgMemberListRenderFunc
}

func newOrgMemberListCmd() *cobra.Command {
	return newOrgMemberListCmdWith(nil)
}

func newOrgMemberListCmdWith(factory orgMemberListClientFactory) *cobra.Command {
	var args orgMemberListArgs
	output := outputflag.OutputFlag[orgMemberListRenderFunc]{
		RenderForTerminal: renderOrgMemberListTable,
		RenderJSON:        renderOrgMemberListJSON,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[EXPERIMENTAL] List members of an organization",
		Long: "[EXPERIMENTAL] List members of an organization.\n" +
			"\n" +
			"Returns the members of the organization, showing each member's user name,\n" +
			"role, and join date. Default output is a human-readable table; pass\n" +
			"--output=json for the full response as a JSON envelope.\n" +
			"\n" +
			"Wraps the `ListOrganizationMembers` Pulumi Cloud REST endpoint.",
		Example: "  # List members of the default organization.\n" +
			"  pulumi org member list\n\n" +
			"  # List members of a specific organization.\n" +
			"  pulumi org member list --org acme\n\n" +
			"  # List up to 100 members (auto-paginating as needed).\n" +
			"  pulumi org member list --count 100\n\n" +
			"  # Fetch every member, paging through all results.\n" +
			"  pulumi org member list --all\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi org member list --output json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := factory
			if f == nil {
				f = defaultOrgMemberListClientFactory
			}
			args.renderOutput = output.Get()
			return runOrgMemberList(cmd.Context(), cmd.OutOrStdout(), f, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.org, "org", "",
		"The organization to list members for. Defaults to the user's default organization")
	cmd.Flags().IntVar(&args.count, "count", 0,
		"The number of members to return; if larger than the server page size, additional pages are fetched automatically. "+
			"Defaults to the size of the first page")
	cmd.Flags().BoolVar(&args.all, "all", false,
		"Fetch every page until the server reports no more results. Mutually exclusive with --count")
	outputflag.VarP(cmd.Flags(), &output)

	cmd.MarkFlagsMutuallyExclusive("count", "all")

	return cmd
}

// defaultOrgMemberListClientFactory resolves the current Pulumi Cloud context
// and returns a client plus the organization name to query. When --org is
// empty, falls back to the user's default org (from the backend) and finally
// to the user's own login.
func defaultOrgMemberListClientFactory(
	ctx context.Context, orgFlag string,
) (orgMemberListClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	project, _, err := ws.ReadProject("")
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, "", err
	}

	currentBe, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBe, ok := currentBe.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"listing organization members requires the Pulumi Cloud backend; run `pulumi login`")
	}

	orgName := orgFlag
	if orgName == "" {
		defaultOrg, err := cloudBe.GetDefaultOrg(ctx)
		if err != nil {
			return nil, "", err
		}
		orgName = defaultOrg
	}

	userName, orgs, _, err := cloudBe.CurrentUser()
	if err != nil {
		return nil, "", err
	}
	if orgName == "" {
		orgName = userName
	}
	if !slices.Contains(orgs, orgName) && orgName != userName {
		return nil, "", fmt.Errorf("user %s is not a member of organization %s", userName, orgName)
	}

	return cloudBe.Client(), orgName, nil
}

// runOrgMemberList is the cobra-decoupled command body so tests can drive it
// directly without spinning up the flag parser.
func runOrgMemberList(
	ctx context.Context, w io.Writer, factory orgMemberListClientFactory, args orgMemberListArgs,
) error {
	if args.count < 0 {
		return fmt.Errorf("--count must be non-negative, got %d", args.count)
	}

	c, orgName, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	members, err := fetchOrganizationMembers(ctx, c, orgName, args.count, args.all)
	if err != nil {
		return fmt.Errorf("listing organization members: %w", err)
	}

	return args.renderOutput(w, members)
}

// fetchOrganizationMembers issues one or more ListOrganizationMembers calls,
// stopping when the server runs out of pages, when --all is unset and the
// requested count is reached, or when --all is unset and --count is zero
// (the caller asked for "just the first page").
func fetchOrganizationMembers(
	ctx context.Context,
	c orgMemberListClient,
	orgName string,
	count int,
	all bool,
) ([]apitype.OrganizationMember, error) {
	var members []apitype.OrganizationMember
	var token *string
	for {
		resp, err := c.ListOrganizationMembers(ctx, orgName, "frontend", token)
		if err != nil {
			return nil, err
		}
		members = append(members, resp.Members...)

		// Without --all, --count==0 means "first page only".
		if !all && count == 0 {
			return members, nil
		}
		// With --count, stop as soon as we have enough rows. We over-fetch by
		// at most one server page, then truncate; the API does not accept a
		// client-side limit.
		if !all && count > 0 && len(members) >= count {
			return members[:count], nil
		}
		// Server signals end-of-results by omitting (or zeroing) the token.
		if resp.ContinuationToken == "" {
			return members, nil
		}
		next := resp.ContinuationToken
		token = &next
	}
}

func renderOrgMemberListTable(w io.Writer, members []apitype.OrganizationMember) error {
	if len(members) == 0 {
		fmt.Fprintln(w, "No members found for this organization.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"USER", "NAME", "ROLE", "JOINED"})

	for _, m := range members {
		login := m.User.GitHubLogin
		if login == "" {
			login = m.User.Name
		}
		role := m.FGARole.Name
		if role == "" {
			role = m.Role
		}
		t.AppendRow(table.Row{
			login,
			m.User.Name,
			role,
			m.Created,
		})
	}
	t.Render()

	fmt.Fprintf(w, "\n%d member(s)\n", len(members))
	return nil
}

// orgMemberListEnvelope is the JSON shape emitted by
// `pulumi org member list --output=json`. The wire response from the cloud
// is one page at a time; this envelope aggregates the pages we actually
// fetched into a single document for scripts to consume.
type orgMemberListEnvelope struct {
	Members []apitype.OrganizationMember `json:"members"`
	Count   int                          `json:"count"`
}

func renderOrgMemberListJSON(w io.Writer, members []apitype.OrganizationMember) error {
	if members == nil {
		members = []apitype.OrganizationMember{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(orgMemberListEnvelope{
		Members: members,
		Count:   len(members),
	})
}
