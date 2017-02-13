// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/compiler/types/predef"
	"github.com/marapongo/mu/pkg/graph"
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
			// Perform the compilation and, if non-nil is returned, output the plan.
			if mugl := compile(cmd, args); mugl != nil {
				// Sort the graph output so that it's a DAG.
				// TODO: consider pruning out all non-resources so there is less to sort.
				sorted, err := graph.TopSort(mugl)
				if err != nil {
					fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
					os.Exit(-1)
				}

				// Now walk the elements and (for now), just print out which resources will be created.
				for _, vert := range sorted {
					o := vert.Obj()
					t := o.Type()
					if types.HasBaseName(t, predef.MuResourceClass) {
						fmt.Printf("%v:\n", o.Type())
						for key, prop := range o.Properties() {
							fmt.Printf("\t%v: %v\n", key, prop)
						}
					}
				}
			}
		},
	}

	return cmd
}
