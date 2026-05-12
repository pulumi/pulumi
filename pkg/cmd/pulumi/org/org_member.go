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

func newOrgMemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "member",
		Short:  "Manage organization members",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newOrgMemberListCmd())
	cmd.AddCommand(newOrgMemberGetCmd())
	cmd.AddCommand(newOrgMemberEditCmd())
	cmd.AddCommand(newOrgMemberRemoveCmd())
	return cmd
}

func newOrgMemberListCmd() *cobra.Command {
	var (
		org  string
		mode string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List members of an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&org, "org", "", "The organization to list members for")
	cmd.Flags().StringVar(&mode, "mode", "frontend",
		"Member list mode: frontend or backend")

	return cmd
}

func newOrgMemberGetCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "Get a member of an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "user-login"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the member")

	return cmd
}

func newOrgMemberEditCmd() *cobra.Command {
	var (
		org       string
		role      string
		fgaRoleID string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit",
		Short:  "Modify a member's role within an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "user-login"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the member")
	cmd.Flags().StringVar(&role, "role", "",
		"The built-in role to assign: member, admin, or billingManager")
	cmd.Flags().StringVar(&fgaRoleID, "fga-role-id", "",
		"The custom role to assign (takes precedence over --role)")

	return cmd
}

func newOrgMemberRemoveCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "remove",
		Short:  "Remove a member from an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "user-login"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the member")

	return cmd
}
