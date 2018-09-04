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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newCancelCmd() *cobra.Command {
	var yes bool
	var cmd = &cobra.Command{
		Use:   "cancel [<stack-name>]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Cancel a stack's currently running update, if any",
		Long: "Cancel a stack's currently running update, if any.\n" +
			"\n" +
			"This command cancels the update currently being applied to a stack if any exists.\n" +
			"Note that this operation is _very dangerous_, and may leave the stack in an\n" +
			"inconsistent state if a resource operation was pending when the update was canceled.\n" +
			"\n" +
			"After this command completes successfully, the stack will be ready for further\n" +
			"updates.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Use the stack provided or, if missing, default to the current one.
			stack := ""
			if len(args) > 0 {
				stack = args[0]
			}

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stack, false, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}

			// Ensure that we are targeting the Pulumi cloud.
			backend, ok := s.Backend().(httpstate.Backend)
			if !ok {
				return errors.New("the `cancel` command is not supported for local stacks")
			}

			// Ensure the user really wants to do this.
			prompt := fmt.Sprintf("This will irreversibly cancel the currently running update for '%s'!", s.Name())
			if !yes && !confirmPrompt(prompt, s.Name().String(), opts) {
				return errors.New("confirmation declined")
			}

			// Cancel the update.
			if err := backend.CancelCurrentUpdate(commandContext(), s.Name()); err != nil {
				return err
			}

			msg := fmt.Sprintf("%sThe currently running update for '%s' has been canceled!%s", colors.SpecAttention, s.Name(),
				colors.Reset)
			fmt.Println(opts.Color.Colorize(msg))

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with cancellation anyway")

	return cmd
}
