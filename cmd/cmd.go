// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "mu",
	Short: "Mu is a framework and toolset for reusable stacks of services",
}

func init() {
	Cmd.AddCommand(newBuildCmd())
	Cmd.AddCommand(newVersionCmd())
}
