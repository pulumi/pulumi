// Copyright 2016, Pulumi Corporation.
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
	backendSecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/spf13/cobra"
)

func NewPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage policy packs, policy groups, policy issues and compliance",
		Long: "Manage policy packs, policy groups, policy issues and compliance.\n" +
			"\n" +
			"Policy packs contain the policies that check your resources. Policy groups decide which\n" +
			"packs run on which stacks and cloud accounts. Policy issues are the violations those packs\n" +
			"find, and compliance summarizes them per stack or account. See `pulumi policy group --help`,\n" +
			"`pulumi policy issue --help` and `pulumi policy compliance --help`.",
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newPolicyAnalyzeCmd(
		pkgWorkspace.Instance, cmdBackend.DefaultLoginManager,
		backendSecrets.DefaultProvider, nil))
	cmd.AddCommand(newPolicyDisableCmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newPolicyEnableCmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newPolicyGetCmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newPolicyGroupCmd())
	cmd.AddCommand(newPolicyInstallCmd())
	cmd.AddCommand(newPolicyListCmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newPolicyNewCmd())
	cmd.AddCommand(newPolicyPublishCmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newPolicyRemoveCmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newPolicyValidateCmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager))
	cmd.AddCommand(newPolicyComplianceCmd())
	cmd.AddCommand(newPolicyIssueCmd())

	return cmd
}
