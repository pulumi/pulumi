// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newPreviewCmd() *cobra.Command {
	var analyzers []string
	var debug bool
	var env string
	var parallel int
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var summary bool
	var cmd = &cobra.Command{
		Use:   "preview [<package>] [-- [<args>]]",
		Short: "Show a preview of changes to an environment's resources",
		Long: "Show a preview of changes an environment's resources\n" +
			"\n" +
			"This command displays a preview of the changes to an existing environment whose state is\n" +
			"represented by an existing snapshot file. The new desired state is computed by compiling\n" +
			"and evaluating an executable package, and extracting all resource allocations from its\n" +
			"resulting object graph. These allocations are then compared against the existing state to\n" +
			"determine what operations must take place to achieve the desired state. No changes to the\n" +
			"environment will actually take place.\n" +
			"\n" +
			"By default, the package to execute is loaded from the current directory. Optionally, an\n" +
			"explicit path can be provided using the [package] argument.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return lumiEngine.Preview(engine.PreviewOptions{
				Package:              pkgargFromArgs(args),
				Debug:                debug,
				Environment:          env,
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
		"Run one or more analyzers as part of this preview")
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
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and operations")

	return cmd
}
