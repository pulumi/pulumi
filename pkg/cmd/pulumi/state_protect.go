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

	"github.com/pulumi/pulumi/pkg/v2/backend/display"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v2/resource/edit"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"

	"github.com/spf13/cobra"
)

func newStateProtectCommand() *cobra.Command {
	var protectAll bool
	var stack string
	var yes bool

	cmd := &cobra.Command{
		Use:   "protect <resource URN>",
		Short: "Protect resources in a stack's state",
		Long: `Protect resource in a stack's state

This command adds the 'protect' bit on one or more resources, ensuring those resources cannot be deleted.

Any resources protected in this manner will need to have the protect resource option added to their code.
For more information, please visit https://www.pulumi.com/docs/intro/concepts/resources/#protect
`,
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			yes = yes || skipConfirmations()
			// Show the confirmation prompt if the user didn't pass the --yes parameter to skip it.
			showPrompt := !yes

			if protectAll {
				return protectAllResources(stack, showPrompt)
			}

			if len(args) != 1 {
				return result.Error("must provide a URN corresponding to a resource")
			}

			urn := resource.URN(args[0])
			return protectResource(stack, urn, showPrompt)
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVar(&protectAll, "all", false, "Protect all resources in the checkpoint")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")

	return cmd
}

func protectAllResources(stackName string, showPrompt bool) result.Result {
	res := runTotalStateEdit(stackName, showPrompt, func(_ display.Options, snap *deploy.Snapshot) error {
		// Protects against Panic when a user tries to protect non-existing resources
		if snap == nil {
			return fmt.Errorf("no resources found to protect")
		}

		for _, res := range snap.Resources {
			err := edit.ProtectResource(snap, res)
			contract.AssertNoError(err)
		}

		return nil
	})

	if res != nil {
		return res
	}
	fmt.Println("All resources successfully protected. Please ensure application code has the correct " +
		"`protect` resource option on the resources")
	return nil
}

func protectResource(stackName string, urn resource.URN, showPrompt bool) result.Result {
	res := runStateEdit(stackName, showPrompt, urn, edit.ProtectResource)
	if res != nil {
		return res
	}
	fmt.Println("Resource successfully protected. Please ensure application code has the correct " +
		"`protect` resource option on the resource")
	return nil
}
