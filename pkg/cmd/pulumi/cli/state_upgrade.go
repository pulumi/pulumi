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
	"fmt"
	"io"
	"os"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/spf13/cobra"
)

func newStateUpgradeCommand() *cobra.Command {
	var sucmd stateUpgradeCmd
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Migrates the current backend to the latest supported version",
		Long: `Migrates the current backend to the latest supported version

This only has an effect on self-managed backends.
`,
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			if err := sucmd.Run(commandContext()); err != nil {
				return result.FromError(err)
			}
			return nil
		}),
	}
	return cmd
}

// stateUpgradeCmd implements the 'pulumi state upgrade' command.
type stateUpgradeCmd struct {
	Stdin  io.Reader // defaults to os.Stdin
	Stdout io.Writer // defaults to os.Stdout
	Stderr io.Writer // defaults to os.Stderr

	// Used to mock out the currentBackend function for testing.
	// Defaults to currentBackend function.
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func (cmd *stateUpgradeCmd) Run(ctx context.Context) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = currentBackend
	}
	currentBackend := cmd.currentBackend // shadow top-level currentBackend

	dopts := display.Options{
		Color:  cmdutil.GetGlobalColorization(),
		Stdin:  cmd.Stdin,
		Stdout: cmd.Stdout,
	}

	b, err := currentBackend(ctx, nil, dopts)
	if err != nil {
		return err
	}

	lb, ok := b.(filestate.Backend)
	if !ok {
		// Only the file state backend supports upgrades,
		// but we don't want to error out here.
		// Report the no-op.
		fmt.Fprintln(cmd.Stdout, "Nothing to do")
		return nil
	}

	prompt := "This will upgrade the current backend to the latest supported version.\n" +
		"Older versions of Pulumi will not be able to read the new format.\n" +
		"Are you sure you want to proceed?"
	if !confirmPrompt(prompt, "yes", dopts) {
		fmt.Fprintln(cmd.Stdout, "Upgrade cancelled")
		return nil
	}

	var opts filestate.UpgradeOptions
	// If we're in interactive mode, prompt for the project name
	// for each stack that doesn't have one.
	if cmdutil.Interactive() {
		opts.ProjectsForDetachedStacks = cmd.projectsForDetachedStacks
	}
	return lb.Upgrade(ctx, &opts)
}

func (cmd *stateUpgradeCmd) projectsForDetachedStacks(stacks []tokens.StackName) ([]tokens.Name, error) {
	projects := make([]tokens.Name, len(stacks))
	err := (&stateUpgradeProjectNameWidget{
		Stdin:  cmd.Stdin,
		Stdout: cmd.Stdout,
		Stderr: cmd.Stderr,
	}).Prompt(stacks, projects)
	return projects, err
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
