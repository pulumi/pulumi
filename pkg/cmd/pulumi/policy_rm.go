// Copyright 2016-2018, Pulumi Corporation.
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
	"fmt"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/spf13/cobra"
)

const allKeyword = "all"

func newPolicyRmCmd() *cobra.Command {
	var yes bool
	var cmd = &cobra.Command{
		Use:   "rm <org-name>/<policy-pack-name> <all|version>",
		Args:  cmdutil.ExactArgs(2),
		Short: "Removes a Policy Pack from a Pulumi organization",
		Long: "Removes a Policy Pack from a Pulumi organization. " +
			"The Policy Pack must be disabled from all Policy Groups before it can be removed.",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			yes = yes || skipConfirmations()
			// Obtain current PolicyPack, tied to the Pulumi service backend.
			policyPack, err := requirePolicyPack(args[0])
			if err != nil {
				return result.FromError(err)
			}

			var version *string
			if args[1] != allKeyword {
				version = &args[1]
			}

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			prompt := fmt.Sprintf("This will permanently remove the '%s' policy!", args[0])
			if !yes && !confirmPrompt(prompt, args[0], opts) {
				fmt.Println("confirmation declined")
				return result.Bail()
			}

			// Attempt to remove the Policy Pack.
			err = policyPack.Remove(commandContext(), backend.PolicyPackOperation{
				VersionTag: version, Scopes: cancellationScopes})
			if err != nil {
				return result.FromError(err)
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with removal anyway")

	return cmd
}
