// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/tokens"
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
			b, err := currentBackend()
			if err != nil {
				return err
			}

			if len(args) > 0 {
				// A stack was given, ask all known backends about it
				stackName := tokens.QName(args[0])

				stack, stackErr := b.GetStack(stackName)
				if stackErr != nil {
					return stackErr
				} else if stack != nil {
					return state.SetCurrentStack(stackName)
				}

				return errors.Errorf("no stack named '%s' found", stackName)
			}

			// If no stack was given, prompt the user to select a name from the available ones.
			stack, err := chooseStack(b, true)
			if err != nil {
				return err
			}
			return state.SetCurrentStack(stack.Name())

		}),
	}
	cmd.PersistentFlags().StringVarP(
		&cloud, "cloud", "c", "", "A URL for the Pulumi Cloud containing the stack to be selected")
	return cmd
}
