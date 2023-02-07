// Copyright 2016-2023, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"fmt"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/edit"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/spf13/cobra"
)

func newStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Edit the current stack's state",
		Long: `Edit the current stack's state

Subcommands of this command can be used to surgically edit parts of a stack's state. These can be useful when
troubleshooting a stack or when performing specific edits that otherwise would require editing the state file by hand.`,
		Args: cmdutil.NoArgs,
	}

	cmd.AddCommand(newStateDeleteCommand())
	cmd.AddCommand(newStateUnprotectCommand())
	cmd.AddCommand(newStateRenameCommand())
	return cmd
}

// locateStackResource attempts to find a unique resource associated with the given URN in the given snapshot.
func locateStackResource(opts display.Options, snap *deploy.Snapshot, urn resource.URN) (*resource.State, error) {
	candidateResources := edit.LocateResource(snap, urn)
	switch {
	case len(candidateResources) == 0: // resource was not found
		return nil, fmt.Errorf("No such resource %q exists in the current state", urn)
	case len(candidateResources) == 1: // resource was unambiguously found
		return candidateResources[0], nil
	}

	// Error: There exist multiple resources that have the requested URN.
	errorMsg := "Resource URN ambiguously referred to multiple resources:\n"
	for _, res := range candidateResources {
		errorMsg += fmt.Sprintf("  %s\n", res.ID)
	}
	return nil, errors.New(errorMsg)
}

// runStateEdit runs the given state edit function on a resource with the given URN in a given stack.
func runStateEdit(
	ctx context.Context, stackName string, showPrompt bool,
	urn resource.URN, operation edit.OperationFunc) result.Result {
	return runTotalStateEdit(ctx, stackName, showPrompt, func(opts display.Options, snap *deploy.Snapshot) error {
		res, err := locateStackResource(opts, snap, urn)
		if err != nil {
			return fmt.Errorf("Unable to perform state edit.\n\n"+
				"You can edit your stack's state using 'pulumi stack export' to export your stack to a file and editing it."+
				" Once this is complete, you can use 'pulumi stack import' to import the edited stack.\n\n"+
				"Error: %w", err)
		}

		return operation(snap, res)
	})
}

// runTotalStateEdit runs a snapshot-mutating function on the entirety of the given stack's snapshot.
// Before mutating, the user may be prompted to for confirmation if the current session is interactive.
func runTotalStateEdit(
	ctx context.Context, stackName string, showPrompt bool,
	operation func(opts display.Options, snap *deploy.Snapshot) error) result.Result {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}
	s, err := requireStack(ctx, stackName, stackOfferNew, opts)
	if err != nil {
		return result.FromError(err)
	}
	return totalStateEdit(ctx, s, showPrompt, opts, operation)
}

func totalStateEdit(ctx context.Context, s backend.Stack, showPrompt bool, opts display.Options,
	operation func(opts display.Options, snap *deploy.Snapshot) error) result.Result {
	snap, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
	if err != nil {
		return result.FromError(err)
	} else if snap == nil {
		return nil
	}

	if showPrompt && cmdutil.Interactive() {
		confirm := false
		surveycore.DisableColor = true
		prompt := opts.Color.Colorize(colors.Yellow + "warning" + colors.Reset + ": ")
		prompt += "This command will edit your stack's state directly. Confirm?"
		if err = survey.AskOne(&survey.Confirm{
			Message: prompt,
		}, &confirm, surveyIcons(opts.Color)); err != nil || !confirm {
			fmt.Println("confirmation declined")
			return result.Bail()
		}
	}

	// The `operation` callback will mutate `snap` in-place. In order to validate the correctness of the transformation
	// that we are doing here, we verify the integrity of the snapshot before the mutation. If the snapshot was valid
	// before we mutated it, we'll assert that we didn't make it invalid by mutating it.
	stackIsAlreadyHosed := snap.VerifyIntegrity() != nil
	if err = operation(opts, snap); err != nil {
		return result.FromError(err)
	}

	// If the stack is already broken, don't bother verifying the integrity here.
	if !stackIsAlreadyHosed {
		contract.AssertNoErrorf(snap.VerifyIntegrity(), "state edit produced an invalid snapshot")
	}

	sdep, err := stack.SerializeDeployment(snap, snap.SecretsManager, false /* showSecrets */)
	if err != nil {
		return result.FromError(fmt.Errorf("serializing deployment: %w", err))
	}

	// Once we've mutated the snapshot, import it back into the backend so that it can be persisted.
	bytes, err := json.Marshal(sdep)
	if err != nil {
		return result.FromError(err)
	}
	dep := apitype.UntypedDeployment{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Deployment: bytes,
	}
	return result.WrapIfNonNil(s.ImportDeployment(ctx, &dep))
}
