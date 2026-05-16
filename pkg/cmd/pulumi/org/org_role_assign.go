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
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newOrgRoleAssignCmd() *cobra.Command {
	return newOrgRoleAssignCmdWith(defaultOrgRoleClientFactory)
}

// roleAssignRender renders the outcome of an assign operation.
type roleAssignRender func(w io.Writer, orgName, team, roleID string) error

func defaultRoleAssignOutput() outputflag.OutputFlag[roleAssignRender] {
	return outputflag.OutputFlag[roleAssignRender]{
		RenderForTerminal: renderRoleAssignText,
		RenderJSON:        renderRoleAssignJSON,
	}
}

func newOrgRoleAssignCmdWith(factory orgRoleClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "factory must not be nil")

	var org string
	output := defaultRoleAssignOutput()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "assign <role-id> <team>",
		Short:  "Assign a role to a team",
		Long: "[EXPERIMENTAL] Assign a custom role to a team.\n" +
			"\n" +
			"Each team can hold a single custom role at a time, so running this command\n" +
			"replaces the team's previously assigned role.\n" +
			"\n" +
			"Both --output default and --output json report the assignment, with JSON\n" +
			"shaped as an envelope (organization, action, team, roleId) for scripting.",
		Example: "  pulumi org role assign role-123 platform\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrgRoleAssign(
				cmd.Context(), cmd.OutOrStdout(), factory, org, args[1], args[0], output.Get())
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "role-id"},
			{Name: "team"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&org, "org", "",
		"The organization that owns the role. Defaults to the current default organization")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}

func runOrgRoleAssign(
	ctx context.Context,
	w io.Writer,
	factory orgRoleClientFactory,
	orgFlag, team, roleID string,
	render roleAssignRender,
) error {
	c, orgName, err := factory(ctx, orgFlag)
	if err != nil {
		return err
	}

	if err := c.AssignTeamRole(ctx, orgName, team, roleID); err != nil {
		return err
	}

	return render(w, orgName, team, roleID)
}

type roleAssignEnvelope struct {
	Organization string `json:"organization"`
	Action       string `json:"action"`
	Team         string `json:"team"`
	RoleID       string `json:"roleId"`
}

func renderRoleAssignJSON(w io.Writer, orgName, team, roleID string) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(roleAssignEnvelope{
		Organization: orgName,
		Action:       "Assigned",
		Team:         team,
		RoleID:       roleID,
	})
}

func renderRoleAssignText(w io.Writer, _ /*orgName*/, team, roleID string) error {
	fmt.Fprintf(w, "Assigned role %q to team %q\n", roleID, team)
	return nil
}
