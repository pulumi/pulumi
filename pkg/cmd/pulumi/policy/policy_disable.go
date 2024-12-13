// Copyright 2016-2024, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type policyDisableArgs struct {
	policyGroup string
	version     string
}

func newPolicyDisableCmd() *cobra.Command {
	args := policyDisableArgs{}

	cmd := &cobra.Command{
		Use:   "disable <org-name>/<policy-pack-name>",
		Args:  cmdutil.ExactArgs(1),
		Short: "Disable a Policy Pack for a Pulumi organization",
		Long:  "Disable a Policy Pack for a Pulumi organization",
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			// Obtain current PolicyPack, tied to the Pulumi Cloud backend.
			var err error
			policyPack, err := requirePolicyPack(ctx, cliArgs[0], cmdBackend.DefaultLoginManager)
			if err != nil {
				return err
			}

			// Attempt to disable the Policy Pack.
			return policyPack.Disable(ctx, args.policyGroup, backend.PolicyPackOperation{
				VersionTag: &args.version, Scopes: backend.CancellationScopes,
			})
		}),
	}

	cmd.PersistentFlags().StringVar(
		&args.policyGroup, "policy-group", "",
		"The Policy Group for which the Policy Pack will be disabled; if not specified, the default Policy Group is used")

	cmd.PersistentFlags().StringVar(
		&args.version, "version", "",
		"The version of the Policy Pack that will be disabled; "+
			"if not specified, any enabled version of the Policy Pack will be disabled")

	return cmd
}
