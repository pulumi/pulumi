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
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"

	"github.com/spf13/cobra"
)

// stateUntaintResult is the shape of the --json output for the state untaint command.
type stateUntaintResult struct {
	Operation string   `json:"operation"`
	Resources []string `json:"resources"`
	Count     int      `json:"count"`
	Warnings  []string `json:"warnings,omitempty"`
}

func newStateUntaintCommand() *cobra.Command {
	var untaintAll bool
	var stack string
	var yes bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "untaint",
		Short: "Untaint one or more resources in the stack's state",
		Long: `Untaint one or more resources in the stack's state.

After running this, the resources will no longer be destroyed and recreated upon the next ` + "`pulumi up`" + `.

To see the list of URNs in a stack, use ` + "`pulumi stack --show-urns`" + `.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sink := cmdutil.Diag()
			ws := pkgWorkspace.Instance
			yes = yes || env.SkipConfirmations.Value()

			// Show the confirmation prompt if the user didn't pass the --yes parameter to skip it.
			showPrompt := !yes

			if untaintAll {
				return untaintAllResources(ctx, sink, ws, stack, showPrompt, jsonOut)
			}

			// If URN arguments were provided, use those:
			if len(args) > 0 {
				return untaintMultipleResources(ctx, sink, ws, stack, args, showPrompt, jsonOut)
			}

			// Otherwise, use interactive selection:
			if !cmdutil.Interactive() {
				return missingNonInteractiveArg("resource URN")
			}

			urn, err := getURNFromState(
				ctx,
				sink,
				ws,
				backend.DefaultLoginManager,
				stack,
				nil,
				"Select a resource to untaint:",
			)
			if err != nil {
				return fmt.Errorf("failed to select resource: %w", err)
			}

			return untaintResource(ctx, sink, ws, stack, urn, showPrompt, jsonOut)
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
	cmd.Flags().BoolVar(&untaintAll, "all", false, "Untaint all resources in the checkpoint")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")

	return cmd
}

func untaintAllResources(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stackName string, showPrompt bool, jsonOut bool,
) error {
	var untaintedURNs []string
	err := runTotalStateEdit(
		ctx, sink, ws, backend.DefaultLoginManager, stackName, showPrompt,
		func(_ display.Options, snap *deploy.Snapshot) error {
			// Protects against Panic when a user tries to untaint non-existing resources
			if snap == nil {
				return errors.New("no resources found to untaint")
			}

			for _, res := range snap.Resources {
				// Skip resources that are pending deletion
				if !res.Delete {
					res.Taint = false
					untaintedURNs = append(untaintedURNs, string(res.URN))
				}
			}

			return nil
		})
	if err != nil {
		return err
	}

	if jsonOut {
		return ui.FprintJSON(os.Stdout, stateUntaintResult{
			Operation: "untaint",
			Resources: untaintedURNs,
			Count:     len(untaintedURNs),
		})
	}

	fmt.Println("All resources untainted")
	return nil
}

// untaintResourcesInSnapshot handles the logic for untainting resources in a snapshot.
// Returns the list of untainted URNs, the count, and any errors encountered.
func untaintResourcesInSnapshot(snap *deploy.Snapshot, urns []string) ([]string, int, []error) {
	if snap == nil {
		return nil, 0, []error{errors.New("no resources found to untaint")}
	}

	var errs []error
	var untaintedURNs []string

	// Build a map of URNs to resources, excluding those pending deletion.
	urnToResource := make(map[resource.URN]*resource.State)
	for _, res := range snap.Resources {
		if !res.Delete {
			urnToResource[res.URN] = res
		}
	}

	for _, urnStr := range urns {
		urn := resource.URN(urnStr)
		res, found := urnToResource[urn]

		if found {
			res.Taint = false
			untaintedURNs = append(untaintedURNs, urnStr)
		} else {
			errs = append(errs, fmt.Errorf("No such resource %q exists in the current state", urn))
		}
	}

	return untaintedURNs, len(untaintedURNs), errs
}

// untaintMultipleResources untaints multiple resources specified by their URNs.
func untaintMultipleResources(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stackName string, urns []string,
	showPrompt bool, jsonOut bool,
) error {
	var untaintedURNs []string
	var warnings []string

	err := runTotalStateEdit(
		ctx, sink, ws, backend.DefaultLoginManager, stackName, showPrompt,
		func(_ display.Options, snap *deploy.Snapshot) error {
			var errs []error
			untaintedURNs, _, errs = untaintResourcesInSnapshot(snap, urns)

			if len(errs) > 0 {
				for _, err := range errs {
					warnings = append(warnings, err.Error())
				}
				// If no resources were untainted, return an error
				if len(untaintedURNs) == 0 {
					var errMsgs []string
					for _, err := range errs {
						errMsgs = append(errMsgs, err.Error())
					}
					return errors.New(strings.Join(errMsgs, "\n"))
				}
			}

			return nil
		})
	if err != nil {
		return err
	}

	if jsonOut {
		return ui.FprintJSON(os.Stdout, stateUntaintResult{
			Operation: "untaint",
			Resources: untaintedURNs,
			Count:     len(untaintedURNs),
			Warnings:  warnings,
		})
	}

	fmt.Printf("%d resources untainted\n", len(untaintedURNs))
	return nil
}

func untaintResource(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stackName string, urn resource.URN,
	showPrompt bool, jsonOut bool,
) error {
	return untaintMultipleResources(ctx, sink, ws, stackName, []string{string(urn)}, showPrompt, jsonOut)
}
