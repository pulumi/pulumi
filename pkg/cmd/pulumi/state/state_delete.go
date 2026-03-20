// Copyright 2016, Pulumi Corporation.
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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/edit"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"

	"github.com/spf13/cobra"
)

func newStateDeleteCommand(ws pkgWorkspace.Context, lm backend.LoginManager) *cobra.Command {
	var force bool // Force deletion of protected resources
	var stack string
	var yes bool
	var targetDependents bool
	var all bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes one or more resources from a stack's state",
		Long: `Deletes one or more resources from a stack's state

This command deletes resources from a stack's state, as long as it is safe to do so. Each resource is specified
by its Pulumi URN, or by a unique case-insensitive substring of its URN when the argument is not a valid URN.
If no resource is specified, this command will prompt for one.

Resources can't be deleted if other resources depend on it or are parented to it. Protected resources
will not be deleted unless specifically requested using the --force flag.

Make sure that URNs are single-quoted to avoid having characters unexpectedly interpreted by the shell.

To see the list of URNs in a stack, use ` + "`pulumi stack --show-urns`" + `.
`,
		Example: "pulumi state delete 'urn:pulumi:stage::demo::pkg:index:Type::res-a' " +
			"'urn:pulumi:stage::demo::pkg:index:Type::res-b'",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sink := cmdutil.Diag()
			yes = yes || env.SkipConfirmations.Value()
			if all {
				if len(args) != 0 {
					return errors.New("cannot specify a resource URN when deleting all resources")
				}
			}
			// Show the confirmation prompt if the user didn't pass the --yes parameter to skip it.
			showPrompt := !yes
			nDeleted := 0

			var handleProtected func(*resource.State) error
			if force {
				handleProtected = func(res *resource.State) error {
					cmdutil.Diag().Warningf(diag.Message(res.URN,
						"deleting protected resource %s due to presence of --force"), res.URN)
					res.Protect = false
					return nil
				}
			}

			var err error
			if all {
				err = runTotalStateEdit(ctx, sink, ws, lm, stack, showPrompt,
					func(opts display.Options, snap *deploy.Snapshot) error {
						// Iterate the resources backwards (so we delete dependents first) and delete them.
						for i := len(snap.Resources) - 1; i >= 0; i-- {
							res := snap.Resources[i]
							if err := edit.DeleteResource(snap, res, handleProtected, targetDependents); err != nil {
								return err
							}
						}
						return nil
					})
			} else {
				var urnArgs []string
				if len(args) == 0 {
					if !cmdutil.Interactive() {
						return missingNonInteractiveArg("resource URN")
					}
					urn, selErr := getURNFromState(ctx, sink, ws, backend.DefaultLoginManager, stack, nil,
						"Select the resource to delete")
					if selErr != nil {
						return fmt.Errorf("failed to select resource: %w", selErr)
					}
					urnArgs = []string{string(urn)}
				} else {
					urnArgs = args
				}
				nDeleted, err = runStateDeleteResources(
					ctx, sink, ws, lm, stack, showPrompt, urnArgs, handleProtected, targetDependents)
			}
			if err != nil {
				switch e := err.(type) {
				case edit.ResourceHasDependenciesError:
					var message strings.Builder
					message.WriteString(string(e.Condemned.URN))
					message.WriteString(" can't be safely deleted because the following resources depend on it:\n")
					for _, dependentResource := range e.Dependencies {
						depUrn := dependentResource.URN
						message.WriteString(fmt.Sprintf(" * %-15q (%s)\n", depUrn.Name(), depUrn))
					}
					message.WriteString("\nDelete those resources first or pass --target-dependents.")
					return errors.New(message.String())
				case edit.ResourceProtectedError:
					return fmt.Errorf(
						"%s can't be safely deleted because it is protected. "+
							"Re-run this command with --force to force deletion", string(e.Condemned.URN))
				default:
					return err
				}
			}
			if all {
				fmt.Println("Resources deleted")
			} else if nDeleted == 1 {
				fmt.Println("Resource deleted")
			} else {
				fmt.Printf("%d resources deleted\n", nDeleted)
			}
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "resource-urn"},
		},
		Required: 0,
		Variadic: true,
	})

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().BoolVar(&force, "force", false, "Force deletion of protected resources")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&all, "all", false, "Delete all resources in the stack")
	cmd.Flags().BoolVar(&targetDependents, "target-dependents", false, "Delete the URN and all its dependents")
	return cmd
}

func runStateDeleteResources(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, lm backend.LoginManager,
	stackName string, showPrompt bool, args []string,
	handleProtected func(*resource.State) error, targetDependents bool,
) (int, error) {
	var deleted int
	err := runTotalStateEdit(ctx, sink, ws, lm, stackName, showPrompt,
		func(opts display.Options, snap *deploy.Snapshot) error {
			targets := make(map[*resource.State]struct{})
			for _, arg := range args {
				res, err := resolveStateResourceArg(opts, snap, arg)
				if err != nil {
					return err
				}
				targets[res] = struct{}{}
			}
			order, err := computeStateDeleteOrder(snap, targets)
			if err != nil {
				return err
			}
			for _, res := range order {
				if err := edit.DeleteResource(snap, res, handleProtected, targetDependents); err != nil {
					return err
				}
			}
			deleted = len(order)
			return nil
		})
	return deleted, err
}

func computeStateDeleteOrder(
	snap *deploy.Snapshot, targets map[*resource.State]struct{},
) ([]*resource.State, error) {
	dg := graph.NewDependencyGraph(snap.Resources)
	remaining := make(map[*resource.State]struct{}, len(targets))
	for k := range targets {
		remaining[k] = struct{}{}
	}
	order := make([]*resource.State, 0, len(targets))
	for len(remaining) > 0 {
		var next *resource.State
		for res := range remaining {
			deps := dg.OnlyDependsOn(res)
			ok := true
			for _, d := range deps {
				if _, stillThere := remaining[d]; stillThere {
					ok = false
					break
				}
			}
			if ok {
				next = res
				break
			}
		}
		if next == nil {
			return nil, errors.New(
				"could not order deletions: selected resources have cyclic or conflicting dependencies; " +
					"try deleting them one at a time")
		}
		order = append(order, next)
		delete(remaining, next)
	}
	return order, nil
}
