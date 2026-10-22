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
	"encoding/json"
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

type policyValidateResult struct {
	Organization string `json:"organization"`
	PolicyPack   string `json:"policyPack"`
	VersionTag   string `json:"versionTag"`
	Valid        bool   `json:"valid"`
}

type policyValidateRenderFunc func(w io.Writer, result policyValidateResult) error

func renderPolicyValidateText(w io.Writer, result policyValidateResult) error {
	fmt.Fprintln(w, "Policy Pack configuration is valid.")
	return nil
}

func renderPolicyValidateJSON(w io.Writer, result policyValidateResult) error {
	return ui.FprintJSON(w, result)
}

func newPolicyValidateCmd() *cobra.Command {
	return newPolicyValidateCmdWith(defaultPolicyPackCmdDeps)
}

func newPolicyValidateCmdWith(deps policyPackCmdDeps) *cobra.Command {
	contract.Assertf(deps != nil, "policyPackCmdDeps must not be nil")
	var argConfig string
	outputFormat := outputflag.OutputFlag[policyValidateRenderFunc]{
		RenderForTerminal: renderPolicyValidateText,
		RenderJSON:        renderPolicyValidateJSON,
	}

	cmd := &cobra.Command{
		Use:   "validate-config",
		Short: "Validate a Policy Pack configuration",
		Long: "Validate a Policy Pack configuration against the configuration schema of the specified version.\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi policy validate-config acme/aws-guardrails 3 --config config.json\n" +
			"  pulumi policy validate-config acme/aws-guardrails 3 --config config.json --output json",
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			// Obtain current PolicyPack, tied to the Pulumi Cloud backend.
			policyPack, _, err := deps(ctx, cliArgs[0])
			if err != nil {
				return err
			}

			// Get version from cmd argument
			version := &cliArgs[1]

			// Load the configuration from the user-specified JSON file into config object.
			var config map[string]*json.RawMessage
			if argConfig != "" {
				config, err = loadPolicyConfigFromFile(argConfig)
				if err != nil {
					return err
				}
			}

			err = policyPack.Validate(ctx,
				backend.PolicyPackOperation{
					VersionTag: version,
					Scopes:     backend.CancellationScopes,
					Config:     config,
				})
			if err != nil {
				return err
			}
			return outputFormat.Get()(cmd.OutOrStdout(), policyValidateResult{
				Organization: policyPack.Ref().OrgName(),
				PolicyPack:   string(policyPack.Ref().Name()),
				VersionTag:   *version,
				Valid:        true,
			})
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "policy-pack", Usage: "<org-name>/<policy-pack-name>"},
			{Name: "version"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&argConfig, "config", "",
		"The file path for the Policy Pack configuration file")
	cmd.MarkFlagRequired("config") //nolint:errcheck

	outputflag.VarP(cmd.Flags(), &outputFormat)
	return cmd
}
