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

	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage ESC environments for a stack",
		Long: "Manages the ESC environment associated with a specific stack. To create a new environment\n" +
			"from a stack's configuration, use `pulumi config env init`.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			impl.stdout = cmd.OutOrStdout()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			impl.initArgs()
			return impl.runStatus(cmd, jsonOut)
		},
	}

	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

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

	stackRef   *string
	configFile *string
}

func (cmd *configEnvCmd) initArgs() {
	cmd.interactive = cmdutil.Interactive()
	cmd.color = cmdutil.GetGlobalColorization()

	cmd.ssml = cmdStack.NewStackSecretsManagerLoaderFromEnv()
}

func (cmd *configEnvCmd) resolveStack(ctx context.Context) (*workspace.Project, backend.Stack, error) {
	project, _, err := cmd.ws.ReadProject()
	if err != nil {
		return nil, nil, err
	}
	stack, err := cmd.requireStack(
		ctx,
		cmd.diags,
		cmd.ws,
		cmdBackend.DefaultLoginManager,
		*cmd.stackRef,
		cmdStack.LoadOnly,
		display.Options{Color: cmd.color},
		*cmd.configFile,
	)
	return project, stack, err
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
		*cmd.configFile,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	_, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return nil, nil, nil, fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}

	projectStack, err := cmd.loadProjectStack(ctx, cmd.diags, project, stack, *cmd.configFile)
	if err != nil {
		return nil, nil, nil, err
	}

	return projectStack, project, &stack, nil
}

func (cmd *configEnvCmd) listStackEnvironments(ctx context.Context, jsonOut bool) error {
	projectStack, _, stack, err := cmd.loadEnvPreamble(ctx)
	if err != nil {
		return err
	}

	var imports []string
	if (*stack).ConfigLocation().IsRemote {
		editor, err := newESCConfigEditor(ctx, *stack)
		if err != nil {
			return err
		}
		imports = editor.Imports()
	} else {
		imports = projectStack.Environment.Imports()
	}

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

// importOp is one import mutation: exactly one of addEnvs or removeEnv is set.
type importOp struct {
	addEnvs   []string
	removeEnv string
}

func (op importOp) applyLocal(stack *workspace.ProjectStack) {
	if len(op.addEnvs) > 0 {
		stack.Environment = stack.Environment.Append(op.addEnvs...)
		return
	}
	stack.Environment = stack.Environment.Remove(op.removeEnv)
}

func (op importOp) applyRemote(editor *escConfigEditor) error {
	if len(op.addEnvs) > 0 {
		return editor.AddImports(op.addEnvs...)
	}
	return editor.RemoveImport(op.removeEnv)
}

func (cmd *configEnvCmd) editStackEnvironment(
	ctx context.Context,
	showSecrets bool,
	yes bool,
	op importOp,
) error {
	if !yes && !cmd.interactive {
		return backenderr.ErrNonInteractiveRequiresYes
	}

	projectStack, project, stack, err := cmd.loadEnvPreamble(ctx)
	if err != nil {
		return err
	}

	if configStoreIsRemote(*stack, *cmd.configFile) {
		return cmd.editRemoteStackEnvironment(ctx, yes, *stack, op)
	}

	op.applyLocal(projectStack)

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

		response := ui.PromptUser("Save?", []string{"yes", "no"}, "yes", cmdutil.GetGlobalColorization())
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

// editRemoteStackEnvironment edits the backing ESC environment's imports, skipping the full
// merged-config display the local path shows since that would require re-resolving the environment.
func (cmd *configEnvCmd) editRemoteStackEnvironment(
	ctx context.Context,
	yes bool,
	stack backend.Stack,
	op importOp,
) error {
	if err := rejectIfPinned(stack, *cmd.configFile); err != nil {
		return err
	}

	editor, err := newESCConfigEditor(ctx, stack)
	if err != nil {
		return err
	}

	if err := op.applyRemote(editor); err != nil {
		return err
	}

	imports := editor.Imports()
	if len(imports) == 0 {
		ui.Fprintf(cmd.stdout, "This stack configuration has no environments listed.\n")
	} else {
		printImportsTable(cmd.stdout, imports)
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

	if err := editor.Save(ctx); err != nil {
		return fmt.Errorf("saving stack config: %w", err)
	}
	return nil
}

func printImportsTable(w io.Writer, imports []string) {
	rows := make([]cmdutil.TableRow, 0, len(imports))
	for _, imp := range imports {
		rows = append(rows, cmdutil.TableRow{Columns: []string{imp}})
	}
	ui.FprintTable(w, cmdutil.Table{
		Headers: []string{"ENVIRONMENTS"},
		Rows:    rows,
	}, nil)
}

// runStatus implements bare `pulumi config env`, reporting where the stack's config lives (linked
// ESC environment or local file).
func (cmd *configEnvCmd) runStatus(cobraCmd *cobra.Command, jsonOut bool) error {
	ctx := cobraCmd.Context()

	project, stack, err := cmd.resolveStack(ctx)
	if err != nil {
		// Bare `config env` historically printed help offline; preserve that when no stack resolves.
		return cobraCmd.Help()
	}

	if stack.ConfigLocation().IsRemote {
		return cmd.runRemoteStatus(ctx, stack, jsonOut)
	}
	return cmd.runLocalStatus(ctx, project, stack, jsonOut)
}

func (cmd *configEnvCmd) runRemoteStatus(ctx context.Context, stack backend.Stack, jsonOut bool) error {
	loc := stack.ConfigLocation()
	envRef := ""
	if loc.EscEnv != nil {
		envRef = *loc.EscEnv
	}

	editor, err := newESCConfigEditor(ctx, stack)
	if err != nil {
		return err
	}
	imports := editor.Imports()

	if jsonOut {
		return ui.FprintJSON(cmd.stdout, struct {
			Source      string   `json:"source"`
			Environment string   `json:"environment"`
			Imports     []string `json:"imports"`
		}{Source: "remote", Environment: envRef, Imports: imports})
	}

	ui.Fprintf(cmd.stdout, "This stack's configuration is stored remotely in environment %q.\n", envRef)
	if len(imports) == 0 {
		ui.Fprintf(cmd.stdout, "It imports no environments.\n")
	} else {
		printImportsTable(cmd.stdout, imports)
	}
	return nil
}

func (cmd *configEnvCmd) runLocalStatus(
	ctx context.Context, project *workspace.Project, stack backend.Stack, jsonOut bool,
) error {
	configPath := *cmd.configFile
	if configPath == "" {
		_, path, err := workspace.DetectProjectStackPath(stack.Ref().Name().Q())
		if err != nil {
			return fmt.Errorf("getting configuration file: %w", err)
		}
		configPath = path
	}

	projectStack, err := cmd.loadProjectStack(ctx, cmd.diags, project, stack, *cmd.configFile)
	if err != nil {
		return err
	}
	imports := projectStack.Environment.Imports()

	if jsonOut {
		return ui.FprintJSON(cmd.stdout, struct {
			Source     string   `json:"source"`
			ConfigFile string   `json:"configFile"`
			Imports    []string `json:"imports"`
		}{Source: "local", ConfigFile: configPath, Imports: imports})
	}

	ui.Fprintf(cmd.stdout, "This stack's configuration is stored locally in %q.\n", configPath)
	if len(imports) == 0 {
		ui.Fprintf(cmd.stdout, "It imports no environments.\n")
	} else {
		printImportsTable(cmd.stdout, imports)
	}
	return nil
}
