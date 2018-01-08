// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newDestroyCmd() *cobra.Command {
	var debug bool
	var preview bool
	var stack string
	var parallel int
	var summary bool
	var yes bool
	var color string
	var cmd = &cobra.Command{
		Use:        "destroy",
		SuggestFor: []string{"delete", "down", "kill", "remove", "rm", "stop"},
		Short:      "Destroy an existing stack and its resources",
		Long: "Destroy an existing stack and its resources\n" +
			"\n" +
			"This command deletes an entire existing stack by name.  The current state is\n" +
			"loaded from the associated snapshot file in the workspace.  After running to completion,\n" +
			"all of this stack's resources and associated state will be gone.\n" +
			"\n" +
			"Warning: although old snapshots can be used to recreate an stack, this command\n" +
			"is generally irreversable and should be used with great care.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			s, err := requireStack(tokens.QName(stack))
			if err != nil {
				return err
			}

			col, err := parseColorization(debug, color)
			if err != nil {
				return err
			}

			pkg, root, err := readPackage()
			if err != nil {
				return err
			}

			if preview || yes ||
				confirmPrompt("This will permanently destroy all resources in the '%v' stack!", string(s.Name())) {

				return s.Destroy(pkg, root, debug, engine.DestroyOptions{
					DryRun:   preview,
					Parallel: parallel,
					Summary:  summary,
					Color:    col,
				})
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVarP(
		&preview, "preview", "n", false,
		"Don't actually delete resources; just preview the planned deletions")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVar(
		&summary, "summary", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with the destruction anyway")
	cmd.PersistentFlags().StringVar(
		&color, "color", "auto",
		"Colorize output. Choices are: always, never, raw, auto")

	return cmd
}
