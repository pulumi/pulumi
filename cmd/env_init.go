// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
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
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the environment to use.
			if len(args) == 0 {
				return errors.New("missing required environment name")
			}

			if _, staterr := os.Stat(workspace.EnvPath(tokens.QName(args[0]))); staterr == nil {
				return fmt.Errorf("environment '%v' already exists", args[0])
			}

			return lumiEngine.InitEnv(args[0])
		}),
	}
}
