// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newEnvInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <env>",
		Short: "Create an empty environment with the given name, ready for updates",
		Long: "Create an empty environment with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty environment with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the environment to use.
			if len(args) == 0 {
				return errors.New("missing required environment name")
			}

			envName := tokens.QName(args[0])

			if _, err := lumiEngine.GetEnvironmentInfo(envName); err == nil {
				return fmt.Errorf("environment '%v' already exists", envName)

			}

			err := lumiEngine.InitEnv(envName)
			if err != nil {
				return err
			}

			return setCurrentEnv(envName, false)
		}),
	}
}
