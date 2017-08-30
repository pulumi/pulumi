// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/engine"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
)

func newDeployCmd() *cobra.Command {
	var analyzers []string
	var debug bool
	var dryRun bool
	var env string
	var showConfig bool
	var showReads bool
	var showReplacementSteps bool
	var showSames bool
	var summary bool
	var cmd = &cobra.Command{
		Use:     "deploy [<package>] [-- [<args>]]",
		Aliases: []string{"run", "up", "update"},
		Short:   "Deploy resource updates, creations, and deletions to an environment",
		Long: "Deploy resource updates, creations, and deletions to an environment\n" +
			"\n" +
			"This command updates an existing environment whose state is represented by the\n" +
			"existing snapshot file.  The new desired state is computed by compiling and evaluating an\n" +
			"executable package, and extracting all resource allocations from its resulting object graph.\n" +
			"This graph is compared against the existing state to determine what operations must take\n" +
			"place to achieve the desired state.  This command results in a full snapshot of the\n" +
			"environment's new resource state, so that it may be updated incrementally again later.\n" +
			"\n" +
			"By default, the package to execute is loaded from the current directory.  Optionally, an\n" +
			"explicit path can be provided using the [package] argument.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return lumiEngine.Deploy(engine.DeployOptions{
				Environment:          env,
				Package:              pkgargFromArgs(args),
				Debug:                debug,
				DryRun:               dryRun,
				Analyzers:            analyzers,
				ShowConfig:           showConfig,
				ShowReads:            showReads,
				ShowReplacementSteps: showReplacementSteps,
				ShowSames:            showSames,
				Summary:              summary,
			})
		}),
	}

	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this deployment")
	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually update resources, just print out the planned updates (synonym for plan)")
	cmd.PersistentFlags().StringVarP(
		&env, "env", "e", "",
		"Choose an environment other than the currently selected one")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReads, "show-reads", false,
		"Show resources that will be read, in addition to those that will be modified")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", true,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")

	return cmd
}
