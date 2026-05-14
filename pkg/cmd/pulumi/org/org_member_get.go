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

// AI Generated - needs human review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// orgMemberGetClient is the narrow subset of cloud-API operations the get
// command needs. The list endpoint is the only documented way to read a
// single organization member.
type orgMemberGetClient interface {
	ListOrganizationMembers(
		ctx context.Context, orgName, mode string, continuationToken *string,
	) (apitype.ListOrganizationMembersResponse, error)
}

// orgMemberGetClientFactory resolves a cloud client and the organization
// the member lives in. orgFlag carries the raw value of `--org` (empty means
// "use the default org").
type orgMemberGetClientFactory func(
	ctx context.Context, orgFlag string,
) (orgMemberGetClient, string, error)

// orgMemberGetArgs collects the flag values for the get command.
type orgMemberGetArgs struct {
	org          string
	outputFormat outputflag.OutputFlag[orgMemberGetRenderFunc]
}

// defaultOrgMemberGetOutputFormat wires the OutputFlag to the per-format
// renderers so `--output` selects between them.
func defaultOrgMemberGetOutputFormat() outputflag.OutputFlag[orgMemberGetRenderFunc] {
	return outputflag.OutputFlag[orgMemberGetRenderFunc]{
		RenderForTerminal: renderOrgMemberGetText,
		RenderJSON:        renderOrgMemberGetJSON,
	}
}

// newOrgMemberGetCmd builds `pulumi org member get` with the production
// client factory.
func newOrgMemberGetCmd() *cobra.Command {
	return newOrgMemberGetCmdWith(defaultOrgMemberGetClientFactory)
}

func newOrgMemberGetCmdWith(factory orgMemberGetClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "orgMemberGetClientFactory must not be nil")
	var args orgMemberGetArgs
	args.outputFormat = defaultOrgMemberGetOutputFormat()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get <user-login>",
		Short:  "[EXPERIMENTAL] Get a member of an organization",
		Long: "[EXPERIMENTAL] Get a member of an organization.\n" +
			"\n" +
			"Retrieves a single organization member by GitHub login. Matching is\n" +
			"case-insensitive.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"raw member record as JSON.",
		Example: "  # Get a member of the default organization\n" +
			"  pulumi org member get alice\n\n" +
			"  # Get a member of a specific organization as JSON\n" +
			"  pulumi org member get alice --org acme --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runOrgMemberGet(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "user-login"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization that owns the member")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultOrgMemberGetClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the
// underlying *client.Client.
func defaultOrgMemberGetClientFactory(
	ctx context.Context, orgFlag string,
) (orgMemberGetClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"getting an organization member requires the Pulumi Cloud backend; run `pulumi login`")
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

// runOrgMemberGet is the cobra-decoupled command body so tests can drive it
// directly without spinning up the flag parser.
func runOrgMemberGet(
	ctx context.Context, w io.Writer,
	factory orgMemberGetClientFactory, userLogin string, args orgMemberGetArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	member, err := findOrganizationMember(ctx, c, org, userLogin)
	if err != nil {
		return err
	}

	return args.outputFormat.Get()(w, member)
}

// findOrganizationMember pages through ListOrganizationMembers and returns
// the first member whose GitHub login matches userLogin (case-insensitively).
// Both member modes ("frontend" and "backend") are queried so the command
// surfaces members regardless of which list they appear in.
func findOrganizationMember(
	ctx context.Context, c orgMemberGetClient, org, userLogin string,
) (apitype.OrganizationMember, error) {
	wantLogin := strings.ToLower(userLogin)
	for _, mode := range []string{"frontend", "backend"} {
		var continuationToken *string
		for {
			resp, err := c.ListOrganizationMembers(ctx, org, mode, continuationToken)
			if err != nil {
				return apitype.OrganizationMember{}, fmt.Errorf("getting organization member: %w", err)
			}
			for _, m := range resp.Members {
				if strings.EqualFold(m.User.GitHubLogin, wantLogin) {
					return m, nil
				}
			}
			if resp.ContinuationToken == "" {
				break
			}
			next := resp.ContinuationToken
			continuationToken = &next
		}
	}
	return apitype.OrganizationMember{}, fmt.Errorf("organization member %q not found in %s", userLogin, org)
}

type orgMemberGetRenderFunc func(w io.Writer, member apitype.OrganizationMember) error

func renderOrgMemberGetText(w io.Writer, member apitype.OrganizationMember) error {
	name := member.User.Name
	if name == "" {
		name = "-"
	}
	fmt.Fprintf(w, "%-16s %s\n", "User name:", name)
	fmt.Fprintf(w, "%-16s %s\n", "GitHub login:", member.User.GitHubLogin)
	if member.Role != "" {
		fmt.Fprintf(w, "%-16s %s\n", "Role:", member.Role)
	}
	if member.FGARole.Name != "" {
		fmt.Fprintf(w, "%-16s %s\n", "FGA role:", member.FGARole.Name)
	}
	if member.FGARole.ID != "" {
		fmt.Fprintf(w, "%-16s %s\n", "FGA role ID:", member.FGARole.ID)
	}
	if member.Created != "" {
		fmt.Fprintf(w, "%-16s %s\n", "Joined at:", member.Created)
	}
	return nil
}

// orgMemberGetJSON is the JSON envelope emitted by
// `pulumi org member get --output=json`.
type orgMemberGetJSON struct {
	Role          string               `json:"role"`
	User          apitype.UserInfo     `json:"user"`
	Created       string               `json:"created"`
	KnownToPulumi bool                 `json:"knownToPulumi"`
	VirtualAdmin  bool                 `json:"virtualAdmin"`
	Links         *apitype.MemberLinks `json:"links,omitempty"`
	FGARole       apitype.FGARole      `json:"fgaRole"`
}

func toOrgMemberGetJSON(member apitype.OrganizationMember) orgMemberGetJSON {
	return orgMemberGetJSON{
		Role:          member.Role,
		User:          member.User,
		Created:       member.Created,
		KnownToPulumi: member.KnownToPulumi,
		VirtualAdmin:  member.VirtualAdmin,
		Links:         member.Links,
		FGARole:       member.FGARole,
	}
}

func renderOrgMemberGetJSON(w io.Writer, member apitype.OrganizationMember) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toOrgMemberGetJSON(member))
}
