// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
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
			bes, hasClouds := allBackends()
			if len(args) > 0 {
				// A stack was given, ask all known backends about it
				stackName := tokens.QName(args[0])

				var result error
				for _, b := range bes {
					stack, err := b.GetStack(stackName)
					if err != nil {
						// If there is an error, file it away, but keep going in case it's a transient cloud error.
						result = multierror.Append(result, errors.Wrapf(err,
							"could not query '%s' backend for stack selection", b.Name()))
						continue
					} else if stack != nil {
						return state.SetCurrentStack(stackName)
					}
				}

				// If we fell through, the stack was not found.  Issue an error.  Also customize the error
				// message if no clouds are logged into, since that is presumably a common mistake.
				msg := fmt.Sprintf("no stack named '%s' found", stackName)
				if !hasClouds {
					msg += "; you aren't logged into the Pulumi Cloud -- did you forget to 'pulumi login'?"
				}
				return multierror.Append(result, errors.New(msg))
			}

			// If no stack was given, prompt the user to select a name from the available ones.
			stack, err := chooseStack(bes, true)
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
