// Copyright 2016-2022, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/edit"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"

	"github.com/spf13/cobra"
)

func updateDependencies(dependencies []resource.URN, oldUrn resource.URN, newUrn resource.URN) []resource.URN {
	var updatedDependencies []resource.URN
	for _, dependency := range dependencies {
		if dependency == oldUrn {
			// replace old URN with new URN
			updatedDependencies = append(updatedDependencies, newUrn)
		} else {
			updatedDependencies = append(updatedDependencies, dependency)
		}
	}
	return updatedDependencies
}

func newStateRenameCommand() *cobra.Command {
	var stack string
	var yes bool

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
			ctx := cmd.Context()
			yes = yes || skipConfirmations()
			urn := resource.URN(args[0])
			newResourceName := args[1]
			// Show the confirmation prompt if the user didn't pass the --yes parameter to skip it.
			showPrompt := !yes

			if !urn.IsValid() {
				return result.Error("The provided input URN is not valid")
			}

			res := runTotalStateEdit(ctx, stack, showPrompt, func(opts display.Options, snap *deploy.Snapshot) error {
				// Check whether the input URN corresponds to an existing resource
				existingResources := edit.LocateResource(snap, urn)
				if len(existingResources) != 1 {
					return errors.New("The input URN does not correspond to an existing resource")
				}

				inputResource := existingResources[0]
				oldUrn := inputResource.URN
				// update the URN with only the name part changed
				newUrn := oldUrn.Rename(newResourceName)
				// Check whether the new URN _does not_ correspond to an existing resource
				candidateResources := edit.LocateResource(snap, newUrn)
				if len(candidateResources) > 0 {
					return errors.New("The chosen new name for the state corresponds to an already existing resource")
				}

				// Update the URN of the input resource
				inputResource.URN = newUrn
				// Update the dependants of the input resource
				for _, existingResource := range snap.Resources {
					// update resources other than the input resource
					if existingResource.URN != inputResource.URN {
						// Update dependencies
						existingResource.Dependencies = updateDependencies(existingResource.Dependencies, oldUrn, newUrn)
						// Update property dependencies
						for property, dependencies := range existingResource.PropertyDependencies {
							existingResource.PropertyDependencies[property] = updateDependencies(dependencies, oldUrn, newUrn)
						}
					}
				}

				return nil
			})

			if res != nil {
				// an error occurred
				// return it
				return res
			}

			fmt.Println("Resource renamed")
			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	return cmd
}
