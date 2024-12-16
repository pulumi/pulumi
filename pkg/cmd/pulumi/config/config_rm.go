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
	"fmt"
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

type configRmCmd struct {
	// Parsed arguments to the command.
	Args *configRmArgs

	// The command's standard output.
	Stdout io.Writer

	// The workspace to operate on.
	Workspace pkgWorkspace.Context
	// The login manager to use for authenticating with and loading backends.
	LoginManager cmdBackend.LoginManager
	// The project stack manager to use for loading and saving project stack configuration.
	ProjectStackManager cmdStack.ProjectStackManager
}

// A set of arguments for the `config rm` command.
type configRmArgs struct {
	Colorizer colors.Colorization
	Path      bool
	Stack     string
}

func newConfigRmCmd(configFile *string, stack *string) *cobra.Command {
	configRm := &configRmCmd{
		Args: &configRmArgs{
			Colorizer: cmdutil.GetGlobalColorization(),
		},
		Stdout:       os.Stdout,
		Workspace:    pkgWorkspace.Instance,
		LoginManager: cmdBackend.DefaultLoginManager,
	}

	rmCmd := &cobra.Command{
		Use:   "rm <key>",
		Short: "Remove configuration value",
		Long: "Remove configuration value.\n\n" +
			"The `--path` flag can be used to remove a value inside a map or list:\n\n" +
			"  - `pulumi config rm --path outer.inner` will remove the `inner` key, " +
			"if the value of `outer` is a map `inner: value`.\n" +
			"  - `pulumi config rm --path 'names[0]'` will remove the first item, " +
			"if the value of `names` is a list.",
		Args: cmdutil.SpecificArgs([]string{"key"}),
		Run: cmd.RunCmdFunc(func(command *cobra.Command, args []string) error {
			configRm.Args.Stack = *stack
			configRm.ProjectStackManager = cmdStack.NewProjectStackManager(*configFile)

			ctx := command.Context()
			err := configRm.run(ctx, args[0])

			return err
		}),
	}
	rmCmd.PersistentFlags().BoolVar(
		&configRm.Args.Path, "path", false,
		"The key contains a path to a property in a map or list to remove")

	return rmCmd
}

func (cmd *configRmCmd) run(ctx context.Context, keyArg string) error {
	opts := display.Options{
		Color: cmd.Args.Colorizer,
	}

	project, _, err := cmd.Workspace.ReadProject()
	if err != nil {
		return err
	}

	stack, err := cmdStack.RequireStack(
		ctx,
		cmd.Workspace,
		cmd.LoginManager,
		cmd.Args.Stack,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		opts,
	)
	if err != nil {
		return err
	}

	key, err := ParseConfigKey(keyArg)
	if err != nil {
		return fmt.Errorf("invalid configuration key: %w", err)
	}

	ps, err := cmd.ProjectStackManager.Load(project, stack)
	if err != nil {
		return err
	}

	err = ps.Config.Remove(key, cmd.Args.Path)
	if err != nil {
		return err
	}

	return cmd.ProjectStackManager.Save(stack, ps)
}
