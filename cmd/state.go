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
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/edit"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/spf13/cobra"
	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"
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
	return cmd
}

// locateStackResource attempts to find a unique resource associated with the given URN in the given snapshot. If the
// given URN is ambiguous and this is an interactive terminal, it prompts the user to select one of the resources in
// the list of resources with identical URNs to operate upon.
func locateStackResource(opts display.Options, snap *deploy.Snapshot, urn resource.URN) (*resource.State, error) {
	candidateResources := edit.LocateResource(snap, urn)
	switch {
	case len(candidateResources) == 0: // resource was not found
		return nil, errors.Errorf("No such resource %q exists in the current state", urn)
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
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = opts.Color.Colorize(colors.BrightGreen + ">" + colors.Reset)
	prompt := "Multiple resources with the given URN exist, please select the one to edit:"
	prompt = opts.Color.Colorize(colors.SpecPrompt + prompt + colors.Reset)

	var options []string
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
		PageSize: len(options),
	}, &option, nil); err != nil {
		return nil, errors.New("no resource selected")
	}

	return optionMap[option], nil
}

// runStateEdit runs the given state edit function on a resource with the given URN in a given stack.
func runStateEdit(stackName string, urn resource.URN, operation edit.OperationFunc) error {
	return runTotalStateEdit(stackName, func(opts display.Options, snap *deploy.Snapshot) error {
		res, err := locateStackResource(opts, snap, urn)
		if err != nil {
			return err
		}

		return operation(snap, res)
	})
}

// runTotalStateEdit runs a snapshot-mutating function on the entirity of the given stack's snapshot. Before mutating
// the snapshot, the user is prompted for confirmation if the current session is interactive.
func runTotalStateEdit(stackName string, operation func(opts display.Options, snap *deploy.Snapshot) error) error {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}
	s, err := requireStack(stackName, true, opts, true /*setCurrent*/)
	if err != nil {
		return err
	}
	snap, err := s.Snapshot(commandContext())
	if err != nil {
		return err
	}

	if cmdutil.Interactive() {
		confirm := false
		surveycore.DisableColor = true
		surveycore.QuestionIcon = ""
		surveycore.SelectFocusIcon = opts.Color.Colorize(colors.BrightGreen + ">" + colors.Reset)
		prompt := opts.Color.Colorize(colors.Yellow + "warning" + colors.Reset + ": ")
		prompt += "This command will edit your stack's state directly. Confirm?"
		if err = survey.AskOne(&survey.Confirm{
			Message: prompt,
		}, &confirm, nil); err != nil || !confirm {
			return errors.New("confirmation declined")
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
	if !stackIsAlreadyHosed {
		contract.AssertNoErrorf(snap.VerifyIntegrity(), "state edit produced an invalid snapshot")
	}

	// Once we've mutated the snapshot, import it back into the backend so that it can be persisted.
	bytes, err := json.Marshal(stack.SerializeDeployment(snap))
	if err != nil {
		return err
	}
	dep := apitype.UntypedDeployment{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Deployment: bytes,
	}
	return s.ImportDeployment(commandContext(), &dep)
}
