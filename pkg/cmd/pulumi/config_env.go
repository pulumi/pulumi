// Copyright 2016-2024, Pulumi Corporation.
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
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newConfigEnvCmd(stackRef *string) *cobra.Command {
	impl := configEnvCmd{
		stdin:            os.Stdin,
		stdout:           os.Stdout,
		ws:               pkgWorkspace.Instance,
		requireStack:     cmdStack.RequireStack,
		loadProjectStack: cmdStack.LoadProjectStack,
		saveProjectStack: cmdStack.SaveProjectStack,
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
	cmd.AddCommand(newConfigEnvLsCmd(&impl))

	return cmd
}

type configEnvCmd struct {
	stdin  io.Reader
	stdout io.Writer

	interactive bool
	color       colors.Colorization

	ssml cmdStack.SecretsManagerLoader
	ws   pkgWorkspace.Context

	requireStack func(
		ctx context.Context,
		ws pkgWorkspace.Context,
		lm cmdBackend.LoginManager,
		stackName string,
		lopt cmdStack.LoadOption,
		opts display.Options,
	) (backend.Stack, error)

	loadProjectStack func(project *workspace.Project, stack backend.Stack) (*workspace.ProjectStack, error)

	saveProjectStack func(stack backend.Stack, ps *workspace.ProjectStack) error

	stackRef *string
}

func (cmd *configEnvCmd) initArgs() {
	cmd.interactive = cmdutil.Interactive()
	cmd.color = cmdutil.GetGlobalColorization()

	cmd.ssml = cmdStack.NewStackSecretsManagerLoaderFromEnv()
}

func (cmd *configEnvCmd) loadEnvPreamble(ctx context.Context,
) (*workspace.ProjectStack, *workspace.Project, *backend.Stack, error) {
	opts := display.Options{Color: cmd.color}

	project, _, err := cmd.ws.ReadProject()
	if err != nil {
		return nil, nil, nil, err
	}

	stack, err := cmd.requireStack(
		ctx,
		cmd.ws,
		cmdBackend.DefaultLoginManager,
		*cmd.stackRef,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		opts,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	_, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, nil, nil, fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}

	projectStack, err := cmd.loadProjectStack(project, stack)
	if err != nil {
		return nil, nil, nil, err
	}

	return projectStack, project, &stack, nil
}

func (cmd *configEnvCmd) listStackEnvironments(ctx context.Context, jsonOut bool) error {
	projectStack, _, _, err := cmd.loadEnvPreamble(ctx)
	if err != nil {
		return err
	}
	imports := projectStack.Environment.Imports()

	if jsonOut {
		if len(imports) == 0 {
			ui.Fprintf(cmd.stdout, "[]\n")
		} else {
			err := ui.FprintJSON(cmd.stdout, imports)
			if err != nil {
				return err
			}
		}
	} else {
		rows := []cmdutil.TableRow{}
		for _, imp := range imports {
			rows = append(rows, cmdutil.TableRow{Columns: []string{imp}})
		}

		if len(imports) > 0 {
			ui.FprintTable(cmd.stdout, cmdutil.Table{
				Headers: []string{"ENVIRONMENTS"},
				Rows:    rows,
			}, nil)
		} else {
			ui.Fprintf(cmd.stdout, "This stack configuration has no environments listed. "+
				"Try adding one with `pulumi config env add <projectName>/<envName>`.\n")
		}
	}

	return nil
}

func (cmd *configEnvCmd) editStackEnvironment(
	ctx context.Context,
	showSecrets bool,
	yes bool,
	edit func(stack *workspace.ProjectStack) error,
) error {
	if !yes && !cmd.interactive {
		return errors.New("--yes must be passed in to proceed when running in non-interactive mode")
	}

	projectStack, project, stack, err := cmd.loadEnvPreamble(ctx)
	if err != nil {
		return err
	}

	if err := edit(projectStack); err != nil {
		return err
	}

	if err := listConfig(
		ctx,
		cmd.ssml,
		cmd.stdout,
		project,
		*stack,
		projectStack,
		showSecrets,
		false, /*jsonOut*/
		false, /*openEnvironment*/
	); err != nil {
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

	if err = cmd.saveProjectStack(*stack, projectStack); err != nil {
		return fmt.Errorf("saving stack config: %w", err)
	}
	return nil
}
