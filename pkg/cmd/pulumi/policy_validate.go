// Copyright 2016-2020, Pulumi Corporation.
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

package main

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type PolicyValidateArgs struct {
	Config string `argsUsage:"The file path for the Policy Pack configuration file"`
}

func newPolicyValidateCmd(
	v *viper.Viper,
	parentPolicyCmd *cobra.Command,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-config <org-name>/<policy-pack-name> <version>",
		Args:  cmdutil.ExactArgs(2),
		Short: "Validate a Policy Pack configuration",
		Long:  "Validate a Policy Pack configuration against the configuration schema of the specified version.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cliArgs []string) error {
			args := UnmarshalArgs[PolicyValidateArgs](v, cmd)

			ctx := cmd.Context()
			// Obtain current PolicyPack, tied to the Pulumi Cloud backend.
			policyPack, err := requirePolicyPack(ctx, cliArgs[0], loginToCloud)
			if err != nil {
				return err
			}

			// Get version from cmd argument
			version := &cliArgs[1]

			// Load the configuration from the user-specified JSON file into jsonConfig object.
			var jsonConfig map[string]*json.RawMessage
			if args.Config != "" {
				jsonConfig, err = loadPolicyConfigFromFile(args.Config)
				if err != nil {
					return err
				}
			}

			err = policyPack.Validate(ctx,
				backend.PolicyPackOperation{
					VersionTag: version,
					Scopes:     backend.CancellationScopes,
					Config:     jsonConfig,
				})
			if err != nil {
				return err
			}
			fmt.Println("Policy Pack configuration is valid.")
			return nil
		}),
	}

	parentPolicyCmd.AddCommand(cmd)
	BindFlags[PolicyValidateArgs](v, cmd)

	contract.AssertNoErrorf(cmd.MarkPersistentFlagRequired("config"), `Could not mark "config" as required`)

	return cmd
}
