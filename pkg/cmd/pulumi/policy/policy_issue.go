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
	"github.com/spf13/cobra"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

func newPolicyIssueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "[EXPERIMENTAL] Manage policy issues",
		Long:  "[EXPERIMENTAL] Manage policy issues.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newPolicyIssueListCmd())
	cmd.AddCommand(newPolicyIssueGetCmd())
	cmd.AddCommand(newPolicyIssueExportCmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newPolicyIssueEditCmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	return cmd
}
