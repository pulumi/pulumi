// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "plan [blueprint] [-- [args]]",
		Short: "Generate a deployment plan from a Mu blueprint",
		Long: "Generate a deployment plan from a Mu blueprint.\n" +
			"\n" +
			"A plan describes the overall graph and set of operations that will be performed\n" +
			"as part of a Mu deployment.  No actual resource creations, updates, or deletions\n" +
			"will take place.  This plan is as complete as possible without actually performing\n" +
			"the operations described in the plan (with the caveat that conditional execution\n" +
			"may obscure certain details, something that will be evident in plan's output).\n" +
			"\n" +
			"By default, a blueprint package is loaded from the current directory.  Optionally,\n" +
			"a path to a blueprint elsewhere can be provided as the [blueprint] argument.",
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	// TODO: options; most importantly, what to compare the blueprint against.

	return cmd
}
