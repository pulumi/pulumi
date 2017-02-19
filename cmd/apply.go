// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/compiler/errors"
)

func newApplyCmd() *cobra.Command {
	var delete bool
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
			if comp, plan := plan(cmd, args, delete); plan != nil {
				if _, err, _ := plan.Apply(); err != nil {
					// TODO: we want richer diagnostics in the event that a plan apply fails.  For instance, we want to
					//     know precisely what step failed, we want to know whether it was catastrophic, etc.  We also
					//     probably want to plumb diag.Sink through apply so it can issue its own rich diagnostics.
					comp.Diag().Errorf(errors.ErrorPlanApplyFailed, err)
				}
			}
		},
	}

	// TODO: options; most importantly, what to compare the blueprint against.
	cmd.PersistentFlags().BoolVar(
		&delete, "delete", false,
		"Delete the entirety of the blueprint's resources")

	return cmd
}
