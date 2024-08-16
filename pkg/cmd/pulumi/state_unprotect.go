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
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/edit"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type StateUnprotectArgs struct {
	UnprotectAll bool   `args:"all" argsUsage:"Unprotect all resources in the checkpoint"`
	Stack        string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
	Yes          bool   `argsShort:"y" argsUsage:"Skip confirmation prompts"`
}

func newStateUnprotectCommand(
	v *viper.Viper,
	parentStateCmd *cobra.Command,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unprotect [resource URN]",
		Short: "Unprotect resources in a stack's state",
		Long: `Unprotect resource in a stack's state

This command clears the 'protect' bit on one or more resources, allowing those resources to be deleted.

To see the list of URNs in a stack, use ` + "`pulumi stack --show-urns`" + `.`,
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cmdArgs []string) error {
			args := UnmarshalArgs[StateUnprotectArgs](v, cmd)

			ctx := cmd.Context()
			args.Yes = args.Yes || skipConfirmations()
			// Show the confirmation prompt if the user didn't pass the --yes parameter to skip it.
			showPrompt := !args.Yes

			if args.UnprotectAll {
				return unprotectAllResources(ctx, args.Stack, showPrompt)
			}

			var urn resource.URN

			if len(cmdArgs) != 1 {
				if !cmdutil.Interactive() {
					return missingNonInteractiveArg("resource URN")
				}
				var err error
				urn, err = getURNFromState(ctx, args.Stack, nil, "Select a resource to unprotect:")
				if err != nil {
					return fmt.Errorf("failed to select resource: %w", err)
				}
			} else {
				urn = resource.URN(cmdArgs[0])
			}
			return unprotectResource(ctx, args.Stack, urn, showPrompt)
		}),
	}

	parentStateCmd.AddCommand(cmd)
	BindFlags[StateUnprotectArgs](v, cmd)

	return cmd
}

func unprotectAllResources(ctx context.Context, stackName string, showPrompt bool) error {
	err := runTotalStateEdit(ctx, stackName, showPrompt, func(_ display.Options, snap *deploy.Snapshot) error {
		// Protects against Panic when a user tries to unprotect non-existing resources
		if snap == nil {
			return errors.New("no resources found to unprotect")
		}

		for _, res := range snap.Resources {
			err := edit.UnprotectResource(snap, res)
			contract.AssertNoErrorf(err, "Unable to unprotect resource %q", res.URN)
		}

		return nil
	})
	if err != nil {
		return err
	}
	fmt.Println("All resources unprotected")
	return nil
}

func unprotectResource(ctx context.Context, stackName string, urn resource.URN, showPrompt bool) error {
	err := runStateEdit(ctx, stackName, showPrompt, urn, edit.UnprotectResource)
	if err != nil {
		return err
	}
	fmt.Println("Resource unprotected")
	return nil
}
