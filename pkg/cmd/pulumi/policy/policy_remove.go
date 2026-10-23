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
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

const allKeyword = "all"

type policyRemoveResult struct {
	OrganizationName string  `json:"organizationName"`
	Name             string  `json:"name"`
	VersionTag       *string `json:"versionTag"`
}

type policyRemoveRenderFunc func(w io.Writer, result policyRemoveResult) error

func renderPolicyRemoveText(w io.Writer, result policyRemoveResult) error {
	if result.VersionTag == nil {
		fmt.Fprintf(w, "Removed policy pack %s/%s.\n", result.OrganizationName, result.Name)
	} else {
		fmt.Fprintf(w, "Removed version %s of policy pack %s/%s.\n",
			*result.VersionTag, result.OrganizationName, result.Name)
	}
	return nil
}

func renderPolicyRemoveJSON(w io.Writer, result policyRemoveResult) error {
	return ui.FprintJSON(w, result)
}

func newPolicyRemoveCmd() *cobra.Command {
	return newPolicyRemoveCmdWith(defaultPolicyPackCmdDeps)
}

func newPolicyRemoveCmdWith(deps policyPackCmdDeps) *cobra.Command {
	contract.Assertf(deps != nil, "policyPackCmdDeps must not be nil")
	var yes bool
	outputFormat := outputflag.OutputFlag[policyRemoveRenderFunc]{
		RenderForTerminal: renderPolicyRemoveText,
		RenderJSON:        renderPolicyRemoveJSON,
	}
	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm", "delete"},
		Short:   "Removes a Policy Pack from a Pulumi organization",
		Long: "Removes a Policy Pack from a Pulumi organization. " +
			"The Policy Pack must be disabled from all Policy Groups before it can be removed.\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi policy rm acme/aws-guardrails 3\n" +
			"  pulumi policy rm acme/aws-guardrails all --yes --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			yes = yes || env.SkipConfirmations.Value()
			// Obtain current PolicyPack, tied to the Pulumi Cloud backend.
			policyPack, _, err := deps(ctx, args[0])
			if err != nil {
				return err
			}

			var version *string
			if args[1] != allKeyword {
				version = &args[1]
			}

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			prompt := fmt.Sprintf("This will permanently remove the '%s' policy!", args[0])
			if err := ui.ConfirmDeletion(
				yes, cmdutil.Interactive(), prompt, args[0], cmd.OutOrStdout(), opts,
			); err != nil {
				return err
			}

			// Attempt to remove the Policy Pack.
			err = policyPack.Remove(ctx, backend.PolicyPackOperation{
				VersionTag: version, Scopes: backend.CancellationScopes,
			})
			if err != nil {
				return err
			}

			return outputFormat.Get()(cmd.OutOrStdout(), policyRemoveResult{
				OrganizationName: policyPack.Ref().OrgName(),
				Name:             string(policyPack.Ref().Name()),
				VersionTag:       version,
			})
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "policy-pack", Usage: "<org-name>/<policy-pack-name>"},
			{Name: "version", Usage: "<all|version>"},
		},
		Required: 2,
	})

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with removal anyway")

	outputflag.VarP(cmd.Flags(), &outputFormat)
	return cmd
}
