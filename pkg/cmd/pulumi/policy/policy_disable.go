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
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

type policyDisableArgs struct {
	policyGroup string
	version     string
}

func newPolicyDisableCmd() *cobra.Command {
	return newPolicyDisableCmdWith(defaultPolicyPackCmdDeps)
}

func newPolicyDisableCmdWith(deps policyPackCmdDeps) *cobra.Command {
	contract.Assertf(deps != nil, "policyPackCmdDeps must not be nil")
	args := policyDisableArgs{}
	outputFormat := defaultPolicyGroupGetOutputFormat()

	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable a Policy Pack for a Pulumi organization",
		Long: "Disable a Policy Pack for a Pulumi organization\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi policy disable acme/aws-guardrails\n" +
			"  pulumi policy disable acme/aws-guardrails --policy-group prod --output json",
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			// Obtain current PolicyPack, tied to the Pulumi Cloud backend.
			policyPack, client, err := deps(ctx, cliArgs[0])
			if err != nil {
				return err
			}

			// Attempt to disable the Policy Pack.
			err = policyPack.Disable(ctx, args.policyGroup, backend.PolicyPackOperation{
				VersionTag: &args.version, Scopes: backend.CancellationScopes,
			})
			if err != nil {
				return err
			}

			resp, err := readBackPolicyGroup(ctx, client, policyPack, args.policyGroup)
			if err != nil {
				return err
			}
			return outputFormat.Get()(cmd.OutOrStdout(), resp)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "policy-pack", Usage: "<org-name>/<policy-pack-name>"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().StringVar(
		&args.policyGroup, "policy-group", "",
		"The Policy Group for which the Policy Pack will be disabled; if not specified, the default Policy Group is used")

	cmd.PersistentFlags().StringVar(
		&args.version, "version", "",
		"The version of the Policy Pack that will be disabled; "+
			"if not specified, any enabled version of the Policy Pack will be disabled")

	outputflag.VarP(cmd.Flags(), &outputFormat)
	return cmd
}
