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

package cancel

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/adder"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func NewCancelCmd(env adder.Environment) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a stack's currently running update, if any",
		Long: "Cancel a stack's currently running update, if any.\n" +
			"\n" +
			"This command cancels the update currently being applied to a stack if any exists.\n" +
			"Note that this operation is _very dangerous_, and may leave the stack in an\n" +
			"inconsistent state if a resource operation was pending when the update was canceled.\n" +
			"\n" +
			"After this command completes successfully, the stack will be ready for further\n" +
			"updates.",
	}
	stack := adder.StackFlag(cmd, "")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		// Use the stack provided or, if missing, default to the current one.
		if len(args) > 0 {
			if flag, err := cmd.Flags().GetString("stack"); err != nil {
				return err
			} else if flag != "" {
				return errors.New("only one of --stack or argument stack name may be specified, not both")
			}
			if err := cmd.Flags().Set("stack", args[0]); err != nil {
				return err
			}
		}

		opts := display.Options{
			Color: cmdutil.GetGlobalColorization(),
		}

		s, err := stack.Resolve(cmd, env)
		if err != nil {
			return err
		}

		// Ensure the user really wants to do this.
		stackName := s.Ref().Name().String()
		prompt := fmt.Sprintf("This will irreversibly cancel the currently running update for '%s'!", stackName)
		if cmdutil.Interactive() && (!yes && !ui.ConfirmPrompt(prompt, stackName, opts)) {
			return result.FprintBailf(cmd.OutOrStdout(), "confirmation declined")
		}

		// Cancel the update.
		if err := s.Backend().CancelCurrentUpdate(ctx, s.Ref()); err != nil {
			return err
		}

		msg := fmt.Sprintf(
			"%sThe currently running update for '%s' has been canceled!%s",
			colors.SpecAttention, stackName, colors.Reset)
		fmt.Fprintln(cmd.OutOrStdout(), opts.Color.Colorize(msg))

		return nil
	}
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "stack-name"}},
		Required:  0,
	})
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with cancellation anyway")

	return cmd
}
