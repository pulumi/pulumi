// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"flag"
	"strconv"

	"github.com/spf13/cobra"
)

func NewMuCmd() *cobra.Command {
	var verbose int

	cmd := &cobra.Command{
		Use:   "mu",
		Short: "Mu is a framework and toolset for reusable stacks of services",
		Run: func(cmd *cobra.Command, args []string) {
			if verbose > 0 {
				// Enable verbose logging in glog at the specified level.  Poking around at the flags by name feels
				// kind of hacky, however it's how the glog library works.
				flag.Lookup("v").Value.Set(strconv.Itoa(verbose))
			}
		},
	}

	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0,
		"Enable verbose logging (e.g., v=3); warning: anything >3 is very verbose",
	)

	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}
