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

package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/resource/deploy"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/edit"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"

	"github.com/spf13/cobra"
)

func newStateUnprotectCommand() *cobra.Command {
	var unprotectAll bool
	var stack string

	cmd := &cobra.Command{
		Use:   "unprotect",
		Short: "Unprotect resources in a stack's state",
		Long: `Unprotect resource in a stack's state

This command clears the 'protect' bit on one or more resources, allowing those resources to be deleted.`,
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if unprotectAll {
				return unprotectAllResources(stack)
			}

			if len(args) != 1 {
				return errors.New("must provide a URN corresponding to a resource")
			}

			urn := resource.URN(args[0])
			return unprotectResource(stack, urn)
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVar(&unprotectAll, "all", false, "Unprotect all resources in the checkpoint")
	return cmd
}

func unprotectAllResources(stackName string) error {
	err := runTotalStateEdit(stackName, func(_ display.Options, snap *deploy.Snapshot) error {
		for _, res := range snap.Resources {
			err := edit.UnprotectResource(snap, res)
			contract.AssertNoError(err)
		}

		return nil
	})

	if err != nil {
		return err
	}
	fmt.Println("All resources successfully unprotected")
	return nil
}

func unprotectResource(stackName string, urn resource.URN) error {
	err := runStateEdit(stackName, urn, edit.UnprotectResource)
	if err != nil {
		return err
	}
	fmt.Println("Resource successfully unprotected")
	return nil
}
