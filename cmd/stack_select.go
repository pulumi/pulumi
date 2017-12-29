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
		Use:   "select [<stack-name>]",
		Short: "Switch the current workspace to the given stack",
		Long: "Switch the current workspace to the given stack.  This allows you to use\n" +
			"other commands like `config`, `preview`, and `push` without needing to specify the\n" +
			"stack name each and every time.\n" +
			"\n" +
			"If no <stack> argument is supplied, the current stack is printed.",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Display the name of the current stack if a new one isn't specified.
			if len(args) == 0 {
				name, err := requireCurrentStack()
				if err != nil {
					return err
				}

				fmt.Printf("%v\n", name)
				return nil
			}

			// Ask all known backends about this stack.
			var result error
			bes, hasClouds := allBackends()
			toSelect := tokens.QName(args[0])
			for _, b := range bes {
				stack, err := b.GetStack(toSelect)
				if err != nil {
					// If there is an error, file it away, but keep going in case it's a transient cloud error.
					result = multierror.Append(result, errors.Wrapf(err,
						"could not query '%s' backend for stack selection", b.Name()))
					continue
				} else if stack != nil {
					return state.SetCurrentStack(toSelect)
				}
			}

			// If we fell through, the stack was not found.  Issue an error.  Also customize the error
			// message if no clouds are logged into, since that is presumably a common mistake.
			msg := fmt.Sprintf("no stack named '%s' found", toSelect)
			if !hasClouds {
				msg += "; you aren't logged into the Pulumi Cloud -- did you forget to 'pulumi login'?"
			}
			return multierror.Append(result, errors.New(msg))
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&cloud, "cloud", "c", "", "A URL for the Pulumi Cloud containing the stack to be selected")
	return cmd
}
