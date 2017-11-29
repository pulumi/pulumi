// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// newStackSelectCmd handles both the "local" and "cloud" scenarios in its implementation.
func newStackSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select [<stack>]",
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
				name, err := getCurrentStack()
				if err != nil {
					return err
				}

				fmt.Printf("%v\n", name)
				return nil
			}

			selectedStack := tokens.QName(args[0])

			// Confirm the stack name is valid.
			summeries, err := backend.GetStacks()
			if err != nil {
				return err
			}

			for _, stack := range summeries {
				if stack.Name == selectedStack {
					return setCurrentStack(selectedStack)
				}
			}

			return errors.Errorf("no stack with name '%v' found", selectedStack)
		}),
	}
}
