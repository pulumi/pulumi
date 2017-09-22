// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newPushCmd() *cobra.Command {
	var analyzers []string
	var debug bool
	var dryRun bool
	var env string
	var parallel int
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var summary bool
	var cmd = &cobra.Command{
		Use:   "push [<package>] [-- [<args>]]",
		Short: "Push resource updates, creations, and deletions to an environment",
		Long: "Push resource updates, creations, and deletions to an environment\n" +
			"\n" +
			"This command updates an existing environment whose state is represented by the\n" +
			"existing snapshot file. The new desired state is computed by compiling and evaluating an\n" +
			"executable package, and extracting all resource allocations from its resulting object graph.\n" +
			"These allocations are then compared against the existing state to determine what operations\n" +
			"must take place to achieve the desired state. This command results in a full snapshot of the\n" +
			"environment's new resource state, so that it may be updated incrementally again later.\n" +
			"\n" +
			"By default, the package to execute is loaded from the current directory. Optionally, an\n" +
			"explicit path can be provided using the [package] argument.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return lumiEngine.Deploy(engine.DeployOptions{
				Environment:          env,
				Package:              pkgargFromArgs(args),
				Debug:                debug,
				DryRun:               dryRun,
				Analyzers:            analyzers,
				Parallel:             parallel,
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSames:            showSames,
				Summary:              summary,
			})
		}),
	}

	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this push")
	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&env, "env", "e", "",
		"Choose an environment other than the currently selected one")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", true,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and operations")

	return cmd
}
