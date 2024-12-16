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

package config

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type configCopyCmd struct {
	// Parsed arguments to the command.
	Args *configCopyArgs

	// The command's standard output.
	Stdout io.Writer

	// The workspace to operate on.
	Workspace pkgWorkspace.Context
	// The login manager to use for authenticating with and loading backends.
	LoginManager cmdBackend.LoginManager
	// The project stack manager to use for loading and saving project stack configuration.
	ProjectStackManager cmdStack.ProjectStackManager
}

// A set of arguments for the `config cp` command.
type configCopyArgs struct {
	Colorizer        colors.Colorization
	DestinationStack string
	Path             bool
	SourceStack      string
}

func newConfigCopyCmd(configFile *string, stack *string) *cobra.Command {
	configCopy := &configCopyCmd{
		Args: &configCopyArgs{
			Colorizer: cmdutil.GetGlobalColorization(),
		},
		Stdout:       os.Stdout,
		Workspace:    pkgWorkspace.Instance,
		LoginManager: cmdBackend.DefaultLoginManager,
	}

	cpCommand := &cobra.Command{
		Use:   "cp [key]",
		Short: "Copy config to another stack",
		Long: "Copies the config from the current stack to the destination stack. If `key` is omitted,\n" +
			"then all of the config from the current stack will be copied to the destination stack.",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmd.RunCmdFunc(func(command *cobra.Command, args []string) error {
			configCopy.Args.SourceStack = *stack
			configCopy.ProjectStackManager = cmdStack.NewProjectStackManager(*configFile)

			ctx := command.Context()
			err := configCopy.run(ctx, args)

			return err
		}),
	}

	cpCommand.PersistentFlags().BoolVar(
		&configCopy.Args.Path, "path", false,
		"The key contains a path to a property in a map or list to set")
	cpCommand.PersistentFlags().StringVarP(
		&configCopy.Args.DestinationStack, "dest", "d", "",
		"The name of the new stack to copy the config to")

	return cpCommand
}

func (cmd *configCopyCmd) run(ctx context.Context, args []string) error {
	opts := display.Options{
		Color: cmd.Args.Colorizer,
	}

	project, _, err := cmd.Workspace.ReadProject()
	if err != nil {
		return err
	}

	// Get current stack and ensure that it is a different stack to the destination stack
	currentStack, err := cmdStack.RequireStack(
		ctx,
		cmd.Workspace,
		cmd.LoginManager,
		cmd.Args.SourceStack,
		cmdStack.SetCurrent,
		opts,
	)
	if err != nil {
		return err
	}
	if currentStack.Ref().Name().String() == cmd.Args.DestinationStack {
		return errors.New("current stack and destination stack are the same")
	}
	currentProjectStack, err := cmd.ProjectStackManager.Load(project, currentStack)
	if err != nil {
		return err
	}

	// Get the destination stack
	destinationStack, err := cmdStack.RequireStack(
		ctx,
		cmd.Workspace,
		cmd.LoginManager,
		cmd.Args.DestinationStack,
		cmdStack.LoadOnly,
		opts,
	)
	if err != nil {
		return err
	}
	destinationProjectStack, err := cmd.ProjectStackManager.Load(project, destinationStack)
	if err != nil {
		return err
	}

	ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()

	// Do we need to copy a single value or the entire map
	if len(args) > 0 {
		// A single key was specified so we only need to copy that specific value
		return copySingleConfigKey(
			ctx,
			ssml,
			args[0],
			cmd.Args.Path,
			currentStack,
			currentProjectStack,
			destinationStack,
			destinationProjectStack,
		)
	}

	requiresSaving, err := cmdStack.CopyEntireConfigMap(
		ctx,
		ssml,
		currentStack,
		currentProjectStack,
		destinationStack,
		destinationProjectStack,
	)
	if err != nil {
		return err
	}

	// The use of `requiresSaving` here ensures that there was actually some config
	// that needed saved, otherwise it's an unnecessary save call
	if requiresSaving {
		err := cmd.ProjectStackManager.Save(destinationStack, destinationProjectStack)
		if err != nil {
			return err
		}
	}

	return nil
}
