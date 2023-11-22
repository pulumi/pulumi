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
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/erikgeiser/promptkit/confirmation"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newConfigEnvCmd(stackRef *string) *cobra.Command {
	impl := configEnvCmd{
		ctx:              commandContext(),
		stdin:            os.Stdin,
		stdout:           os.Stdout,
		interactive:      cmdutil.Interactive(),
		color:            cmdutil.GetGlobalColorization(),
		readProject:      readProject,
		requireStack:     requireStack,
		loadProjectStack: loadProjectStack,
		saveProjectStack: saveProjectStack,
		stackRef:         stackRef,
	}

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage ESC environments for a stack",
		Long: "Manages the ESC environment associated with a specific stack. To create a new environment\n" +
			"from a stack's configuration, use `pulumi config env init`.",
		Args: cmdutil.NoArgs,
	}

	cmd.AddCommand(newConfigEnvInitCmd(&impl))
	cmd.AddCommand(newConfigEnvAddCmd(&impl))
	cmd.AddCommand(newConfigEnvRmCmd(&impl))

	return cmd
}

type configEnvCmd struct {
	ctx context.Context

	stdin  io.Reader
	stdout io.Writer

	interactive bool
	color       colors.Colorization

	readProject func() (*workspace.Project, string, error)

	requireStack func(
		ctx context.Context,
		stackName string,
		lopt stackLoadOption,
		opts display.Options,
	) (backend.Stack, error)

	loadProjectStack func(project *workspace.Project, stack backend.Stack) (*workspace.ProjectStack, error)

	saveProjectStack func(stack backend.Stack, ps *workspace.ProjectStack) error

	stackRef *string
}

func (cmd *configEnvCmd) editStackEnvironment(
	showSecrets bool,
	yes bool,
	edit func(stack *workspace.ProjectStack) error,
) error {
	if !yes && !cmd.interactive {
		return errors.New("--yes must be passed in to proceed when running in non-interactive mode")
	}

	opts := display.Options{Color: cmd.color}

	project, _, err := cmd.readProject()
	if err != nil {
		return err
	}

	stack, err := cmd.requireStack(cmd.ctx, *cmd.stackRef, stackOfferNew|stackSetCurrent, opts)
	if err != nil {
		return err
	}

	_, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}

	projectStack, err := cmd.loadProjectStack(project, stack)
	if err != nil {
		return err
	}

	if err := edit(projectStack); err != nil {
		return err
	}

	if err := listConfig(cmd.ctx, cmd.stdout, project, stack, projectStack, showSecrets, false); err != nil {
		return err
	}

	if !yes {
		fmt.Fprintln(cmd.stdout)

		confirm := confirmation.New("Save?", confirmation.Yes)
		confirm.Input, confirm.Output = cmd.stdin, cmd.stdout

		save, err := confirm.RunPrompt()
		if err != nil {
			return err
		}
		if !save {
			return errors.New("canceled")
		}
	}

	if err = cmd.saveProjectStack(stack, projectStack); err != nil {
		return fmt.Errorf("saving stack config: %w", err)
	}
	return nil
}
