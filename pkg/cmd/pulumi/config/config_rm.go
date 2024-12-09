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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newConfigRmCmd(stack *string) *cobra.Command {
	var path bool

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
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			stack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.OfferNew|cmdStack.SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			key, err := ParseConfigKey(args[0])
			if err != nil {
				return fmt.Errorf("invalid configuration key: %w", err)
			}

			ps, err := cmdStack.LoadProjectStack(project, stack)
			if err != nil {
				return err
			}

			err = ps.Config.Remove(key, path)
			if err != nil {
				return err
			}

			return cmdStack.SaveProjectStack(stack, ps)
		}),
	}
	rmCmd.PersistentFlags().BoolVar(
		&path, "path", false,
		"The key contains a path to a property in a map or list to remove")

	return rmCmd
}
