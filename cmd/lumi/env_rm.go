// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
)

func newEnvRmCmd() *cobra.Command {
	var yes bool
	var force bool
	var cmd = &cobra.Command{
		Use:   "rm <env>",
		Short: "Remove an environment and its configuration",
		Long: "Remove an environment and its configuration\n" +
			"\n" +
			"This command removes an environment and its configuration state.  Please refer to the\n" +
			"`destroy` command for removing a resources, as this is a distinct operation.\n" +
			"\n" +
			"After this command completes, the environment will no longer be available for deployments.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {

			if len(args) == 0 || args[0] == "" {
				return errors.Errorf("missing required environment name")
			}

			envName := args[0]

			// Ensure the user really wants to do this.
			if yes ||
				confirmPrompt("This will permanently remove the '%v' environment!", envName) {
				return lumiEngine.RemoveEnv(envName, force)
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"By default, removal of a environment with resources will be rejected; this forces it")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with removal anyway")

	return cmd
}
