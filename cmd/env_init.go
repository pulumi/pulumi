// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/tokens"
)

func newEnvInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "init <env>",
		Aliases: []string{"create"},
		Short:   "Create an empty environment with the given name, ready for deployments",
		Long: "Create an empty environment with the given name, ready for deployments\n" +
			"\n" +
			"This command creates an empty environment with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `deploy` command.",
		Run: func(cmd *cobra.Command, args []string) {
			// Read in the name of the environment to use.
			if len(args) == 0 {
				exitError("missing required environment name")
			}

			name := tokens.QName(args[0])
			create(name)
		},
	}
}
