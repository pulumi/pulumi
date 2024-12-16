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

type configRmAllCmd struct {
	// Parsed arguments to the command.
	Args *configRmAllArgs

	// The command's standard output.
	Stdout io.Writer

	// The workspace to operate on.
	Workspace pkgWorkspace.Context
	// The login manager to use for authenticating with and loading backends.
	LoginManager cmdBackend.LoginManager
	// The project stack manager to use for loading and saving project stack configuration.
	ProjectStackManager cmdStack.ProjectStackManager
}

// A set of arguments for the `config rm-all` command.
type configRmAllArgs struct {
	Colorizer colors.Colorization
	Path      bool
	Stack     string
}

func newConfigRmAllCmd(configFile *string, stack *string) *cobra.Command {
	configRmAll := &configRmAllCmd{
		Args: &configRmAllArgs{
			Colorizer: cmdutil.GetGlobalColorization(),
		},
		Stdout:       os.Stdout,
		Workspace:    pkgWorkspace.Instance,
		LoginManager: cmdBackend.DefaultLoginManager,
	}

	rmAllCmd := &cobra.Command{
		Use:   "rm-all <key1> <key2> <key3> ...",
		Short: "Remove multiple configuration values",
		Long: "Remove multiple configuration values.\n\n" +
			"The `--path` flag indicates that keys should be parsed within maps or lists:\n\n" +
			"  - `pulumi config rm-all --path  outer.inner 'foo[0]' key1` will remove the \n" +
			"    `inner` key of the `outer` map, the first key of the `foo` list and `key1`.\n" +
			"  - `pulumi config rm-all outer.inner 'foo[0]' key1` will remove the literal" +
			"    `outer.inner`, `foo[0]` and `key1` keys",
		Args: cmdutil.MinimumNArgs(1),
		Run: cmd.RunCmdFunc(func(command *cobra.Command, args []string) error {
			configRmAll.Args.Stack = *stack
			configRmAll.ProjectStackManager = cmdStack.NewProjectStackManager(*configFile)

			ctx := command.Context()
			err := configRmAll.run(ctx, args)

			return err
		}),
	}
	rmAllCmd.PersistentFlags().BoolVar(
		&configRmAll.Args.Path, "path", false,
		"Parse the keys as paths in a map or list rather than raw strings")

	return rmAllCmd
}

func (cmd *configRmAllCmd) run(ctx context.Context, args []string) error {
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
		cmdStack.OfferNew,
		opts,
	)
	if err != nil {
		return err
	}

	ps, err := cmd.ProjectStackManager.Load(project, stack)
	if err != nil {
		return err
	}

	for _, arg := range args {
		key, err := ParseConfigKey(arg)
		if err != nil {
			return fmt.Errorf("invalid configuration key: %w", err)
		}

		err = ps.Config.Remove(key, cmd.Args.Path)
		if err != nil {
			return err
		}
	}

	return cmd.ProjectStackManager.Save(stack, ps)
}
