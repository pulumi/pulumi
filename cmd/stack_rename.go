// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newStackRenameCmd() *cobra.Command {
	var stack string
	var cmd = &cobra.Command{
		Use:   "rename <new-stack-name>",
		Args:  cmdutil.ExactArgs(1),
		Short: "Rename an existing stack",
		Long: "Rename an existing stack.\n" +
			"\n" +
			"Note: Because renaming a stack will change the value of `getStack()` inside a Pulumi program, if this\n" +
			"name is used as part of a resource's name, the next `pulumi up` will want to delete the old resource and\n" +
			"create a new copy. For now, if you don't want these changes to be applied, you should rename your stack\n" +
			"back to its previous name.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stack, false, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}

			oldConfigPath, err := workspace.DetectProjectStackPath(s.Ref().Name())
			if err != nil {
				return err
			}

			newConfigPath, err := workspace.DetectProjectStackPath(tokens.QName(args[0]))
			if err != nil {
				return err
			}

			if err := s.Rename(commandContext(), tokens.QName(args[0])); err != nil {
				return err
			}

			if err := os.Rename(oldConfigPath, newConfigPath); err != nil {
				return errors.Wrapf(err, "renaming %s to %s", filepath.Base(oldConfigPath), filepath.Base(newConfigPath))
			}

			if err := state.SetCurrentStack(args[0]); err != nil {
				return errors.Wrap(err, "setting current stack")
			}

			fmt.Printf("Renamed %s\n", s.Ref().String())
			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	return cmd
}
