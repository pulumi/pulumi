// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newDestroyCmd() *cobra.Command {
	var debug bool
	var stack string
	var yes bool

	var message string

	// Flags for engine.UpdateOptions.
	var analyzers []string
	var color colorFlag
	var parallel int
	var preview bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var diffDisplay bool

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
			"Warning: although old snapshots can be used to recreate a stack, this command\n" +
			"is generally irreversable and should be used with great care.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			s, err := requireStack(tokens.QName(stack), false)
			if err != nil {
				return err
			}
			proj, root, err := readProject()
			if err != nil {
				return err
			}

			m, err := getUpdateMetadata(message, root)
			if err != nil {
				return errors.Wrap(err, "gathering environment metadata")
			}

			prompt := fmt.Sprintf("This will permanently destroy all resources in the '%s' stack!", s.Name())
			if !preview && !yes && !confirmPrompt(prompt, string(s.Name())) {
				return errors.New("confirmation declined")
			}

			return s.Destroy(proj, root, m, engine.UpdateOptions{
				Analyzers: analyzers,
				DryRun:    preview,
				Parallel:  parallel,
				Debug:     debug,
			}, backend.DisplayOptions{
				Color:                color.Colorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSames:            showSames,
				DiffDisplay:          diffDisplay,
				Debug:                debug,
			})
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose a stack other than the currently selected one")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with the destruction anyway")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the destroy operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().VarP(
		&color, "color", "c", "Colorize output. Choices are: always, never, raw, auto")
	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVarP(
		&preview, "preview", "n", false,
		"Don't create/delete resources; just preview the planned operations")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")

	return cmd
}
