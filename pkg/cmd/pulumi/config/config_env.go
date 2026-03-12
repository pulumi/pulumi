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

package config

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend"
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

func newConfigEnvCmd(ws pkgWorkspace.Context, stackRef *string) *cobra.Command {
	impl := configEnvCmd{
		stdin:            os.Stdin,
		stdout:           os.Stdout,
		diags:            cmdutil.Diag(),
		ws:               ws,
		requireStack:     cmdStack.RequireStack,
		loadProjectStack: cmdStack.LoadProjectStack,
		saveProjectStack: cmdStack.SaveProjectStack,
		stackRef:         stackRef,
	}

	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Show config source or manage ESC environment imports",
		Long: "Manages the ESC environment associated with a specific stack. To create a new environment\n" +
			"from a stack's configuration, use `pulumi config env init`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			impl.initArgs()
			return impl.showConfigSource(cmd.Context(), jsonOut)
		},
	}

	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newConfigEnvInitCmd(&impl))
	cmd.AddCommand(newConfigEnvAddCmd(&impl))
	cmd.AddCommand(newConfigEnvRmCmd(&impl))
	cmd.AddCommand(newConfigEnvLsCmd(&impl))
	cmd.AddCommand(newConfigEnvEjectCmd(&impl))

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
	) (backend.Stack, error)

	loadProjectStack func(
		ctx context.Context,
		diags diag.Sink,
		project *workspace.Project,
		stack backend.Stack,
	) (*workspace.ProjectStack, error)

	saveProjectStack func(ctx context.Context, stack backend.Stack, ps *workspace.ProjectStack) error

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
		cmd.diags,
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

	projectStack, err := cmd.loadProjectStack(ctx, cmd.diags, project, stack)
	if err != nil {
		return nil, nil, nil, err
	}

	return projectStack, project, &stack, nil
}

// showConfigSource prints the config source for the stack (T025).
func (cmd *configEnvCmd) showConfigSource(ctx context.Context, jsonOut bool) error {
	opts := display.Options{Color: cmd.color}

	stack, err := cmd.requireStack(
		ctx, cmd.diags, cmd.ws,
		cmdBackend.DefaultLoginManager,
		*cmd.stackRef,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		opts,
	)
	if err != nil {
		return err
	}

	loc := stack.ConfigLocation()
	if loc.IsRemote && loc.EscEnv != nil {
		orgName, orgErr := stackOrgName(stack)
		if orgErr != nil {
			return orgErr
		}
		if jsonOut {
			return ui.FprintJSON(cmd.stdout, map[string]string{
				"sourceType":   "service-backed",
				"environment":  *loc.EscEnv,
				"organization": orgName,
			})
		}
		fmt.Fprintf(cmd.stdout, "Source type:     service-backed\n")
		fmt.Fprintf(cmd.stdout, "ESC environment: %s (org: %s)\n", *loc.EscEnv, orgName)
		fmt.Fprintf(cmd.stdout, "\nRun `pulumi config web` to view in the console, or `pulumi config env eject` to return to local config.\n")
		return nil
	}

	_, configFilePath, pathErr := workspace.DetectProjectStackPath(stack.Ref().Name().Q())
	if pathErr != nil {
		return pathErr
	}
	if jsonOut {
		return ui.FprintJSON(cmd.stdout, map[string]string{
			"sourceType": "local",
			"configFile": configFilePath,
		})
	}
	fmt.Fprintf(cmd.stdout, "Source type:  local\n")
	fmt.Fprintf(cmd.stdout, "Config file:  %s\n", configFilePath)
	return nil
}

func (cmd *configEnvCmd) listStackEnvironments(ctx context.Context, jsonOut bool) error {
	projectStack, _, stack, err := cmd.loadEnvPreamble(ctx)
	if err != nil {
		return err
	}

	// For service-backed stacks, the environment is managed via ESC — direct to better command.
	if (*stack).ConfigLocation().IsRemote {
		loc := (*stack).ConfigLocation()
		if jsonOut {
			return ui.FprintJSON(cmd.stdout, map[string]string{"environment": *loc.EscEnv})
		}
		fmt.Fprintf(cmd.stdout,
			"This stack uses a single ESC environment for all config: %s\n"+
				"Run `pulumi config env` to inspect it, or `pulumi config web` to view in the console.\n",
			*loc.EscEnv)
		return nil
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

	// config env add/rm modify local environment imports, which are not applicable to service-backed stacks.
	if (*stack).ConfigLocation().IsRemote {
		return errors.New(
			"config env add/rm is not supported for service-backed stacks\n" +
				"  To manage imports within the ESC environment, use `pulumi config edit`\n" +
				"  To return to local config, use `pulumi config env eject`")
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

		response := ui.PromptUser("Save?", []string{"yes", "no"}, "yes", cmdutil.GetGlobalColorization())
		switch response {
		case "no":
			return errors.New("canceled")
		case "yes":
		}
	}

	if err = cmd.saveProjectStack(ctx, *stack, projectStack); err != nil {
		return fmt.Errorf("saving stack config: %w", err)
	}
	return nil
}
