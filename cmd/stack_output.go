// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackOutputCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "output [property-name]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Show a stack's output properties",
		Long: "Show a stack's output properties.\n" +
			"\n" +
			"By default, this command lists all output properties exported from a stack.\n" +
			"If a specific property-name is supplied, just that property's value is shown.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Fetch the current stack and its output properties.
			s, err := requireCurrentStack(false)
			if err != nil {
				return err
			}
			res, outputs := stack.GetRootStackResource(s.Snapshot())
			if res == nil || outputs == nil {
				return errors.New("current stack has no output properties")
			}

			// If there is an argument, just print that property.  Else, print them all (similar to `pulumi stack`).
			if len(args) > 0 {
				name := args[0]
				v, has := outputs[name]
				if has {
					fmt.Printf("%v\n", stringifyOutput(v))
				} else {
					return errors.Errorf("current stack does not have output property '%v'", name)
				}
			} else {
				printStackOutputs(outputs)
			}
			return nil
		}),
	}
}
