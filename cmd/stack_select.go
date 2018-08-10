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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// newStackSelectCmd handles both the "local" and "cloud" scenarios in its implementation.
func newStackSelectCmd() *cobra.Command {
	var cloud string
	cmd := &cobra.Command{
		Use:   "select [<stack>]",
		Short: "Switch the current workspace to the given stack",
		Long: "Switch the current workspace to the given stack.\n" +
			"\n" +
			"Selecting a stack allows you to use commands like `config`, `preview`, and `update`\n" +
			"without needing to type the stack name each time.\n" +
			"\n" +
			"If no <stack> argument is supplied, you will be prompted to select one interactively.",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := backend.DisplayOptions{
				Color: cmdutil.GetGlobalColorization(),
			}

			b, err := currentBackend(opts)
			if err != nil {
				return err
			}

			if len(args) > 0 {
				// A stack was given, ask all known backends about it
				stackRef, stackErr := b.ParseStackReference(args[0])
				if stackErr != nil {
					return stackErr
				}

				stack, stackErr := b.GetStack(commandContext(), stackRef)
				if stackErr != nil {
					return stackErr
				} else if stack != nil {
					return state.SetCurrentStack(stackRef.String())
				}

				return errors.Errorf("no stack named '%s' found", stackRef)
			}

			// If no stack was given, prompt the user to select a name from the available ones.
			stack, err := chooseStack(b, true, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}
			return state.SetCurrentStack(stack.Name().String())

		}),
	}
	cmd.PersistentFlags().StringVarP(
		&cloud, "cloud", "c", "", "A URL for the Pulumi Cloud containing the stack to be selected")
	return cmd
}
