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
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"

	"github.com/spf13/cobra"
)

func newStateIgnoreRefreshChangesCommand() *cobra.Command {
	var stack string
	var paths []string
	var yes bool

	cmd := &cobra.Command{
		Use:   "ignore-refresh-changes [resource URN] --path [property-path-1] ...",
		Short: "Applies ignoreRefreshChanges to the given URN and property paths",
		Long: `Applies ignoreRefreshChanges to the given URN and property paths

This command adds or updates the ignoreRefreshChanges resource option for given URN, setting it to the list of provided property paths.

Passing no property paths will clear the ignoreRefreshChanges option for the given URN.

To see the list of URNs in a stack, use ` + "`pulumi stack --show-urns`" + `.`,
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			yes = yes || skipConfirmations()
			// Show the confirmation prompt if the user didn't pass the --yes parameter to skip it.
			showPrompt := !yes

			var urn resource.URN

			if len(args) != 1 {
				if !cmdutil.Interactive() {
					return missingNonInteractiveArg("resource URN")
				}
				var err error
				urn, err = getURNFromState(ctx, stack, nil, "Select a URN to set ignoreRefreshChanges on:")
				if err != nil {
					return fmt.Errorf("failed to select resource: %w", err)
				}
			} else {
				urn = resource.URN(args[0])
			}
			return setIgnoreRefreshChangesOnResource(ctx, stack, urn, paths, showPrompt)
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringArrayVarP(
		&paths, "path", "p", []string{},
		"Path to the property to ignore during refresh. Can be specified multiple times")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")

	return cmd
}

func setIgnoreRefreshChangesOnResource(ctx context.Context, stackName string, urn resource.URN, paths []string, showPrompt bool) error {
	err := runStateEdit(ctx, stackName, showPrompt, urn, func(_ *deploy.Snapshot, res *resource.State) error {
		if res != nil {
			fmt.Printf("Changed ignoreRefreshChanges for resource %s from %v to %v\n", urn, res.IgnoreRefreshChanges, paths)
			res.IgnoreRefreshChanges = paths
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
