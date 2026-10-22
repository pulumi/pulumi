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
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

type policyDisableArgs struct {
	policyGroup string
	version     string
}

type policyDisableResult struct {
	Organization string  `json:"organization"`
	PolicyGroup  string  `json:"policyGroup"`
	PolicyPack   string  `json:"policyPack"`
	VersionTag   *string `json:"versionTag"`
}

type policyDisableRenderFunc func(w io.Writer, result policyDisableResult) error

func renderPolicyDisableText(w io.Writer, result policyDisableResult) error {
	if result.VersionTag == nil {
		fmt.Fprintf(w, "Disabled policy pack %s/%s in policy group %s.\n",
			result.Organization, result.PolicyPack, result.PolicyGroup)
	} else {
		fmt.Fprintf(w, "Disabled version %s of policy pack %s/%s in policy group %s.\n",
			*result.VersionTag, result.Organization, result.PolicyPack, result.PolicyGroup)
	}
	return nil
}

func renderPolicyDisableJSON(w io.Writer, result policyDisableResult) error {
	return ui.FprintJSON(w, result)
}

func newPolicyDisableCmd() *cobra.Command {
	return newPolicyDisableCmdWith(defaultPolicyPackCmdDeps)
}

func newPolicyDisableCmdWith(deps policyPackCmdDeps) *cobra.Command {
	contract.Assertf(deps != nil, "policyPackCmdDeps must not be nil")
	args := policyDisableArgs{}
	outputFormat := outputflag.OutputFlag[policyDisableRenderFunc]{
		RenderForTerminal: renderPolicyDisableText,
		RenderJSON:        renderPolicyDisableJSON,
	}

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
			policyPack, _, err := deps(ctx, cliArgs[0])
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

			group := args.policyGroup
			if group == "" {
				group = apitype.DefaultPolicyGroup
			}
			var versionTag *string
			if args.version != "" {
				versionTag = &args.version
			}
			return outputFormat.Get()(cmd.OutOrStdout(), policyDisableResult{
				Organization: policyPack.Ref().OrgName(),
				PolicyGroup:  group,
				PolicyPack:   string(policyPack.Ref().Name()),
				VersionTag:   versionTag,
			})
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
