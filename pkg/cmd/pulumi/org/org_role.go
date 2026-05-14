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
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newOrgRoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "role",
		Short:  "Manage organization custom roles",
		Long:   "[EXPERIMENTAL] Manage organization custom roles.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newOrgRoleListCmd())
	cmd.AddCommand(newOrgRoleNewCmd())
	cmd.AddCommand(newOrgRoleEditCmd())
	cmd.AddCommand(newOrgRoleRemoveCmd())
	cmd.AddCommand(newOrgRoleAssignCmd())
	return cmd
}

// orgRoleClientFactory builds an org-role client and resolves the organization name
// from the given --org flag (or the user's default org).
type orgRoleClientFactory func(ctx context.Context, orgFlag string) (orgRoleClient, string, error)

// orgRoleClient is the subset of the cloud client used by org role commands.
// *client.Client already satisfies this interface directly; the indirection
// exists so tests can substitute a fake.
type orgRoleClient interface {
	ListOrgRoles(ctx context.Context, orgName, uxPurpose string) ([]apitype.Role, error)
	CreateOrgRole(ctx context.Context, orgName string, req apitype.CreateRoleRequest) (apitype.Role, error)
	GetOrgRole(ctx context.Context, orgName, roleID string) (apitype.Role, error)
	UpdateOrgRole(
		ctx context.Context, orgName, roleID string, req apitype.UpdateRoleRequest,
	) (apitype.Role, error)
	DeleteOrgRole(ctx context.Context, orgName, roleID string, force bool) error
	AssignTeamRole(ctx context.Context, orgName, teamName, roleID string) error
}

func defaultOrgRoleClientFactory(ctx context.Context, orgFlag string) (orgRoleClient, string, error) {
	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
	ws := pkgWorkspace.Instance

	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, "", err
	}

	currentBe, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, displayOpts)
	if err != nil {
		return nil, "", err
	}
	cloudBe, ok := currentBe.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New("organization roles require the Pulumi Cloud backend; run `pulumi login`")
	}

	orgName := orgFlag
	if orgName == "" {
		defaultOrg, err := cloudBe.GetDefaultOrg(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("resolving default organization: %w", err)
		}
		if defaultOrg == "" {
			return nil, "", errors.New(
				"no organization specified and no default organization is set; pass --org or run `pulumi org set-default`")
		}
		orgName = defaultOrg
	}

	return cloudBe.Client(), orgName, nil
}
