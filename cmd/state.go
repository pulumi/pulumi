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
		return nil, nil
	case len(candidateResources) == 1: // resource was unambiguously found
		return candidateResources[0], nil
	}

	// If there exist multiple resources that have the requested URN, prompt the user to select one if we're running
	// interactively. If we're not, early exit.
	if !cmdutil.Interactive() {
		return nil, errors.Errorf("Resource URN %q ambiguously refers to multiple resources", urn)
	}

	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = opts.Color.Colorize(colors.BrightGreen + ">" + colors.Reset)
	prompt := "Multiple resources with the given URN exist, please select the one to edit:"
	prompt = opts.Color.Colorize(colors.SpecPrompt + prompt + colors.Reset)

	var options []string
	optionMap := make(map[string]*resource.State)
	for _, ambiguousResource := range candidateResources {
		// Prompt the user to select from a list of IDs, since these resources are known to all have the same URN.
		message := fmt.Sprintf("Resource %q", ambiguousResource.ID)
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

// runStateEdit runs the given state edit function on a resource with the given URN in the current stack.
func runStateEdit(urn resource.URN, operation func(snap *deploy.Snapshot, res *resource.State) error) error {
	return runTotalStateEdit(func(opts display.Options, snap *deploy.Snapshot) error {
		res, err := locateStackResource(opts, snap, urn)
		if err != nil {
			return err
		}

		if res == nil {
			return errors.Errorf("No such resource %q exists in the current state", urn)
		}

		return operation(snap, res)
	})
}

// runTotalStateEdit runs a snapshot-mutating function on the entirity of the current stack's snapshot. Before mutating
// the snapshot, the user is prompted for confirmation if the current session is interactive.
func runTotalStateEdit(operation func(opts display.Options, snap *deploy.Snapshot) error) error {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}
	s, err := requireCurrentStack(true, opts, true /*setCurrent*/)
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
		if err = survey.AskOne(&survey.Confirm{
			Message: "This command will edit your stack's state directly. Confirm?",
		}, &confirm, nil); err != nil || !confirm {
			return errors.New("confirmation declined")
		}
	}

	stackIsAlreadyHosed := snap.VerifyIntegrity() != nil
	if err = operation(opts, snap); err != nil {
		return err
	}

	// If the stack is already broken, don't bother verifying the integrity here.
	if !stackIsAlreadyHosed {
		contract.AssertNoErrorf(snap.VerifyIntegrity(), "state edit produced an invalid snapshot")
	}
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
