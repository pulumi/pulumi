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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newOrgRoleEditCmd() *cobra.Command {
	return newOrgRoleEditCmdWith(defaultOrgRoleClientFactory)
}

func newOrgRoleEditCmdWith(factory orgRoleClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "factory must not be nil")

	var (
		org         string
		newName     string
		description string
		detailsFile string
	)
	output := defaultRoleSingleOutput()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit <role-id>",
		Short:  "Update a custom role's name, description, or permissions",
		Long: "[EXPERIMENTAL] Update a custom role's name, description, or permissions.\n" +
			"\n" +
			"Each field follows ternary semantics: a flag that is not passed leaves the\n" +
			"current value unchanged.\n" +
			"\n" +
			"--details-file replaces the role's permission tree. Pass `-` to read from\n" +
			"stdin. Both --output default and --output json print the updated role.",
		Example: "  # Rename a role and tweak its description\n" +
			"  pulumi org role edit role-123 --name auditor --description \"Read-only auditor\"\n\n" +
			"  # Replace a role's permission tree from a file\n" +
			"  pulumi org role edit role-123 --details-file ./updated.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			editArgs := orgRoleEditArgs{RoleID: args[0]}
			if cmd.Flags().Changed("name") {
				editArgs.SetName = true
				editArgs.Name = newName
			}
			if cmd.Flags().Changed("description") {
				editArgs.SetDescription = true
				editArgs.Description = description
			}
			if cmd.Flags().Changed("details-file") {
				details, err := readRoleDetails(cmd.InOrStdin(), detailsFile)
				if err != nil {
					return err
				}
				editArgs.SetDetails = true
				editArgs.Details = details
			}

			if !editArgs.SetName && !editArgs.SetDescription && !editArgs.SetDetails {
				return errors.New(
					"nothing to update: pass at least one of --name, --description, or --details-file")
			}

			return runOrgRoleEdit(cmd.Context(), cmd.OutOrStdout(), factory, org, editArgs, output.Get())
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "role-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "",
		"The organization that owns the role. Defaults to the current default organization")
	cmd.Flags().StringVar(&newName, "name", "", "Rename the role")
	cmd.Flags().StringVar(&description, "description", "", "Update the role's description")
	cmd.Flags().StringVar(&detailsFile, "details-file", "",
		"Path to a JSON file containing the role's new permission tree (use `-` for stdin)")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}

type orgRoleEditArgs struct {
	RoleID string

	SetName bool
	Name    string

	SetDescription bool
	Description    string

	SetDetails bool
	Details    json.RawMessage
}

func runOrgRoleEdit(
	ctx context.Context,
	w io.Writer,
	factory orgRoleClientFactory,
	orgFlag string,
	args orgRoleEditArgs,
	render roleSingleRender,
) error {
	c, orgName, err := factory(ctx, orgFlag)
	if err != nil {
		return err
	}

	current, err := c.GetOrgRole(ctx, orgName, args.RoleID)
	if err != nil {
		return fmt.Errorf("looking up role %q: %w", args.RoleID, err)
	}

	req := apitype.UpdateRoleRequest{
		Name:        current.Name,
		Description: current.Description,
		Details:     current.Details,
	}
	if args.SetName {
		req.Name = args.Name
	}
	if args.SetDescription {
		req.Description = args.Description
	}
	if args.SetDetails {
		req.Details = args.Details
	}

	updated, err := c.UpdateOrgRole(ctx, orgName, args.RoleID, req)
	if err != nil {
		return fmt.Errorf("updating organization role: %w", err)
	}

	return render(w, orgName, "Updated", updated)
}
