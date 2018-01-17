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
	var stack string
	var yes bool

	var color string
	var opts engine.UpdateOptions

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
			pkg, root, err := readPackage()
			if err != nil {
				return err
			}

			// The --color flag doesn't directly change the opts.Color value, so we parse and set it here.
			col, err := parseColorization(debug, color)
			if err != nil {
				return err
			}
			opts.Color = col

			if opts.DryRun || yes ||
				confirmPrompt("This will permanently destroy all resources in the '%v' stack!", string(s.Name())) {
				return s.Destroy(pkg, root, debug, opts)
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with the destruction anyway")

	// Flags for setting engine.UpdateOptions.
	cmd.PersistentFlags().StringVar(
		&color, "color", "auto",
		"Colorize output. Choices are: always, never, raw, auto")
	cmd.PersistentFlags().StringSliceVar(
		&opts.Analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().BoolVarP(
		&opts.DryRun, "dry-run", "r", false,
		"Don't create/delete resources; just preview the planned operations")
	cmd.PersistentFlags().IntVarP(
		&opts.Parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVar(
		&opts.ShowConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&opts.ShowReplacementSteps, "show-replacement-steps", true,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&opts.ShowSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&opts.Summary, "summary", false,
		"Only display summarization of resources and operations")

	return cmd
}
