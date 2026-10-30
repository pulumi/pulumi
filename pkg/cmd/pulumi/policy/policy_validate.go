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
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/spf13/cobra"
)

type policyValidateResult struct {
	OrganizationName string `json:"organizationName"`
	Name             string `json:"name"`
	VersionTag       string `json:"versionTag"`
	Valid            bool   `json:"valid"`
}

type policyValidateRenderFunc func(w io.Writer, result policyValidateResult) error

func renderPolicyValidateText(w io.Writer, result policyValidateResult) error {
	fmt.Fprintf(w, "Policy pack %s/%s version %s configuration is valid.\n",
		result.OrganizationName, result.Name, result.VersionTag)
	return nil
}

func renderPolicyValidateJSON(w io.Writer, result policyValidateResult) error {
	return ui.FprintJSON(w, result)
}

func newPolicyValidateCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
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
			policyPack, err := requirePolicyPack(ctx, ws, lm, cliArgs[0])
			if err != nil {
				return err
			}

			// Get version from cmd argument
			version := &cliArgs[1]

			// Load the configuration from the user-specified JSON file into config object.
			var file policyConfigFile
			if argConfig != "" {
				file, err = loadPolicyConfigFromFile(argConfig)
				if err != nil {
					return err
				}
			}

			err = policyPack.Validate(ctx,
				backend.PolicyPackOperation{
					VersionTag: version,
					Scopes:     backend.CancellationScopes,
					Config:     file.config,
				})
			if err != nil {
				return err
			}
			return outputFormat.Get()(cmd.OutOrStdout(), policyValidateResult{
				OrganizationName: policyPack.Ref().OrgName(),
				Name:             string(policyPack.Ref().Name()),
				VersionTag:       *version,
				Valid:            true,
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
