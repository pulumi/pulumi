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

func newPolicyIssueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "issue",
		Short:  "Inspect policy issues",
		Long:   "[EXPERIMENTAL] Inspect policy issues.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newPolicyIssueListCmd())
	cmd.AddCommand(newPolicyIssueGetCmd())
	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22996]: Not yet implemented.
func newPolicyIssueGetCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "Get the details of a specific policy issue",
		Long:   "[EXPERIMENTAL] Get the details of a specific policy issue.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "issue-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the issue")

	return cmd
}
