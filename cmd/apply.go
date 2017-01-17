// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "apply [blueprint] [-- [args]]",
		Short: "Apply a deployment plan from a Mu blueprint",
		Long: "Apply a deployment plan from a Mu blueprint.\n" +
			"\n" +
			"This command performs creation, update, and delete operations against a target\n" +
			"environment, in order to carry out the activities laid out in a deployment plan.\n" +
			"\n" +
			"These activities are the result of compiling a MuPackage blueprint into a MuGL\n" +
			"graph, and comparing that graph to another one representing the current known\n" +
			"state of the target environment.  After the completion of this command, the target\n" +
			"environment's state will have been advanced so that it matches the input blueprint." +
			"\n" +
			"By default, a blueprint package is loaded from the current directory.  Optionally,\n" +
			"a path to a blueprint elsewhere can be provided as the [blueprint] argument.",
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	// TODO: options; most importantly, what to compare the blueprint against.

	return cmd
}
