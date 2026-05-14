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

package policy

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/22993]: Not yet implemented.
func newPolicyGroupNewCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new",
		Short:  "Create a new Policy Group",
		Long:   "[EXPERIMENTAL] Create a new Policy Group.",
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

	cmd.Flags().StringVar(&org, "org", "", "The organization to create the Policy Group in")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22991]: Not yet implemented.
func newPolicyGroupEditCmd() *cobra.Command {
	var (
		org                   string
		newName               string
		addStack              []string
		removeStack           []string
		addPolicyPack         []string
		removePolicyPack      []string
		addInsightsAccount    []string
		removeInsightsAccount []string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "edit",
		Short:  "Update a Policy Group's configuration",
		Long:   "[EXPERIMENTAL] Update a Policy Group's configuration.",
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

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the Policy Group")
	cmd.Flags().StringVar(&newName, "new-name", "", "Rename the Policy Group")
	cmd.Flags().StringArrayVar(&addStack, "add-stack", nil, "Add a stack to the Policy Group (repeatable)")
	cmd.Flags().StringArrayVar(&removeStack, "remove-stack", nil, "Remove a stack from the Policy Group (repeatable)")
	cmd.Flags().StringArrayVar(&addPolicyPack, "add-policy-pack", nil,
		"Add a Policy Pack to the Policy Group (repeatable)")
	cmd.Flags().StringArrayVar(&removePolicyPack, "remove-policy-pack", nil,
		"Remove a Policy Pack from the Policy Group (repeatable)")
	cmd.Flags().StringArrayVar(&addInsightsAccount, "add-insights-account", nil,
		"Add an Insights account to the Policy Group (repeatable)")
	cmd.Flags().StringArrayVar(&removeInsightsAccount, "remove-insights-account", nil,
		"Remove an Insights account from the Policy Group (repeatable)")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22990]: Not yet implemented.
func newPolicyGroupRemoveCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "remove",
		Short:  "Delete a Policy Group",
		Long:   "[EXPERIMENTAL] Delete a Policy Group.",
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

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the Policy Group")

	return cmd
}
