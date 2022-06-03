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
	"fmt"

	survey "github.com/AlecAivazis/survey/v2"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"

	"github.com/spf13/cobra"
)

func newStatePendingCommand() *cobra.Command {
	var stack string
	var yes bool

	cmd := &cobra.Command{
		Use:   "pending",
		Short: "Interactively fix pending operations in the stack's state",
		Long: `Interactively fix pending operations in the stack's state.

Subcommands of this command can be used to cancel or resolve pending creates`,
		Args: cmdutil.NoArgs,
	}

	cmd.AddCommand(newStatePendingClearCreate(&yes, &stack))
	cmd.AddCommand(newStatePendingResolveCreate(&yes, &stack))

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")

	return cmd
}

func newStatePendingResolveCreate(yes *bool, stack *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve-create",
		Short: "Interactively resolve pending create operations",
		Args:  cmdutil.MaximumNArgs(2),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			yes := *yes || skipConfirmations()
			showPrompt := !yes

			var urn string
			var id string
			var pending []string

			if len(args) >= 1 {
				urn = args[0]
			} else {
				// No urn provided, so get the urn
				res := runTotalStateEdit(*stack, false, func(opts display.Options, snap *deploy.Snapshot) error {
					for _, op := range snap.PendingOperations {
						if op.Type == resource.OperationTypeCreating {
							pending = append(pending, string(op.Resource.URN))
						}
					}
					return nil
				})
				if res != nil {
					return res
				}
				if len(pending) == 0 {
					return result.Error("no pending creates found")
				}
				prompt := &survey.Select{
					Message: "Choose a pending create to resolve",
					Options: pending,
				}
				err := survey.AskOne(prompt, &urn, survey.WithValidator(survey.Required))
				if err != nil {
					return result.FromError(err)
				}
			}

			if len(args) == 2 {
				id = args[1]
			} else {
				// No id provided, so get the id
				prompt := &survey.Input{
					Message: fmt.Sprintf("the id for '%s'", urn),
				}
				err := survey.AskOne(prompt, &id, survey.WithValidator(survey.Required))
				if err != nil {
					return result.FromError(err)
				}
			}

			return runTotalStateEdit(*stack, showPrompt, func(opts display.Options, snap *deploy.Snapshot) error {
				for i, op := range snap.PendingOperations {
					if op.Resource.URN == resource.URN(urn) {
						if op.Type != resource.OperationTypeCreating {
							return fmt.Errorf("'%s' corresponds to a pending %s operation, not a pending create operation", urn, op.Type)
						}
						op.Type = resource.OperationTypeImporting
						op.Resource.ID = resource.ID(id)
						snap.PendingOperations[i] = op
						return nil
					}
				}
				return fmt.Errorf("could not find a pending create with urn '%s'", urn)
			})
		}),
	}
	return cmd
}

func newStatePendingClearCreate(yes *bool, stack *string) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "clear-create",
		Short: "Interactively remove pending operations in the stacks state",
		Long: `Interactively remove pending operations in the stacks state.

This tells pulumi that the resources don't exist and so pulumi does not need to keep track of them.`,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {

			yes := *yes || skipConfirmations()
			showPrompt := !yes
			var numDeleted int
			var hasPending bool
			res := runTotalStateEdit(*stack, false, func(opts display.Options, snap *deploy.Snapshot) error {
				for _, op := range snap.PendingOperations {
					if op.Type == resource.OperationTypeCreating {
						hasPending = true
						break
					}
				}
				return nil
			})
			if res != nil {
				return res
			}
			if !hasPending {
				fmt.Printf("No pending creates detected. Other pending operations will be removed by `pulumi refresh`.\n")
				return nil
			}
			res = runTotalStateEdit(*stack, showPrompt, func(opts display.Options, snap *deploy.Snapshot) error {
				ops := []resource.Operation{}
				if all {
					for _, op := range snap.PendingOperations {
						if op.Type != resource.OperationTypeCreating {
							ops = append(ops, op)
						}
					}
				} else {
					resourceList := []string{}
					for _, op := range snap.PendingOperations {
						if op.Type == resource.OperationTypeCreating {
							resourceList = append(resourceList, string(op.Resource.URN))
						}
					}
					chosen := []string{}
					prompt := &survey.MultiSelect{
						Message: "Select pending creates to clear",
						Options: resourceList,
					}
					err := survey.AskOne(prompt, &chosen, survey.WithValidator(survey.Required))
					if err != nil {
						return err
					}
					chosenSet := make(map[string]bool, len(chosen))
					for _, k := range chosen {
						chosenSet[k] = true
					}
					for _, op := range snap.PendingOperations {
						if !chosenSet[string(op.Resource.URN)] {
							ops = append(ops, op)
						}
					}
				}
				numDeleted = len(snap.PendingOperations) - len(ops)
				snap.PendingOperations = ops
				return nil
			})
			if res != nil {
				return res
			}
			fmt.Printf("%d pending creates removed\n", numDeleted)
			return nil
		}),
	}

	cmd.Flags().BoolVarP(&all, "all", "", false, "Mark all pending creates as unsuccessful")
	return cmd
}
