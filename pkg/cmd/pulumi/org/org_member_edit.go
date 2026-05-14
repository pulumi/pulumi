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
	"errors"
	"fmt"
	"io"
	"slices"

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

// orgMemberEditClient is the narrow subset of cloud-API operations the edit
// command needs.
type orgMemberEditClient interface {
	UpdateOrganizationMember(
		ctx context.Context, orgName, userLogin string, req apitype.UpdateOrganizationMemberRequest,
	) error
	ListOrganizationMembers(
		ctx context.Context, orgName, mode string, continuationToken *string,
	) (apitype.ListOrganizationMembersResponse, error)
}

// orgMemberEditClientFactory resolves a cloud client and the organization
// the member belongs to. orgFlag carries the raw value of `--org` (empty
// means "use the default org").
type orgMemberEditClientFactory func(
	ctx context.Context, orgFlag string,
) (orgMemberEditClient, string, error)

// orgMemberEditArgs collects the flag values for the edit command. Only flags
// listed in changedFlags are applied to the PATCH body; this lets the run
// function distinguish an explicit empty `--role ""` from "user did not pass
// --role", and lets tests drive the command without spinning up cobra.
type orgMemberEditArgs struct {
	org          string
	outputFormat outputflag.OutputFlag[orgMemberGetRenderFunc]
	role         string
	fgaRoleID    string

	// changedFlags records which mutation flags were set by the user. Keys
	// are flag names: "role", "fga-role-id".
	changedFlags map[string]bool
}

// newOrgMemberEditCmd builds `pulumi org member edit` with the production
// client factory.
func newOrgMemberEditCmd() *cobra.Command {
	return newOrgMemberEditCmdWith(defaultOrgMemberEditClientFactory)
}

func newOrgMemberEditCmdWith(factory orgMemberEditClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "orgMemberEditClientFactory must not be nil")
	var args orgMemberEditArgs
	args.outputFormat = defaultOrgMemberGetOutputFormat()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit <user-login>",
		Short:  "[EXPERIMENTAL] Modify a member's role within an organization",
		Long: "[EXPERIMENTAL] Modify a member's role within an organization.\n" +
			"\n" +
			"Updates the role assigned to an organization member. Pass --role to\n" +
			"assign one of the built-in roles (`member`, `admin`, `billingManager`),\n" +
			"or --fga-role-id to assign a custom role. If both are provided the\n" +
			"service uses --fga-role-id. At least one of --role or --fga-role-id\n" +
			"must be supplied.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"raw member record as JSON.",
		Example: "  # Promote a member to admin in the default organization\n" +
			"  pulumi org member edit alice --role admin\n\n" +
			"  # Assign a custom (FGA) role and emit JSON\n" +
			"  pulumi org member edit alice --fga-role-id role-abc123 --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.changedFlags = orgMemberEditChangedFlags(cmd)
			return runOrgMemberEdit(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
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
	cmd.Flags().StringVar(&args.role, "role", "",
		"The built-in role to assign: member, admin, or billingManager")
	cmd.Flags().StringVar(&args.fgaRoleID, "fga-role-id", "",
		"The custom role to assign (takes precedence over --role)")

	return cmd
}

// orgMemberEditChangedFlags snapshots which mutation flags were set on the
// command line. Cobra clears `.Changed` after RunE returns, so we capture it
// inside RunE before calling into the cobra-decoupled body.
func orgMemberEditChangedFlags(cmd *cobra.Command) map[string]bool {
	out := make(map[string]bool, 2)
	for _, n := range []string{"role", "fga-role-id"} {
		f := cmd.Flag(n)
		out[n] = f != nil && f.Changed
	}
	return out
}

// defaultOrgMemberEditClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the
// underlying *client.Client.
func defaultOrgMemberEditClientFactory(
	ctx context.Context, orgFlag string,
) (orgMemberEditClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"editing an organization member requires the Pulumi Cloud backend; run `pulumi login`")
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

// runOrgMemberEdit is the cobra-decoupled command body so tests can drive it
// directly without spinning up the flag parser.
func runOrgMemberEdit(
	ctx context.Context, w io.Writer,
	factory orgMemberEditClientFactory, userLogin string, args orgMemberEditArgs,
) error {
	roleChanged := args.changedFlags["role"]
	fgaChanged := args.changedFlags["fga-role-id"]
	if !roleChanged && !fgaChanged {
		return errors.New(
			"no changes specified; pass at least one of --role or --fga-role-id")
	}

	req := apitype.UpdateOrganizationMemberRequest{}
	if roleChanged {
		role := args.role
		req.Role = &role
	}
	if fgaChanged {
		fgaID := args.fgaRoleID
		req.FgaRoleId = &fgaID
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	if err := c.UpdateOrganizationMember(ctx, org, userLogin, req); err != nil {
		return fmt.Errorf("updating organization member: %w", err)
	}

	member, err := findOrganizationMember(ctx, c, org, userLogin)
	if err != nil {
		return fmt.Errorf("reading organization member after edit: %w", err)
	}

	return args.outputFormat.Get()(w, member)
}
