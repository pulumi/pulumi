// Copyright 2016, Pulumi Corporation.
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

package config

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

// errBackendNoEnvironments indicates that the given backend does not support ESC environments and
// points the user at the Pulumi Cloud backend, which does.
func errBackendNoEnvironments(b backend.Backend) error {
	return fmt.Errorf("backend %v does not support environments; Pulumi ESC environments require the "+
		"Pulumi Cloud backend, use `pulumi login` without arguments to log into the Pulumi Cloud backend", b.Name())
}

func newConfigEnvCmd(ws pkgWorkspace.Context, stackRef *string, configFile *string) *cobra.Command {
	impl := configEnvCmd{
		stdin:            os.Stdin,
		diags:            cmdutil.Diag(),
		ws:               ws,
		requireStack:     cmdStack.RequireStack,
		loadProjectStack: cmdStack.LoadProjectStack,
		saveProjectStack: cmdStack.SaveProjectStack,
		stackRef:         stackRef,
		configFile:       configFile,
	}

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage ESC environments for a stack",
		Long: "Manages the ESC environment associated with a specific stack. To create a new environment\n" +
			"from a stack's configuration, use `pulumi config env init`.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			impl.stdout = cmd.OutOrStdout()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newConfigEnvInitCmd(&impl))
	cmd.AddCommand(newConfigEnvAddCmd(&impl))
	cmd.AddCommand(newConfigEnvRemoveCmd(&impl))
	cmd.AddCommand(newConfigEnvListCmd(&impl))

	return cmd
}

type configEnvCmd struct {
	stdin  io.Reader
	stdout io.Writer

	interactive bool
	color       colors.Colorization
	diags       diag.Sink

	ssml cmdStack.SecretsManagerLoader
	ws   pkgWorkspace.Context

	requireStack func(
		ctx context.Context,
		sink diag.Sink,
		ws pkgWorkspace.Context,
		lm cmdBackend.LoginManager,
		stackName string,
		lopt cmdStack.LoadOption,
		opts display.Options,
		configFile string,
	) (backend.Stack, error)

	loadProjectStack func(
		ctx context.Context,
		diags diag.Sink,
		project *workspace.Project,
		stack backend.Stack,
		configFile string,
	) (*workspace.ProjectStack, error)

	saveProjectStack func(ctx context.Context, stack backend.Stack, ps *workspace.ProjectStack, configFile string) error

	// prompt asks the user to pick one of options.
	prompt func(msg string, options []string, defaultOption string, colorization colors.Colorization,
		surveyAskOpts ...survey.AskOpt) string

	stackRef   *string
	configFile *string
}

func (cmd *configEnvCmd) initArgs() {
	cmd.interactive = cmdutil.Interactive()
	cmd.color = cmdutil.GetGlobalColorization()
	cmd.prompt = ui.PromptUser
	cmd.ssml = cmdStack.NewStackSecretsManagerLoaderFromEnv()
}

func (cmd *configEnvCmd) loadEnvPreamble(ctx context.Context,
) (*workspace.ProjectStack, *workspace.Project, *backend.Stack, error) {
	opts := display.Options{Color: cmd.color}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("getting current working directory: %w", err)
	}

	project, _, err := cmd.ws.ReadProject(cwd)
	if err != nil {
		return nil, nil, nil, err
	}

	stack, err := cmd.requireStack(
		ctx,
		cmd.diags,
		cmd.ws,
		cmdBackend.DefaultLoginManager,
		*cmd.stackRef,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		opts,
		*cmd.configFile,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	_, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, nil, nil, errBackendNoEnvironments(stack.Backend())
	}

	projectStack, err := cmd.loadProjectStack(ctx, cmd.diags, project, stack, *cmd.configFile)
	if err != nil {
		return nil, nil, nil, err
	}

	return projectStack, project, &stack, nil
}

func (cmd *configEnvCmd) listStackEnvironments(ctx context.Context, render stackEnvironmentsRenderFunc) error {
	projectStack, _, _, err := cmd.loadEnvPreamble(ctx)
	if err != nil {
		return err
	}
	imports := projectStack.Environment.Imports()

	return render(cmd.stdout, imports)
}

func (cmd *configEnvCmd) editStackEnvironment(
	ctx context.Context,
	showSecrets bool,
	yes bool,
	edit func(stack *workspace.ProjectStack) error,
) error {
	if !yes && !cmd.interactive {
		return backenderr.ErrNonInteractiveRequiresYes
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
		*cmd.configFile,
	); err != nil {
		return err
	}

	if !yes {
		fmt.Fprintln(cmd.stdout)

		response := cmd.prompt("Save?", []string{"yes", "no"}, "yes", cmdutil.GetGlobalColorization())
		switch response {
		case "no":
			return errors.New("canceled")
		case "yes":
		}
	}

	if err = cmd.saveProjectStack(ctx, *stack, projectStack, *cmd.configFile); err != nil {
		return fmt.Errorf("saving stack config: %w", err)
	}
	return nil
}
