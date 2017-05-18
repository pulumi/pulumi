// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/util/cmdutil"
)

func NewLumiCmd() *cobra.Command {
	var logToStderr bool
	var verbose int
	cmd := &cobra.Command{
		Use:   "lumi",
		Short: "Lumi is a framework and toolset for reusable stacks of services",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmdutil.InitLogging(logToStderr, verbose)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
		},
	}

	cmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDeployCmd())
	cmd.AddCommand(newDestroyCmd())
	cmd.AddCommand(newEnvCmd())
	cmd.AddCommand(newPackCmd())
	cmd.AddCommand(newPlanCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}
