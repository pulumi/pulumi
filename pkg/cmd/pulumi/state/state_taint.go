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

// stateTaintResult is the shape of the --json output for the state taint command.
type stateTaintResult struct {
	Operation string   `json:"operation"`
	Resources []string `json:"resources"`
	Count     int      `json:"count"`
	Warnings  []string `json:"warnings,omitempty"`
}

func newStateTaintCommand() *cobra.Command {
	var stack string
	var yes bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "taint",
		Short: "Taint one or more resources in the stack's state",
		Long: `Taint one or more resources in the stack's state.

This has the effect of ensuring the resources are destroyed and recreated upon the next ` + "`pulumi up`" + `.

To see the list of URNs in a stack, use ` + "`pulumi stack --show-urns`" + `.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sink := cmdutil.Diag()
			ws := pkgWorkspace.Instance
			yes = yes || env.SkipConfirmations.Value()

			// Show the confirmation prompt if the user didn't pass the --yes parameter to skip it.
			showPrompt := !yes

			// If URN arguments were provided, use those:
			if len(args) > 0 {
				return taintMultipleResources(ctx, sink, ws, stack, args, showPrompt, jsonOut)
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
				"Select a resource to taint:",
			)
			if err != nil {
				return fmt.Errorf("failed to select resource: %w", err)
			}

			return taintResource(ctx, sink, ws, stack, urn, showPrompt, jsonOut)
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
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")

	return cmd
}

// taintResourcesInSnapshot handles the logic for tainting resources in a snapshot.
// Returns the list of tainted URNs, the count, and any errors encountered.
func taintResourcesInSnapshot(snap *deploy.Snapshot, urns []string) ([]string, int, []error) {
	if snap == nil {
		return nil, 0, []error{errors.New("no resources found to taint")}
	}

	var errs []error
	var taintedURNs []string

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
			res.Taint = true
			taintedURNs = append(taintedURNs, urnStr)
		} else {
			errs = append(errs, fmt.Errorf("No such resource %q exists in the current state", urn))
		}
	}

	return taintedURNs, len(taintedURNs), errs
}

// taintMultipleResources taints multiple resources specified by their URNs.
func taintMultipleResources(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stackName string, urns []string,
	showPrompt bool, jsonOut bool,
) error {
	var taintedURNs []string
	var warnings []string

	err := runTotalStateEdit(
		ctx, sink, ws, backend.DefaultLoginManager, stackName, showPrompt,
		func(_ display.Options, snap *deploy.Snapshot) error {
			var errs []error
			taintedURNs, _, errs = taintResourcesInSnapshot(snap, urns)

			if len(errs) > 0 {
				for _, err := range errs {
					warnings = append(warnings, err.Error())
				}
				// If no resources were tainted, return an error
				if len(taintedURNs) == 0 {
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
		return ui.FprintJSON(os.Stdout, stateTaintResult{
			Operation: "taint",
			Resources: taintedURNs,
			Count:     len(taintedURNs),
			Warnings:  warnings,
		})
	}

	fmt.Printf("%d resources tainted\n", len(taintedURNs))
	return nil
}

func taintResource(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stackName string, urn resource.URN,
	showPrompt bool, jsonOut bool,
) error {
	return taintMultipleResources(ctx, sink, ws, stackName, []string{string(urn)}, showPrompt, jsonOut)
}
