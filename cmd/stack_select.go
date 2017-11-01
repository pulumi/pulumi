// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select [<stack>]",
		Short: "Switch the current workspace to the given stack",
		Long: "Switch the current workspace to the given stack.  This allows you to use\n" +
			"other commands like `config`, `preview`, and `push` without needing to specify the\n" +
			"stack name each and every time.\n" +
			"\n" +
			"If no <stack> argument is supplied, the current stack is printed.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the stack to switch to.
			if len(args) == 0 {
				name, err := getCurrentStack()
				if err != nil {
					return err
				}

				fmt.Printf("%v\n", name)
				return nil
			}

			allStacks, err := getStacks()
			if err != nil {
				return err
			}

			// Confirm the stack name is valid.
			selectedStack := tokens.QName(args[0])
			for _, stack := range allStacks {
				if stack == selectedStack {
					return setCurrentStack(selectedStack)
				}
			}

			return fmt.Errorf("no stack with name '%v' found", selectedStack)
		}),
	}
}
