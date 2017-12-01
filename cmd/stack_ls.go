// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all known stacks",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			currentStack, err := getCurrentStack()
			if err != nil {
				// If we couldn't figure out the current stack, just don't print the '*' later
				// on instead of failing.
				currentStack = tokens.QName("")
			}

			summaries, err := backend.GetStacks()
			if err != nil {
				return err
			}

			fmt.Printf("%-20s %-48s %-12s\n", "NAME", "LAST UPDATE", "RESOURCE COUNT")
			for _, stack := range summaries {
				if stack.Name == currentStack {
					stack.Name += "*"
				}
				fmt.Printf("%-20s %-48s %-12s\n", stack.Name, stack.LastDeploy, stack.ResourceCount)
			}
			return nil
		}),
	}
}
