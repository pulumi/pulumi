// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newDestroyCmd() *cobra.Command {
	var debug bool
	var preview bool
	var stack string
	var parallel int
	var summary bool
	var yes bool
	var cmd = &cobra.Command{
		Use:        "destroy",
		SuggestFor: []string{"down", "remove"},
		Short:      "Destroy an existing stack and its resources",
		Long: "Destroy an existing stack and its resources\n" +
			"\n" +
			"This command deletes an entire existing stack by name.  The current state is\n" +
			"loaded from the associated snapshot file in the workspace.  After running to completion,\n" +
			"all of this stack's resources and associated state will be gone.\n" +
			"\n" +
			"Warning: although old snapshots can be used to recreate an stack, this command\n" +
			"is generally irreversable and should be used with great care.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stackName, err := explicitOrCurrent(stack)
			if err != nil {
				return err
			}

			cfg, err := getConfiguration(stackName)
			if err != nil {
				return err
			}

			var decrypter config.ValueDecrypter = panicCrypter{}

			if hasSecureValue(cfg) {
				decrypter, err = getSymmetricCrypter()
				if err != nil {
					return err
				}
			}

			if preview || yes ||
				confirmPrompt("This will permanently destroy all resources in the '%v' stack!", stackName.String()) {

				localProvider := localStackProvider{decrypter: decrypter}
				pulumiEngine := engine.Engine{Targets: localProvider, Snapshots: localProvider}

				events := make(chan engine.Event)
				done := make(chan bool)

				go displayEvents(events, done, debug)

				if err := pulumiEngine.Destroy(stackName, events, engine.DestroyOptions{
					DryRun:   preview,
					Parallel: parallel,
					Summary:  summary,
				}); err != nil {
					return err
				}

				<-done
				close(events)
				close(done)
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

	return cmd
}
