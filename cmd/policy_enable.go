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

package cmd

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/spf13/cobra"
)

const latestKeyword = "latest"

type policyEnableArgs struct {
	policyGroup string
}

func newPolicyEnableCmd() *cobra.Command {
	args := policyEnableArgs{}

	var cmd = &cobra.Command{
		Use:   "enable <org-name>/<policy-pack-name> <latest|version>",
		Args:  cmdutil.ExactArgs(2),
		Short: "Enable a Policy Pack for a Pulumi organization",
		Long: "Enable a Policy Pack for a Pulumi organization. " +
			"Can specify latest to enable the latest version of the Policy Pack or a specific version number.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cliArgs []string) error {
			// Obtain current PolicyPack, tied to the Pulumi service backend.
			policyPack, err := requirePolicyPack(cliArgs[0])
			if err != nil {
				return err
			}

			// Parse version if it's specified.
			var version *int
			if cliArgs[1] != latestKeyword {
				v, err := strconv.Atoi(cliArgs[1])
				if err != nil {
					return errors.Wrapf(err, "Could not parse version (should be an integer)")
				}
				version = &v
			}

			// Attempt to enable the Policy Pack.
			return policyPack.Enable(commandContext(), args.policyGroup, backend.PolicyPackOperation{
				Version: version, Scopes: cancellationScopes})
		}),
	}

	cmd.PersistentFlags().StringVar(
		&args.policyGroup, "policy-group", "",
		"The Policy Group for which the Policy Pack will be enabled; if not specified, the default Policy Group is used")

	return cmd
}
