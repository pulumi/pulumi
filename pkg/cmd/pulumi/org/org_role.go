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
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/23003]: Not yet implemented.
func newOrgRoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "role",
		Short:  "Manage organization custom roles",
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

// TODO[https://github.com/pulumi/pulumi/issues/23008]: Not yet implemented.
func newOrgRoleListCmd() *cobra.Command {
	var (
		org     string
		purpose string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List custom roles for an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&org, "org", "", "The organization to list roles for")
	cmd.Flags().StringVar(&purpose, "purpose", "",
		"The UX purpose to filter by: organization, team, or token")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23007]: Not yet implemented.
func newOrgRoleNewCmd() *cobra.Command {
	var (
		org         string
		description string
		permissions []string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new",
		Short:  "Create a new custom role for an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization to create the role in")
	cmd.Flags().StringVar(&description, "description", "", "A description for the role")
	cmd.Flags().StringArrayVar(&permissions, "permission", nil,
		"A permission scope for the role (repeatable)")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23006]: Not yet implemented.
func newOrgRoleEditCmd() *cobra.Command {
	var (
		org         string
		newName     string
		description string
		permissions []string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit",
		Short:  "Update a custom role's name, description, or permissions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "role-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the role")
	cmd.Flags().StringVar(&newName, "new-name", "", "Rename the role")
	cmd.Flags().StringVar(&description, "description", "", "Update the role's description")
	cmd.Flags().StringArrayVar(&permissions, "permission", nil,
		"Set the role's permission scopes (repeatable)")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23005]: Not yet implemented.
func newOrgRoleRemoveCmd() *cobra.Command {
	var (
		org   string
		force bool
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "remove",
		Short:  "Delete a custom role from an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "role-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the role")
	cmd.Flags().BoolVar(&force, "force", false,
		"Force deletion even if the role is currently assigned")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23004]: Not yet implemented.
func newOrgRoleAssignCmd() *cobra.Command {
	var (
		org  string
		team string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "assign",
		Short:  "Assign a role to a team",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "role-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the role")
	cmd.Flags().StringVar(&team, "team", "", "The team to assign the role to")

	return cmd
}
