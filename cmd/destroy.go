// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newDestroyCmd() *cobra.Command {
	var debug bool
	var dryRun bool
	var env string
	var parallel int
	var summary bool
	var yes bool
	var cmd = &cobra.Command{
		Use:     "destroy",
		Aliases: []string{"down", "stop"},
		Short:   "Destroy an existing environment and its resources",
		Long: "Destroy an existing environment and its resources\n" +
			"\n" +
			"This command deletes an entire existing environment by name.  The current state is\n" +
			"loaded from the associated snapshot file in the workspace.  After running to completion,\n" +
			"all of this environment's resources and associated state will be gone.\n" +
			"\n" +
			"Warning: although old snapshots can be used to recreate an environment, this command\n" +
			"is generally irreversable and should be used with great care.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if env == "" {
				env = lumiEngine.GetCurrentEnvName()
			}

			if dryRun || yes ||
				confirmPrompt("This will permanently destroy all resources in the '%v' environment!", env) {
				return lumiEngine.Destroy(engine.DestroyOptions{
					Environment: env,
					Package:     pkgargFromArgs(args),
					DryRun:      dryRun,
					Debug:       debug,
					Parallel:    parallel,
					Summary:     summary,
				})
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually delete resources; just print out the planned deletions")
	cmd.PersistentFlags().StringVarP(
		&env, "env", "e", "",
		"Choose an environment other than the currently selected one")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with the destruction anyway")

	return cmd
}
