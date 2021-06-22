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

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newStackRmCmd() *cobra.Command {
	var stack string
	var yes bool
	var force bool
	var preserveConfig bool
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
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			yes = yes || skipConfirmations()
			// Use the stack provided or, if missing, default to the current one.
			if len(args) > 0 {
				if stack != "" {
					return result.Error("only one of --stack or argument stack name may be specified, not both")
				}
				stack = args[0]
			}

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stack, false, opts, false /*setCurrent*/)
			if err != nil {
				return result.FromError(err)
			}

			// Ensure the user really wants to do this.
			prompt := fmt.Sprintf("This will permanently remove the '%s' stack!", s.Ref())
			if !yes && !confirmPrompt(prompt, s.Ref().String(), opts) {
				fmt.Println("confirmation declined")
				return result.Bail()
			}

			hasResources, err := s.Remove(commandContext(), force)
			if err != nil {
				if hasResources {
					return result.Errorf(
						"'%s' still has resources; removal rejected; pass --force to override", s.Ref())
				}
				return result.FromError(err)
			}

			if !preserveConfig {
				// Blow away stack specific settings if they exist. If we get an ENOENT error, ignore it.
				if path, err := workspace.DetectProjectStackPath(s.Ref().Name()); err == nil {
					if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
						return result.FromError(err)
					}
				}
			}

			msg := fmt.Sprintf("%sStack '%s' has been removed!%s", colors.SpecAttention, s.Ref(), colors.Reset)
			fmt.Println(opts.Color.Colorize(msg))

			contract.IgnoreError(state.SetCurrentStack(""))
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Forces deletion of the stack, leaving behind any resources managed by the stack")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with removal anyway")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().BoolVar(
		&preserveConfig, "preserve-config", false,
		"Do not delete the corresponding Pulumi.<stack-name>.yaml configuration file for the stack")

	return cmd
}
