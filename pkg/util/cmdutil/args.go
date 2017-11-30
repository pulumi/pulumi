// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"github.com/spf13/cobra"
)

// ArgsFunc wraps a standard cobra argument validator with standard Pulumi error handling.
func ArgsFunc(argsValidator cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := argsValidator(cmd, args)
		if err != nil {
			ExitError(err.Error())
		}

		return nil
	}
}

// NoArgs is the same as cobra.NoArgs, except it is wrapped with ArgsFunc to provide standard
// Pulumi error handling.
var NoArgs = ArgsFunc(cobra.NoArgs)

// MaximumNArgs is the same as cobra.MaximumNArgs, except it is wrapped with ArgsFunc to provide standard
// Pulumi error handling.
func MaximumNArgs(n int) cobra.PositionalArgs {
	return ArgsFunc(cobra.MaximumNArgs(n))
}

// ExactArgs is the same as cobra.ExactArgs, except it is wrapped with ArgsFunc to provide standard
// Pulumi error handling.
func ExactArgs(n int) cobra.PositionalArgs {
	return ArgsFunc(cobra.ExactArgs(n))
}

// RangeArgs is the same as cobra.RangeArgs, except it is wrapped with ArgsFunc to provide standard
// Pulumi error handling.
func RangeArgs(min int, max int) cobra.PositionalArgs {
	return ArgsFunc(cobra.RangeArgs(min, max))
}
