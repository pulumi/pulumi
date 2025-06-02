// Copyright 2024, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/edit"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// runStateEdit runs the given state edit function on a resource with the given URN in a given stack.
func runStateEdit(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, lm cmdBackend.LoginManager, stackName string,
	showPrompt bool, urn resource.URN, operation edit.OperationFunc,
) error {
	return runTotalStateEdit(ctx, sink, ws, lm, stackName, showPrompt,
		func(opts display.Options, snap *deploy.Snapshot) error {
			res, err := locateStackResource(opts, snap, urn)
			if err != nil {
				return err
			}

			return operation(snap, res)
		})
}

// runTotalStateEdit runs a snapshot-mutating function on the entirety of the given stack's snapshot.
// Before mutating, the user may be prompted to for confirmation if the current session is interactive.
func runTotalStateEdit(
	ctx context.Context,
	sink diag.Sink,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	stackName string,
	showPrompt bool,
	operation func(opts display.Options, snap *deploy.Snapshot) error,
) error {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}
	s, err := cmdStack.RequireStack(
		ctx,
		sink,
		ws,
		lm,
		stackName,
		cmdStack.OfferNew,
		opts,
	)
	if err != nil {
		return err
	}
	return TotalStateEdit(ctx, s, showPrompt, opts, operation, nil)
}

// runTotalStateEditWithPrompt is the same as runTotalStateEdit, but allows the caller to
// override the prompt message.
func runTotalStateEditWithPrompt(
	ctx context.Context,
	sink diag.Sink,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	stackName string,
	showPrompt bool,
	operation func(opts display.Options, snap *deploy.Snapshot) error,
	overridePromptMessage string,
) error {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}
	s, err := cmdStack.RequireStack(
		ctx,
		sink,
		ws,
		lm,
		stackName,
		cmdStack.OfferNew,
		opts,
	)
	if err != nil {
		return err
	}
	return TotalStateEdit(ctx, s, showPrompt, opts, operation, &overridePromptMessage)
}

func TotalStateEdit(
	ctx context.Context,
	s backend.Stack,
	showPrompt bool,
	opts display.Options,
	operation func(opts display.Options, snap *deploy.Snapshot) error,
	overridePromptMessage *string,
) error {
	snap, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
	if err != nil {
		return err
	} else if snap == nil {
		return nil
	}

	if showPrompt && cmdutil.Interactive() {
		confirm := false
		surveycore.DisableColor = true
		prompt := opts.Color.Colorize(colors.Yellow + "warning" + colors.Reset + ": ")
		if overridePromptMessage != nil {
			prompt += *overridePromptMessage
		} else {
			prompt += "This command will edit your stack's state directly. Confirm?"
		}
		if err = survey.AskOne(&survey.Confirm{
			Message: prompt,
		}, &confirm, ui.SurveyIcons(opts.Color)); err != nil || !confirm {
			return result.FprintBailf(os.Stdout, "confirmation declined")
		}
	}

	// The `operation` callback will mutate `snap` in-place. In order to validate the correctness of the transformation
	// that we are doing here, we verify the integrity of the snapshot before the mutation. If the snapshot was valid
	// before we mutated it, we'll assert that we didn't make it invalid by mutating it.
	stackIsAlreadyHosed := snap.VerifyIntegrity() != nil
	if err = operation(opts, snap); err != nil {
		return err
	}

	// If the stack is already broken, don't bother verifying the integrity here.
	if !stackIsAlreadyHosed && !backend.DisableIntegrityChecking {
		contract.AssertNoErrorf(snap.VerifyIntegrity(), "state edit produced an invalid snapshot")
	}

	sdep, err := stack.SerializeDeployment(ctx, snap, false /* showSecrets */)
	if err != nil {
		return fmt.Errorf("serializing deployment: %w", err)
	}

	// Once we've mutated the snapshot, import it back into the backend so that it can be persisted.
	bytes, err := json.Marshal(sdep)
	if err != nil {
		return err
	}
	dep := apitype.UntypedDeployment{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Deployment: bytes,
	}
	return s.ImportDeployment(ctx, &dep)
}

// locateStackResource attempts to find a unique resource associated with the given URN in the given snapshot. If the
// given URN is ambiguous and this is an interactive terminal, it prompts the user to select one of the resources in
// the list of resources with identical URNs to operate upon.
func locateStackResource(opts display.Options, snap *deploy.Snapshot, urn resource.URN) (*resource.State, error) {
	candidateResources := edit.LocateResource(snap, urn)
	switch {
	case len(candidateResources) == 0: // resource was not found
		return nil, fmt.Errorf("No such resource %q exists in the current state", urn)
	case len(candidateResources) == 1: // resource was unambiguously found
		return candidateResources[0], nil
	}

	// If there exist multiple resources that have the requested URN, prompt the user to select one if we're running
	// interactively. If we're not, early exit.
	if !cmdutil.Interactive() {
		errorMsg := "Resource URN ambiguously referred to multiple resources. Did you mean:\n"
		for _, res := range candidateResources {
			errorMsg += fmt.Sprintf("  %s\n", res.ID)
		}
		return nil, errors.New(errorMsg)
	}

	// Note: this is done to adhere to the same color scheme as the `pulumi new` picker, which also does this.
	surveycore.DisableColor = true
	prompt := "Multiple resources with the given URN exist, please select the one to edit:"
	prompt = opts.Color.Colorize(colors.SpecPrompt + prompt + colors.Reset)

	options := slice.Prealloc[string](len(candidateResources))
	optionMap := make(map[string]*resource.State)
	for _, ambiguousResource := range candidateResources {
		// Prompt the user to select from a list of IDs, since these resources are known to all have the same URN.
		message := fmt.Sprintf("%q", ambiguousResource.ID)
		if ambiguousResource.Protect {
			message += " (Protected)"
		}

		if ambiguousResource.Delete {
			message += " (Pending Deletion)"
		}

		options = append(options, message)
		optionMap[message] = ambiguousResource
	}

	var option string
	if err := survey.AskOne(&survey.Select{
		Message:  prompt,
		Options:  options,
		PageSize: cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: len(options)}),
	}, &option, ui.SurveyIcons(opts.Color)); err != nil {
		return nil, errors.New("no resource selected")
	}

	return optionMap[option], nil
}

// Prompt the user to select a URN from the passed in state.
//
// stackName is the name of the current stack.
//
// snap is the snapshot of the current stack.  If (*snap) is not nil, it will be set to
// the retrieved snapshot value. This allows caching between calls.
//
// Prompt is displayed to the user when selecting the URN.
func getURNFromState(
	ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, lm cmdBackend.LoginManager,
	stackName string, snap **deploy.Snapshot, prompt string,
) (resource.URN, error) {
	if snap == nil {
		// This means we won't cache the value.
		snap = new(*deploy.Snapshot)
	}
	if *snap == nil {
		opts := display.Options{
			Color: cmdutil.GetGlobalColorization(),
		}

		s, err := cmdStack.RequireStack(
			ctx,
			sink,
			ws,
			lm,
			stackName,
			cmdStack.LoadOnly,
			opts,
		)
		if err != nil {
			return "", err
		}
		*snap, err = s.Snapshot(ctx, stack.DefaultSecretsProvider)
		if err != nil {
			return "", err
		}
		if *snap == nil {
			return "", errors.New("no snapshot found")
		}
	}
	urnList := make([]string, len((*snap).Resources))
	for i, r := range (*snap).Resources {
		urnList[i] = string(r.URN)
	}
	var urn string
	err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: urnList,
	}, &urn, survey.WithValidator(survey.Required), ui.SurveyIcons(cmdutil.GetGlobalColorization()))
	if err != nil {
		return "", err
	}
	result := resource.URN(urn)
	contract.Assertf(result.IsValid(),
		"Because we chose from an existing URN, it must be valid")
	return result, nil
}

// Ask the user for a resource name.
func getNewResourceName() (string, error) {
	var resourceName string
	err := survey.AskOne(&survey.Input{
		Message: "Choose a new resource name:",
	}, &resourceName, ui.SurveyIcons(cmdutil.GetGlobalColorization()))
	if err != nil {
		return "", err
	}
	return resourceName, nil
}

// Format a non-nil error that indicates some arguments are missing for a
// non-interactive session.
func missingNonInteractiveArg(args ...string) error {
	switch len(args) {
	case 0:
		panic("cannot create an error message for missing zero args")
	case 1:
		return fmt.Errorf("Must supply <%s> unless pulumi is run interactively", args[0])
	default:
		for i, s := range args {
			args[i] = "<" + s + ">"
		}
		return fmt.Errorf("Must supply %s and %s unless pulumi is run interactively",
			strings.Join(args[:len(args)-1], ", "), args[len(args)-1])
	}
}
