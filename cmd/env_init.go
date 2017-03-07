// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"errors"

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
		Run: runFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the environment to use.
			if len(args) == 0 {
				return errors.New("missing required environment name")
			}

			name := tokens.QName(args[0])
			create(name)
			return nil
		}),
	}
}
