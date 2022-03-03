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
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"

	"github.com/spf13/cobra"
)

func newStateRenameCommand() *cobra.Command {
	var stack string
	var yes bool
	var force bool

	cmd := &cobra.Command{
		Use:   "rename <resource URN> <new name>",
		Short: "Renames a resource from a stack's state",
		Long: `Renames a resource from a stack's state

This command renames a resource from a stack's state. The resource is specified 
by its Pulumi URN (use ` + "`pulumi stack --show-urns`" + ` to get it) and the new name of the resource.

Make sure that URNs are single-quoted to avoid having characters unexpectedly interpreted by the shell.

Example:
pulumi state rename 'urn:pulumi:stage::demo::eks:index:Cluster$pulumi:providers:kubernetes::eks-provider' new-name-here
`,
		Args: cmdutil.ExactArgs(2),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			yes = yes || skipConfirmations()
			urn := resource.URN(args[0])
			newResourceName := args[1]
			// Show the confirmation prompt if the user didn't pass the --yes parameter to skip it.
			showPrompt := !yes

			res := runStateEdit(stack, showPrompt, urn, func(snap *deploy.Snapshot, resource *resource.State) error {

				// the resource is protected but the user didn't use --force
				if !force && resource.Protect {
					return errors.New(`Cannot rename a protected resource
You can either unprotect the resource first or use --force flag`)
				}

				if resource.Protect {
					cmdutil.Diag().Warningf(diag.RawMessage("" /*urn*/, "renaming protected resource due to presence of --force"))
					resource.Protect = false
				}

				// update the URN with only the name part changed
				resource.URN = resource.URN.Rename(newResourceName)

				return nil
			})

			if res != nil {
				// an error occurred
				// return it
				return res
			}

			fmt.Println("Resource renamed successfully")
			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	cmd.Flags().BoolVar(&force, "force", false, "Force rename of protected resources")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	return cmd
}
