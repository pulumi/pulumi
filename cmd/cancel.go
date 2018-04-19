// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newCancelCmd() *cobra.Command {
	var yes bool
	var cmd = &cobra.Command{
		Use:   "cancel [<stack-name>]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Cancel a stack's currently running update, if any",
		Long: "Cancel a stack's currently running, if any.\n" +
			"\n" +
			"This command cancels the update currently being applied to a stack if any exists.\n" +
			"Note that this operation is _very dangerous_, and may leave the stack in an\n" +
			"inconsistent state if a resource operation was pending when the update was canceled.\n" +
			"\n" +
			"After this command completes successfully, the stack will be ready for further\n" +
			"updates.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Use the stack provided or, if missing, default to the current one.
			var stack tokens.QName
			if len(args) > 0 {
				stack = tokens.QName(args[0])
			}
			s, err := requireStack(stack, false)
			if err != nil {
				return err
			}

			// Ensure that we are targeting the Pulumi cloud.
			backend, ok := s.Backend().(cloud.Backend)
			if !ok {
				return errors.New("the `cancel` command is not supported for local stacks")
			}

			// Ensure the user really wants to do this.
			prompt := fmt.Sprintf("This will irreversably cancel the currently running update for '%s'!", s.Name())
			if !yes && !confirmPrompt(prompt, string(s.Name())) {
				return errors.New("confirmation declined")
			}

			// Cancel the update.
			if err := backend.CancelCurrentUpdate(s.Name()); err != nil {
				return err
			}

			msg := fmt.Sprintf("%sThe currently running update for '%s' has been canceled!%s", colors.SpecAttention, s.Name(),
				colors.Reset)
			fmt.Println(colors.ColorizeText(msg))

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with cancellation anyway")

	return cmd
}
