// Copyright 2025, Pulumi Corporation.
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

package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/edit"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/spf13/cobra"
)

const protectMessage = "This will protect by modifying the stack state directly.\n" +
	"If your program does not also set the 'protect' resource option, Pulumi will unprotect the \n" +
	"resource the next time your program runs (e.g. as part of a `pulumi up`)\n" +
	"Confirm?"

func newStateProtectCommand() *cobra.Command {
	var protectAll bool
	var stack string
	var yes bool

	cmd := &cobra.Command{
		Use:   "protect [resource URN]",
		Short: "protect resource in a stack's state",
		Long: `Protect resource in a stack's state

This command sets the 'protect' bit on one or more resources, preventing those resources from being deleted.

Caution: this command is a low-level operation that directly modifies your stack's state.
Setting the 'protect' bit on a resource in your stack's state is not sufficient to protect it in
all cases. If your program does not also set the 'protect' resource option, Pulumi will
unprotect the resource the next time your program runs (e.g. as part of a ` + "`pulumi up`" + `).

See https://www.pulumi.com/docs/iac/concepts/options/protect/ for more information on
the 'protect' resource option and how it can be used to protect resources in your program.

To unprotect a resource, use ` + "`pulumi unprotect`" + `on the resource URN.

To see the list of URNs in a stack, use ` + "`pulumi stack --show-urns`" + `.`,
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			yes = yes || env.SkipConfirmations.Value()
			// Show the confirmation prompt if the user didn't pass the --yes parameter to skip it.
			showPrompt := !yes

			if protectAll {
				return protectAllResources(ctx, ws, stack, showPrompt)
			}

			var urn resource.URN

			if len(args) != 1 {
				if !cmdutil.Interactive() {
					return missingNonInteractiveArg("resource URN")
				}
				var err error
				urn, err = getURNFromState(ctx, ws, backend.DefaultLoginManager, stack, nil, "Select a resource to protect:")
				if err != nil {
					return fmt.Errorf("failed to select resource: %w", err)
				}
			} else {
				urn = resource.URN(args[0])
			}
			return protectResource(ctx, ws, stack, urn, showPrompt)
		},
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVar(&protectAll, "all", false, "Protect all resources in the checkpoint")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")

	return cmd
}

func protectAllResources(ctx context.Context, ws pkgWorkspace.Context, stackName string, showPrompt bool) error {
	err := runTotalStateEditWithPrompt(
		ctx, ws, backend.DefaultLoginManager, stackName, showPrompt, func(_ display.Options, snap *deploy.Snapshot) error {
			// Protects against Panic when a user tries to protect non-existing resources
			if snap == nil {
				return errors.New("no resources found to protect")
			}

			for _, res := range snap.Resources {
				err := edit.ProtectResource(snap, res)
				contract.AssertNoErrorf(err, "Unable to protect resource %q", res.URN)
			}

			return nil
		}, protectMessage)
	if err != nil {
		return err
	}
	fmt.Println("All resources protected")
	return nil
}

func protectResource(
	ctx context.Context, ws pkgWorkspace.Context, stackName string, urn resource.URN, showPrompt bool,
) error {
	err := runStateEditWithPrompt(
		ctx,
		ws,
		backend.DefaultLoginManager,
		stackName,
		showPrompt,
		urn,
		edit.ProtectResource,
		protectMessage,
	)
	if err != nil {
		return err
	}
	fmt.Println("Resource protected")
	return nil
}
