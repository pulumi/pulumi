// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/engine"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

func newPlanCmd() *cobra.Command {
	var analyzers []string
	var debug bool
	var dotOutput bool
	var env string
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var summary bool
	var cmd = &cobra.Command{
		Use:     "plan [<package>] [-- [<args>]]",
		Aliases: []string{"dryrun"},
		Short:   "Show a plan to update, create, and delete an environment's resources",
		Long: "Show a plan to update, create, and delete an environment's resources\n" +
			"\n" +
			"This command displays a plan to update an existing environment whose state is represented by\n" +
			"an existing snapshot file.  The new desired state is computed by compiling and evaluating an\n" +
			"executable package, and extracting all resource allocations from its resulting object graph.\n" +
			"This graph is compared against the existing state to determine what operations must take\n" +
			"place to achieve the desired state.  No changes to the environment will actually take place.\n" +
			"\n" +
			"By default, the package to execute is loaded from the current directory.  Optionally, an\n" +
			"explicit path can be provided using the [package] argument.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			contract.Assertf(!dotOutput, "TODO[pulumi/pulumi-fabric#235]: DOT files not yet supported")

			return lumiEngine.Plan(engine.PlanOptions{
				Package:              pkgargFromArgs(args),
				Debug:                debug,
				Environment:          env,
				Analyzers:            analyzers,
				ShowConfig:           showConfig,
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
	cmd.PersistentFlags().BoolVar(
		&dotOutput, "dot", false,
		"Output the plan as a DOT digraph (graph description language)")
	cmd.PersistentFlags().StringVarP(
		&env, "env", "e", "",
		"Choose an environment other than the currently selected one")
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
		"Only display summarization of resources and plan operations")

	return cmd
}
