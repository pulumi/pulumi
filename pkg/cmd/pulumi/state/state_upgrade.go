// Copyright 2016-2025, Pulumi Corporation.
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
	"fmt"
	"io"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/spf13/cobra"
)

func newStateUpgradeCommand(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Migrates the current backend to the latest supported version",
		Long: `Migrates the current backend to the latest supported version

This only has an effect on DIY backends.
`,
		Args: cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			stdout := cmd.OutOrStdout()
			stdin := cmd.InOrStdin()
			stderr := cmd.ErrOrStderr()

			dopts := display.Options{
				Color:  cmdutil.GetGlobalColorization(),
				Stdin:  stdin,
				Stdout: stdout,
			}

			b, err := cmdBackend.CurrentBackend(
				ctx,
				ws,
				lm,
				nil,
				dopts,
			)
			if err != nil {
				return err
			}

			lb, ok := b.(diy.Backend)
			if !ok {
				// Only the diy backend supports upgrades,
				// but we don't want to error out here.
				// Report the no-op.
				fmt.Fprintln(stdout, "Nothing to do")
				return nil
			}

			prompt := "This will upgrade the current backend to the latest supported version.\n" +
				"Older versions of Pulumi will not be able to read the new format.\n" +
				"Are you sure you want to proceed?"
			if !yes && !ui.ConfirmPrompt(prompt, "yes", dopts) {
				fmt.Fprintln(stdout, "Upgrade cancelled")
				return nil
			}

			var opts diy.UpgradeOptions
			// If we're in interactive mode, prompt for the project name
			// for each stack that doesn't have one.
			if cmdutil.Interactive() {
				opts.ProjectsForDetachedStacks = func(stacks []tokens.StackName) ([]tokens.Name, error) {
					projects := make([]tokens.Name, len(stacks))
					err := (&stateUpgradeProjectNameWidget{
						Stdin:  stdin,
						Stdout: stdout,
						Stderr: stderr,
					}).Prompt(stacks, projects)
					return projects, err
				}
			}
			return lb.Upgrade(ctx, &opts)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Automatically approve and perform the upgrade")
	return cmd
}

// stateUpgradeProjectNameWidget is a widget that prompts the user
// for a project name for every stack that doesn't have one.
//
// It is used by the 'pulumi state upgrade' command
// when it encounters stacks without a project name.
type stateUpgradeProjectNameWidget struct {
	Stdin  io.Reader // required
	Stdout io.Writer // required
	Stderr io.Writer // required
}

// Prompt prompts the user for a project name for each stack
// and stores the result in the corresponding index of projects.
//
// The length of projects must be equal to the length of stacks.
func (w *stateUpgradeProjectNameWidget) Prompt(stacks []tokens.StackName, projects []tokens.Name) error {
	contract.Assertf(len(stacks) == len(projects),
		"length of stacks (%d) must equal length of projects (%d)", len(stacks), len(projects))

	if len(stacks) == 0 {
		// Nothing to prompt for.
		return nil
	}

	stdin, ok1 := w.Stdin.(terminal.FileReader)
	stdout, ok2 := w.Stdout.(terminal.FileWriter)
	if !ok1 || !ok2 {
		// We're not using a real terminal, so we can't prompt.
		// Pretend we're in non-interactive mode.
		return nil
	}

	fmt.Fprintln(stdout, "Found stacks without a project name.")
	fmt.Fprintln(stdout, "Please enter a project name for each stack, or enter to skip that stack.")
	for i, stack := range stacks {
		var project string
		err := survey.AskOne(
			&survey.Input{
				Message: fmt.Sprintf("Stack %s", stack),
				Help:    "Enter a name for the project, or press enter to skip",
			},
			&project,
			survey.WithStdio(stdin, stdout, w.Stderr),
			survey.WithValidator(w.validateProject),
		)
		if err != nil {
			return fmt.Errorf("prompt for %q: %w", stack, err)
		}

		projects[i] = tokens.Name(project)
	}

	return nil
}

func (w *stateUpgradeProjectNameWidget) validateProject(ans any) error {
	proj, ok := ans.(string)
	contract.Assertf(ok, "widget should have a string output, got %T", ans)

	if proj == "" {
		// The user wants to skip this stack.
		return nil
	}

	return tokens.ValidateProjectName(proj)
}
