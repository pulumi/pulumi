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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newStackRmCmd() *cobra.Command {
	var yes bool
	var force bool
	var cmd = &cobra.Command{
		Use:   "rm [<stack-name>]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Remove a stack and its configuration",
		Long: "Remove a stack and its configuration\n" +
			"\n" +
			"This command removes a stack and its configuration state.  Please refer to the\n" +
			"`destroy` command for removing a resources, as this is a distinct operation.\n" +
			"\n" +
			"After this command completes, the stack will no longer be available for updates.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Use the stack provided or, if missing, default to the current one.
			var stack string
			if len(args) > 0 {
				stack = args[0]
			}

			opts := backend.DisplayOptions{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stack, false, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}

			// Ensure the user really wants to do this.
			prompt := fmt.Sprintf("This will permanently remove the '%s' stack!", s.Name())
			if !yes && !confirmPrompt(prompt, s.Name().String(), opts) {
				return errors.New("confirmation declined")
			}

			hasResources, err := s.Remove(commandContext(), force)
			if err != nil {
				if hasResources {
					return errors.Errorf(
						"'%s' still has resources; removal rejected; pass --force to override", s.Name())
				}
				return err
			}

			// Blow away stack specific settings if they exist
			path, err := workspace.DetectProjectStackPath(s.Name().StackName())
			if err != nil {
				return err
			}

			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}

			msg := fmt.Sprintf("%sStack '%s' has been removed!%s", colors.SpecAttention, s.Name(), colors.Reset)
			fmt.Println(opts.Color.Colorize(msg))

			return state.SetCurrentStack("")
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Forces deletion of the stack, leaving behind any resources managed by the stack")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with removal anyway")

	return cmd
}
